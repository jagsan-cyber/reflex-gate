package gui

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"local-jev/internal/api"
	"local-jev/internal/config"
	"local-jev/internal/download"
	"local-jev/internal/proc"
)

type App struct {
	fy         fyne.App
	win        fyne.Window
	cfg        config.Config
	llamaPath  *widget.Label
	modelPath  *widget.Label
	status     *widget.Label
	progress   *widget.ProgressBar
	progLabel  *widget.Label
	startBtn   *widget.Button
	stopBtn    *widget.Button
	demoBtn    *widget.Button
	dlBtn      *widget.Button
	runner     proc.Runner
	api        *api.Server
	apiRunning bool
}

func Run() {
	a := &App{fy: app.NewWithID("local-jev.launcher")}
	a.fy.Settings().SetTheme(theme.DarkTheme())
	a.win = a.fy.NewWindow("local-jev")
	a.win.Resize(fyne.NewSize(720, 560))
	a.cfg = config.Load()

	a.llamaPath = widget.NewLabel(orDash(a.cfg.LlamaServer))
	a.llamaPath.Wrapping = fyne.TextWrapBreak
	a.modelPath = widget.NewLabel(orDash(a.cfg.Model))
	a.modelPath.Wrapping = fyne.TextWrapBreak
	a.status = widget.NewLabel("stopped")
	a.progress = widget.NewProgressBar()
	a.progLabel = widget.NewLabel("")
	a.startBtn = widget.NewButton("JEV 起動", a.start)
	a.stopBtn = widget.NewButton("停止", a.stop)
	a.stopBtn.Disable()
	a.demoBtn = widget.NewButton("デモ画面を開く", a.openDemo)
	a.demoBtn.Disable()
	a.dlBtn = widget.NewButton("バイナリ＆モデルを自動取得", a.autoDownload)

	llamaBox := dropCard("llama-server.exe をドロップ", a.llamaPath, func() {
		dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			_ = rc.Close()
			a.setLlama(rc.URI().Path())
		}, a.win).Show()
	})
	modelBox := dropCard(".gguf をドロップ", a.modelPath, func() {
		dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			_ = rc.Close()
			a.setModel(rc.URI().Path())
		}, a.win).Show()
	})

	a.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		for _, u := range uris {
			p := u.Path()
			low := strings.ToLower(p)
			switch {
			case strings.HasSuffix(low, ".gguf"):
				a.setModel(p)
			case strings.Contains(strings.ToLower(filepath.Base(p)), "llama-server") || strings.HasSuffix(low, ".exe"):
				a.setLlama(p)
			}
		}
	})

	a.win.SetCloseIntercept(func() {
		a.cleanup()
		a.win.Close()
	})
	a.fy.Lifecycle().SetOnStopped(func() { a.cleanup() })

	a.win.SetContent(container.NewVBox(
		widget.NewLabel("local-jev ランチャー"),
		widget.NewLabel("llama-server と Qwen3.5-0.8B GGUF をドロップするか、自動取得してください。"),
		llamaBox,
		modelBox,
		a.dlBtn,
		a.progLabel,
		a.progress,
		container.NewHBox(a.startBtn, a.stopBtn, a.demoBtn),
		a.status,
	))
	a.win.ShowAndRun()
}

func dropCard(title string, path *widget.Label, browse func()) fyne.CanvasObject {
	btn := widget.NewButton("参照…", browse)
	return container.NewVBox(widget.NewLabel(title), path, btn)
}

func orDash(s string) string {
	if s == "" {
		return "(未設定)"
	}
	return s
}

func (a *App) setLlama(p string) {
	a.cfg.LlamaServer = p
	a.llamaPath.SetText(p)
	_ = a.cfg.Save()
}

func (a *App) setModel(p string) {
	a.cfg.Model = p
	a.modelPath.SetText(p)
	_ = a.cfg.Save()
}

func (a *App) ui(fn func()) {
	// Fyne 2.6+: fyne.Do. 2.5: call then refresh.
	fn()
	if a.win != nil && a.win.Canvas() != nil {
		a.win.Content().Refresh()
	}
}

func (a *App) autoDownload() {
	a.dlBtn.Disable()
	a.progLabel.SetText("ダウンロード開始…")
	go func() {
		defer a.ui(func() { a.dlBtn.Enable() })
		base := config.DataDir()
		llama, model, err := download.FetchAll(base, func(label string, done, total int64) {
			a.ui(func() {
				a.progLabel.SetText(label)
				if total > 0 {
					a.progress.SetValue(float64(done) / float64(total))
				}
			})
		})
		a.ui(func() {
			if err != nil {
				a.status.SetText("取得失敗: " + err.Error())
				dialog.ShowError(err, a.win)
				return
			}
			a.setLlama(llama)
			a.setModel(model)
			a.progress.SetValue(1)
			a.progLabel.SetText("準備完了")
			a.status.SetText("バイナリとモデルをセットしました")
		})
	}()
}

func (a *App) start() {
	if a.cfg.LlamaServer == "" || a.cfg.Model == "" {
		dialog.ShowInformation("未設定", "llama-server と GGUF を指定してください。", a.win)
		return
	}
	if _, err := os.Stat(a.cfg.LlamaServer); err != nil {
		dialog.ShowError(fmt.Errorf("llama-server が見つかりません: %w", err), a.win)
		return
	}
	if _, err := os.Stat(a.cfg.Model); err != nil {
		dialog.ShowError(fmt.Errorf("モデルが見つかりません: %w", err), a.win)
		return
	}
	a.startBtn.Disable()
	a.status.SetText("llama-server 起動中…")
	if err := a.runner.Start(a.cfg.LlamaServer, a.cfg.Model); err != nil {
		a.startBtn.Enable()
		dialog.ShowError(err, a.win)
		return
	}
	go func() {
		deadline := time.Now().Add(90 * time.Second)
		ok := false
		for time.Now().Before(deadline) {
			if _, _, err := api.NewLlama("http://127.0.0.1:8080").ModelsOK(); err == nil {
				ok = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !ok {
			a.runner.Stop()
			a.ui(func() {
				a.startBtn.Enable()
				a.status.SetText("llama-server が応答しません")
			})
			return
		}
		a.api = api.NewServer("http://127.0.0.1:8080")
		go func() {
			_ = a.api.Start("127.0.0.1:8090")
		}()
		time.Sleep(300 * time.Millisecond)
		a.ui(func() {
			a.apiRunning = true
			a.stopBtn.Enable()
			a.demoBtn.Enable()
			a.status.SetText("稼働中（Port 8090）")
		})
	}()
}

func (a *App) stop() {
	a.cleanup()
	a.startBtn.Enable()
	a.stopBtn.Disable()
	a.demoBtn.Disable()
	a.status.SetText("stopped")
}

func (a *App) cleanup() {
	if a.api != nil {
		_ = a.api.Stop()
		a.api = nil
	}
	a.apiRunning = false
	a.runner.Stop()
}

func (a *App) openDemo() {
	u, _ := url.Parse("http://127.0.0.1:8090/demo")
	_ = a.fy.OpenURL(u)
}
