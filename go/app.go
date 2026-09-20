package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"local-jev/internal/api"
	"local-jev/internal/config"
	"local-jev/internal/download"
	"local-jev/internal/hw"
	"local-jev/internal/proc"
	"local-jev/internal/selftest"
	"local-jev/internal/version"
)

type HardwareDTO struct {
	Name      string  `json:"name"`
	Vendor    string  `json:"vendor"`
	VRAMGB    float64 `json:"vram_gb"`
	FreeRAMGB float64 `json:"free_ram_gb"`
	Backend   string  `json:"backend"`
	GPULayers int     `json:"gpu_layers"`
	Summary   string  `json:"summary"`
	SummaryEN string  `json:"summary_en"`
}

type ConfigDTO struct {
	LlamaServer string `json:"llama_server"`
	Model       string `json:"model"`
	Backend     string `json:"backend"`
	Context     int    `json:"context"`
	GPULayers   int    `json:"gpu_layers"`
	JevPort     int    `json:"jev_port"`
	LlamaPort   int    `json:"llama_port"`
	Host        string `json:"host"`
	Lang        string `json:"lang"`
}

type ServerStatusDTO struct {
	Running           bool   `json:"running"`
	Starting          bool   `json:"starting"`
	CPUFallback       bool   `json:"cpu_fallback"`
	JevPort           int    `json:"jev_port"`
	Status            string `json:"status"`
	ActiveBackend     string `json:"active_backend"`
	ActiveDevice      string `json:"active_device"`
	LastError         string `json:"last_error,omitempty"`
	ConfiguredContext int    `json:"configured_context"`
	EffectiveContext  int    `json:"effective_context"`
	ContextWarning    string `json:"context_warning,omitempty"`
}

type DownloadProgressDTO struct {
	Label    string  `json:"label"`
	Done     int64   `json:"done"`
	Total    int64   `json:"total"`
	Pct      float64 `json:"pct"`
	DoneMB   float64 `json:"done_mb"`
	TotalMB  float64 `json:"total_mb"`
	Complete bool    `json:"complete"`
	Error    string  `json:"error"`
}

type InitialState struct {
	AppName string          `json:"app_name"`
	Version string          `json:"version"`
	Cfg     ConfigDTO       `json:"cfg"`
	Hw      HardwareDTO     `json:"hw"`
	Status  ServerStatusDTO `json:"status"`
}

type App struct {
	ctx           context.Context
	cfg           config.Config
	hw            hw.Info
	runner        *proc.Runner
	api           *api.Server
	starting      bool
	apiRunning    bool
	isCPUFallback bool
	activeBackend string
	activeDevice  string
	mu            sync.Mutex

	downloadedLlama string
	downloadedModel string
}

