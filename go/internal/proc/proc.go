package proc

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

type Runner struct {
	mu   sync.Mutex
	cmd  *exec.Cmd
}

func (r *Runner) Start(llamaExe, model string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cmd != nil && r.cmd.Process != nil {
		return fmt.Errorf("already running")
	}
	cmd := exec.Command(llamaExe,
		"-m", model,
		"-c", "32768",
		"--parallel", "2",
		"--cache-type-k", "f16",
		"--cache-type-v", "f16",
		"-fa", "1",
		"--port", "8080",
		"--host", "127.0.0.1",
		"--jinja",
		"--reasoning", "on",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	prepare(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	r.cmd = cmd
	go func() {
		_ = cmd.Wait()
		r.mu.Lock()
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
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	killTree(cmd)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
	}
}

func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cmd != nil && r.cmd.Process != nil
}
