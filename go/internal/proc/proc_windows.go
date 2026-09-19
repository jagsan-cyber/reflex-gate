//go:build windows

package proc

import (
	"os/exec"
	"strconv"
	"syscall"
)

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	c := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = c.Run()
}
