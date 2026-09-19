package proc

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type tailBuffer struct {
	mu    sync.Mutex
	lines []string
}

func (b *tailBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	text := string(p)
	parts := strings.Split(text, "\n")
	for _, p := range parts {
		trimmed := strings.TrimRight(p, "\r\n")
		if trimmed != "" {
			b.lines = append(b.lines, trimmed)
			if len(b.lines) > 25 {
				b.lines = b.lines[1:]
			}
		}
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Join(b.lines, "\n")
}

type Runner struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	logBuf   *tailBuffer
	exited   bool
	exitCode int
	lastErr  string
	lastPort int
}

type Options struct {
	Context   int
	Parallel  int
	GPULayers int
	LlamaPort int
	Host      string
}

func (r *Runner) Start(llamaExe, model string, opt Options) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		return fmt.Errorf("already running")
	}
	if opt.Context <= 0 {
		opt.Context = 8192
	}
	if opt.Parallel <= 0 {
		opt.Parallel = 2
	}
	if opt.LlamaPort <= 0 {
		opt.LlamaPort = 8080
	}
	if opt.Host == "" {
		opt.Host = "127.0.0.1"
	}

	// Terminate any stale process occupying the target port before launch
	KillProcessOnPort(opt.LlamaPort)
	r.lastPort = opt.LlamaPort

	args := []string{
		"-m", model,
		"-c", fmt.Sprintf("%d", opt.Context),
		"--parallel", fmt.Sprintf("%d", opt.Parallel),
		"--n-gpu-layers", fmt.Sprintf("%d", opt.GPULayers),
		"--port", fmt.Sprintf("%d", opt.LlamaPort),
		"--host", opt.Host,
		"--jinja",
		"--swa-full",
		"-cms", "0",
		"-sps", "0.05",
	}

	// Only enable Flash Attention and f16 cache if GPU offload is active
	if opt.GPULayers > 0 {
		args = append(args,
			"--cache-type-k", "f16",
			"--cache-type-v", "f16",
			"-fa", "1",
		)
	}

	cmd := exec.Command(llamaExe, args...)
	cmd.Dir = filepath.Dir(llamaExe)
	cmd.Stdout = os.Stdout

	logBuf := &tailBuffer{}
	cmd.Stderr = io.MultiWriter(os.Stderr, logBuf)
	prepare(cmd)

	if err := cmd.Start(); err != nil {
		r.exited = true
		r.exitCode = -1
		r.lastErr = err.Error()
		return err
	}

	r.cmd = cmd
	r.logBuf = logBuf
	r.exited = false
	r.exitCode = 0
	r.lastErr = ""

	go func() {
		err := cmd.Wait()
		r.mu.Lock()
		r.exited = true
		if cmd.ProcessState != nil {
			r.exitCode = cmd.ProcessState.ExitCode()
		}
		if err != nil {
			r.lastErr = fmt.Sprintf("%v\n%s", err, logBuf.String())
		} else {
			r.lastErr = logBuf.String()
		}
		if r.cmd == cmd {
			r.cmd = nil
		}
		r.mu.Unlock()
	}()
	return nil
}

func (r *Runner) Stop() {
	r.mu.Lock()
	cmd := r.cmd
	r.cmd = nil
	port := r.lastPort
	r.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		killTree(cmd)
		done := make(chan struct{})
		go func() {
			_ = cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			_ = cmd.Process.Kill()
		}
	}
	if port > 0 {
		KillProcessOnPort(port)
	}
}

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd != nil && r.cmd.Process != nil && !r.exited
}

func (r *Runner) HasExited() (bool, int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.exited, r.exitCode, r.lastErr
}

func (r *Runner) RecentLogs() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.logBuf != nil {
		return r.logBuf.String()
	}
	return ""
}
