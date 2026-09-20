// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

func bindProcessToJob(pid int) {}

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	_ = cmd.Process.Kill()
}