func NewApp() *App {
	cfg := config.Load()
	hwi := hw.Detect()
	autoDetectCandidates(&cfg, hwi)

	if cfg.Mode == "" {
		cfg.Mode = "auto"
	}
	if cfg.Mode == "auto" {
		cfg.Backend = hwi.Backend
		cfg.GPULayers = hwi.GPULayers
	}
	if cfg.Context <= 0 {
		cfg.Context = 8192
	}
	if cfg.JevPort <= 0 {
		cfg.JevPort = 8090
	}
	if cfg.LlamaPort <= 0 {
		cfg.LlamaPort = 8080
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Lang == "" {
		cfg.Lang = "ja"
	}

	return &App{
		cfg:    cfg,
		hw:     hwi,
		runner: &proc.Runner{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	_ = a.StopServer()
	cleanupTray()
}

func (a *App) GetVersion() string {
	return version.Version
}

func (a *App) GetInitialState() InitialState {
	a.mu.Lock()
	defer a.mu.Unlock()

	return InitialState{
		AppName: version.AppName,
		Version: version.Version,
		Cfg: ConfigDTO{
			LlamaServer: a.cfg.LlamaServer,
			Model:       a.cfg.Model,
			Backend:     a.cfg.Backend,
			Context:     a.cfg.Context,
			GPULayers:   a.cfg.GPULayers,
			JevPort:     a.cfg.JevPort,
			LlamaPort:   a.cfg.LlamaPort,
			Host:        a.cfg.Host,
			Lang:        a.cfg.Lang,
		},
		Hw: HardwareDTO{
			Name:      a.hw.Name,
			Vendor:    a.hw.Vendor,
			VRAMGB:    float64(a.hw.VRAMBytes) / (1024 * 1024 * 1024),
			FreeRAMGB: float64(a.hw.FreeRAM) / (1024 * 1024 * 1024),
			Backend:   a.hw.Backend,
			GPULayers: a.hw.GPULayers,
			Summary:   a.hw.Summary(),
			SummaryEN: a.hw.SummaryEN(),
		},
		Status: ServerStatusDTO{
			Running:           a.apiRunning,
			Starting:          a.starting,
			CPUFallback:       a.isCPUFallback,
			JevPort:           a.cfg.JevPort,
			Status:            "idle",
			ActiveBackend:     a.activeBackend,
			ActiveDevice:      a.activeDevice,
			ConfiguredContext: a.cfg.Context,
			EffectiveContext:  a.cfg.Context,
		},
	}
}

func (a *App) RunSelfTest(mode string) (selftest.SelfTestResult, error) {
	a.mu.Lock()
	if !a.apiRunning {
		a.mu.Unlock()
		return selftest.SelfTestResult{}, fmt.Errorf("server is not running")
	}
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	a.mu.Unlock()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	return selftest.Run(baseURL, mode, func(done, total int, cur string) {
		runtime.EventsEmit(a.ctx, "selftest-progress", map[string]any{
			"done":  done,
			"total": total,
			"pct":   float64(done) / float64(total) * 100.0,
			"curr":  cur,
		})
	})
}

func (a *App) SelectLlamaServer() (string, error) {
	title := "llama-server.exe を選択"
	if a.cfg.Lang == "en" {
		title = "Select llama-server.exe"
	}
	filters := []runtime.FileFilter{
		{DisplayName: "Executable (*.exe)", Pattern: "*.exe"},
		{DisplayName: "All Files (*.*)", Pattern: "*.*"},
	}
	selected, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err == nil && selected != "" {
		a.mu.Lock()
		a.cfg.LlamaServer = selected
		_ = a.cfg.Save()
		a.mu.Unlock()
	}
	return selected, err
}

func (a *App) SelectModel() (string, error) {
	title := "Qwen3.5-0.8B GGUF モデルを選択"
	if a.cfg.Lang == "en" {
		title = "Select Qwen3.5-0.8B GGUF Model"
	}
	filters := []runtime.FileFilter{
		{DisplayName: "GGUF Model (*.gguf)", Pattern: "*.gguf"},
		{DisplayName: "All Files (*.*)", Pattern: "*.*"},
	}
	selected, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err == nil && selected != "" {
		a.mu.Lock()
		a.cfg.Model = selected
		_ = a.cfg.Save()
		a.mu.Unlock()
	}
	return selected, err
}

func (a *App) SaveConfig(dto ConfigDTO) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.LlamaServer = dto.LlamaServer
	a.cfg.Model = dto.Model
	a.cfg.Backend = dto.Backend
	a.cfg.Context = dto.Context
	a.cfg.GPULayers = dto.GPULayers
	a.cfg.JevPort = dto.JevPort
	a.cfg.LlamaPort = dto.LlamaPort
	a.cfg.Host = dto.Host
	a.cfg.Lang = dto.Lang
	return a.cfg.Save()
}

func (a *App) SetBackend(backend string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.Backend = backend
	if backend == "cpu" {
		a.cfg.GPULayers = 0
	} else if a.cfg.GPULayers == 0 {
		a.cfg.GPULayers = 99
	}
	autoDetectCandidates(&a.cfg, a.hw)
	_ = a.cfg.Save()
	return a.cfg.LlamaServer
}

func (a *App) StartServer(dto ConfigDTO) error {
	a.mu.Lock()
	if a.starting || a.apiRunning {
		a.mu.Unlock()
		return fmt.Errorf("server already active or starting")
	}

	a.cfg.LlamaServer = dto.LlamaServer
	a.cfg.Model = dto.Model
	if dto.Backend != "" {
		a.cfg.Backend = dto.Backend
	}
	a.cfg.Context = dto.Context
	a.cfg.GPULayers = dto.GPULayers
	if a.cfg.Backend == "cpu" {
		a.cfg.GPULayers = 0
	}
	a.cfg.JevPort = dto.JevPort
	a.cfg.LlamaPort = dto.LlamaPort
	a.cfg.Host = dto.Host
	_ = a.cfg.Save()

	llamaPath := strings.TrimSpace(a.cfg.LlamaServer)
	modelPath := strings.TrimSpace(a.cfg.Model)
	if llamaPath == "" || modelPath == "" {
		a.mu.Unlock()
		return fmt.Errorf("please specify both llama-server and model paths")
	}
	if _, err := os.Stat(llamaPath); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("llama-server not found: %s", llamaPath)
	}
	if _, err := os.Stat(modelPath); err != nil {
		a.mu.Unlock()
		return fmt.Errorf("model not found: %s", modelPath)
	}

	if a.cfg.LlamaPort == a.cfg.JevPort {
		a.mu.Unlock()
		errMsg := fmt.Sprintf("Llama Port and JEV Port cannot be the same (%d). Please change one in Settings.", a.cfg.LlamaPort)
		if a.cfg.Lang != "en" {
			errMsg = fmt.Sprintf("Llama ポートと JEV ポートが重複しています (%d)。設定画面で別のポートを指定してください。", a.cfg.LlamaPort)
		}
		runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
			Running:   false,
			Starting:  false,
			JevPort:   a.cfg.JevPort,
			Status:    "stopped",
			LastError: errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	if proc.IsPortInUse("127.0.0.1", a.cfg.LlamaPort) {
		a.mu.Unlock()
		errMsg := fmt.Sprintf("Llama Port %d is already in use by another application. Please change the port in Settings.", a.cfg.LlamaPort)
		if a.cfg.Lang != "en" {
			errMsg = fmt.Sprintf("Llama ポート (%d) は既に他のアプリケーションで使用されています。設定画面でポート番号を変更するか、該当アプリを終了してください。", a.cfg.LlamaPort)
		}
		runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
			Running:   false,
			Starting:  false,
			JevPort:   a.cfg.JevPort,
			Status:    "stopped",
			LastError: errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	if proc.IsPortInUse(a.cfg.Host, a.cfg.JevPort) {
		a.mu.Unlock()
		errMsg := fmt.Sprintf("JEV Port %d is already in use by another application. Please change the port in Settings.", a.cfg.JevPort)
		if a.cfg.Lang != "en" {
			errMsg = fmt.Sprintf("JEV ポート (%d) は既に他のアプリケーションで使用されています。設定画面でポート番号を変更するか、該当アプリを終了してください。", a.cfg.JevPort)
		}
		runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
			Running:   false,
			Starting:  false,
			JevPort:   a.cfg.JevPort,
			Status:    "stopped",
			LastError: errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	opts := proc.Options{
		Context:   a.cfg.Context,
		Parallel:  3,
		GPULayers: a.cfg.GPULayers,
		LlamaPort: a.cfg.LlamaPort,
		Host:      "127.0.0.1",
	}

	a.starting = true
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
		Running:  false,
		Starting: true,
		JevPort:  a.cfg.JevPort,
		Status:   "starting",
	})

	if err := a.runner.Start(llamaPath, modelPath, opts); err != nil {
		a.mu.Lock()
		a.starting = false
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
			Running:  false,
			Starting: false,
			JevPort:  a.cfg.JevPort,
			Status:   "failed",
		})
		return fmt.Errorf("failed to start runner: %w", err)
	}

	go func() {
		deadline := time.Now().Add(60 * time.Second)
		ok := false
		checkURL := fmt.Sprintf("http://127.0.0.1:%d/v1/models", opts.LlamaPort)
		hasRetriedCPU := false
		isCPUFallback := false

		for time.Now().Before(deadline) {
			if exited, code, lastErr := a.runner.HasExited(); exited {
				if opts.GPULayers > 0 && !hasRetriedCPU {
					hasRetriedCPU = true
					isCPUFallback = true
					log.Printf("[local-jev] GPU execution failed (code %d: %s). Auto-fallback to CPU (NGL=0)...", code, lastErr)
					runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
						Running:     false,
						Starting:    true,
						CPUFallback: true,
						JevPort:     a.cfg.JevPort,
						Status:      "fallback_cpu",
					})
					time.Sleep(300 * time.Millisecond)
					opts.GPULayers = 0
					if err := a.runner.Start(llamaPath, modelPath, opts); err != nil {
						break
					}
					deadline = time.Now().Add(45 * time.Second)
					continue
				} else {
					a.mu.Lock()
					a.starting = false
					a.apiRunning = false
					a.mu.Unlock()
					runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
						Running:   false,
						Starting:  false,
						JevPort:   a.cfg.JevPort,
						Status:    "failed",
						LastError: fmt.Sprintf("Process exited (code %d): %s", code, lastErr),
					})
					return
				}
			}

			resp, err := http.Get(checkURL)
			if err == nil && resp.StatusCode == 200 {
				_ = resp.Body.Close()
				ok = true
				break
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			time.Sleep(400 * time.Millisecond)
		}

		if !ok {
			a.runner.Stop()
			a.mu.Lock()
			a.starting = false
			a.apiRunning = false
			a.mu.Unlock()
			runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
				Running:  false,
				Starting: false,
				JevPort:  a.cfg.JevPort,
				Status:   "timeout",
			})
			return
		}

		llamaRoot := fmt.Sprintf("http://127.0.0.1:%d", opts.LlamaPort)
		apiSrv := api.NewServer(llamaRoot)
		apiSrv.AuthMode = a.cfg.AuthMode
		apiSrv.APIKey = a.cfg.APIKey
		apiSrv.OnEvent = func(ev api.RequestEvent) {
			runtime.EventsEmit(a.ctx, "telemetry", ev)
		}

		recentLogs := a.runner.RecentLogs()
		activeBackend, activeDevice := detectActiveEngine(llamaPath, opts.GPULayers, isCPUFallback, a.hw, recentLogs)

		a.mu.Lock()
		a.api = apiSrv
		a.isCPUFallback = isCPUFallback
		a.activeBackend = activeBackend
		a.activeDevice = activeDevice
		a.starting = false
		a.apiRunning = true
		bindHost := a.cfg.Host
		if bindHost == "" {
			bindHost = "0.0.0.0"
		}
		bindAddr := fmt.Sprintf("%s:%d", bindHost, a.cfg.JevPort)
		a.mu.Unlock()

		go func() {
			_ = apiSrv.Start(bindAddr)
		}()

		// Query effective context from llama-server props
		effectiveCtx := opts.Context
		var ctxWarning string
		propClient := &http.Client{Timeout: 2 * time.Second}
		if pResp, pErr := propClient.Get(fmt.Sprintf("http://127.0.0.1:%d/props", opts.LlamaPort)); pErr == nil {
			var props struct {
				DefaultGenSettings struct {
					NCtx int `json:"n_ctx"`
				} `json:"default_generation_settings"`
			}
			if json.NewDecoder(pResp.Body).Decode(&props) == nil && props.DefaultGenSettings.NCtx > 0 {
				effectiveCtx = props.DefaultGenSettings.NCtx
				if effectiveCtx < opts.Context {
					ctxWarning = fmt.Sprintf("Context capped by model: configured %d, effective %d", opts.Context, effectiveCtx)
				}
			}
			pResp.Body.Close()
		}

		runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
			Running:           true,
			Starting:          false,
			CPUFallback:       isCPUFallback,
			JevPort:           a.cfg.JevPort,
			Status:            "running",
			ActiveBackend:     activeBackend,
			ActiveDevice:      activeDevice,
			ConfiguredContext: opts.Context,
			EffectiveContext:  effectiveCtx,
			ContextWarning:    ctxWarning,
		})

		updateTrayStatus(fmt.Sprintf("ReflexGate (Running: Port %d)", a.cfg.JevPort))
	}()

	return nil
}

