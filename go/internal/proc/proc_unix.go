//go:build !windows

package proc

import (
	"os/exec"
	"strconv"
	"syscall"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
}

func KillProcessOnPort(port int) {
	if port <= 0 {
		return
	}
	_ = exec.Command("fuser", "-k", strconv.Itoa(port)+"/tcp").Run()
}

func KillAllLlama() {
	_ = exec.Command("pkill", "-f", "llama-server").Run()
}