func (a *App) StopServer() error {
	a.mu.Lock()
	a.apiRunning = false
	a.starting = false

	if a.api != nil {
		_ = a.api.Stop()
		a.api = nil
	}
	a.isCPUFallback = false
	a.activeBackend = ""
	a.activeDevice = ""
	a.runner.Stop()
	a.mu.Unlock()

	runtime.EventsEmit(a.ctx, "status-changed", ServerStatusDTO{
		Running:  false,
		Starting: false,
		JevPort:  a.cfg.JevPort,
		Status:   "stopped",
	})

	updateTrayStatus("ReflexGate (Stopped)")
	return nil
}

func detectActiveEngine(llamaPath string, gpuLayers int, isCPUFallback bool, hwi hw.Info, logs string) (string, string) {
	if isCPUFallback || gpuLayers == 0 {
		return "CPU", "CPU (Fallback)"
	}

	lowLog := strings.ToLower(logs)
	lowExe := strings.ToLower(filepath.Base(llamaPath))
	lowDir := strings.ToLower(filepath.Dir(llamaPath))

	// 1. Check runtime logs & path for Vulkan
	if strings.Contains(lowLog, "vulkan") || strings.Contains(lowDir, "vulkan") || strings.Contains(lowExe, "vulkan") {
		devName := hwi.Name
		if devName == "" {
			devName = "Vulkan GPU"
		}
		return "Vulkan", devName
	}

	// 2. Check for AMD ROCm / HIP
	if strings.Contains(lowLog, "rocm") || strings.Contains(lowDir, "hip") || strings.Contains(lowExe, "hip") {
		devName := hwi.Name
		if devName == "" {
			devName = "AMD ROCm/HIP"
		}
		return "HIP", devName
	}

	// 3. Check for NVIDIA CUDA
	if strings.Contains(lowLog, "cuda") || strings.Contains(lowDir, "cuda") || strings.Contains(lowDir, "cu12") {
		devName := hwi.Name
		if devName == "" {
			devName = "NVIDIA CUDA"
		}
		return "CUDA", devName
	}

	// 4. Check for Intel SYCL
	if strings.Contains(lowLog, "sycl") {
		devName := hwi.Name
		if devName == "" {
			devName = "Intel SYCL"
		}
		return "SYCL", devName
	}

	// 5. Fallback based on detected GPU vendor
	switch hwi.Vendor {
	case "nvidia":
		return "CUDA", hwi.Name
	case "amd":
		return "Vulkan", hwi.Name
	case "intel":
		return "Vulkan", hwi.Name
	default:
		return "CPU", "CPU"
	}
}

func (a *App) OpenDemo() error {
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/demo", port)
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
		if ok {
			return localAddr.IP.String()
		}
	}
	return "127.0.0.1"
}

func (a *App) CopyURL() error {
	host := a.cfg.Host
	if host == "" || host == "0.0.0.0" {
		host = getLocalIP()
	}
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	url := fmt.Sprintf("http://%s:%d", host, port)
	return runtime.ClipboardSetText(a.ctx, url)
}

func (a *App) TriggerQuickTest(taskType string) (string, error) {
	a.mu.Lock()
	if !a.apiRunning {
		a.mu.Unlock()
		return "", fmt.Errorf("server is not running")
	}
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	a.mu.Unlock()

	var path, payload string
	switch taskType {
	case "stop":
		path = "/jev/stop"
		payload = `{"log": "pytest test_api.py -v\n12 passed in 1.42s\nExit code: 0\nAll tasks completed."}`
	case "extract":
		path = "/jev/extract"
		payload = `{"log": "Ran pytest: 3 passed. Modified internal/api/server.go and schema.go."}`
	case "scan":
		path = "/jev/scan"
		payload = `{"log": "Process crashed: SIGSEGV 11: Invalid memory reference at 0x00007fff8820\nThread 4 terminated abnormally."}`
	default:
		return "", fmt.Errorf("unknown test task: %s", taskType)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", port, path)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return fmt.Sprintf("Status: %d", resp.StatusCode), nil
}

func (a *App) StartAutoDownload() error {
	go func() {
		base := config.DataDir()
		backend := a.hw.Backend
		if a.cfg.Backend != "" {
			backend = a.cfg.Backend
		}

		llama, model, err := download.FetchAll(base, backend, func(label string, done, total int64) {
			pct := 0.0
			if total > 0 {
				pct = float64(done) / float64(total) * 100.0
			}
			runtime.EventsEmit(a.ctx, "download-progress", DownloadProgressDTO{
				Label:    label,
				Done:     done,
				Total:    total,
				Pct:      pct,
				DoneMB:   float64(done) / (1024 * 1024),
				TotalMB:  float64(total) / (1024 * 1024),
				Complete: false,
			})
		})

		if err != nil {
			runtime.EventsEmit(a.ctx, "download-progress", DownloadProgressDTO{
				Error:    err.Error(),
				Complete: false,
			})
			return
		}

		a.mu.Lock()
		a.downloadedLlama = llama
		a.downloadedModel = model
		a.cfg.LlamaServer = llama
		a.cfg.Model = model
		_ = a.cfg.Save()
		a.mu.Unlock()

		runtime.EventsEmit(a.ctx, "download-progress", DownloadProgressDTO{
			Label:    "Complete",
			Pct:      100.0,
			Complete: true,
		})
	}()
	return nil
}

func (a *App) SetLanguage(lang string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cfg.Lang = lang
	return a.cfg.Save()
}

func isValidLlamaServer(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Dir = filepath.Dir(path)
	setHideWindow(cmd)
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
}

func autoDetectCandidates(cfg *config.Config, hwi hw.Info) {
	wanted := strings.ToLower(cfg.Backend)
	if wanted == "" || wanted == "auto" {
		if hwi.Vendor == "amd" || hwi.Vendor == "intel" {
			wanted = "vulkan"
		} else if hwi.Vendor == "nvidia" {
			wanted = "cuda"
		} else {
			wanted = "cpu"
		}
	}

	var candidates []string
	switch wanted {
	case "vulkan":
		candidates = []string{
			`..\..\build-vulkan\bin\llama-server.exe`,
			`bin\llama.cpp-vulkan\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`..\..\build\bin\llama-server.exe`,
			`llama-server.exe`,
		}
	case "cuda":
		candidates = []string{
			`..\..\build\bin\llama-server.exe`,
			`..\..\build-cuda\bin\llama-server.exe`,
			`bin\llama.cpp-cuda\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
	case "hip", "rocm":
		candidates = []string{
			`..\..\build-hip\bin\llama-server.exe`,
			`bin\llama.cpp-rocm\llama-server.exe`,
			`bin\llama.cpp-hip\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
	case "sycl", "intel":
		candidates = []string{
			`bin\llama.cpp-sycl\llama-server.exe`,
			`..\..\build-sycl\bin\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
	case "cpu":
		candidates = []string{
			`bin\llama.cpp-cpu\llama-server.exe`,
			`..\..\build\bin\llama-server.exe`,
			`..\..\build-vulkan\bin\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
	default:
		candidates = []string{
			`..\..\build-vulkan\bin\llama-server.exe`,
			`..\..\build\bin\llama-server.exe`,
			`..\..\build-hip\bin\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
	}

	found := false
	for _, cand := range candidates {
		abs, err := filepath.Abs(filepath.Join(config.DataDir(), cand))
		if err == nil && isValidLlamaServer(abs) {
			cfg.LlamaServer = abs
			_ = cfg.Save()
			found = true
			break
		}
	}
	if !found && (cfg.LlamaServer == "" || !isValidLlamaServer(cfg.LlamaServer)) {
		cfg.LlamaServer = ""
		_ = cfg.Save()
	}

	if cfg.Model == "" {
		userProfile := os.Getenv("USERPROFILE")
		candidates := []string{
			filepath.Join(userProfile, `.lmstudio\models\unsloth\Qwen3.5-0.8B-GGUF\Qwen3.5-0.8B-Q8_0.gguf`),
			filepath.Join(config.DataDir(), `models\Qwen3.5-0.8B-Q8_0.gguf`),
			filepath.Join(config.DataDir(), `qwen3.5-0.8b-instruct-q4_k_m.gguf`),
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				cfg.Model = cand
				break
			}
		}
	}
}
