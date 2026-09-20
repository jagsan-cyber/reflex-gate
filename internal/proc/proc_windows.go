// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build windows

package proc

import (
	"os"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	jobOnce   sync.Once
	jobHandle windows.Handle
)

func getJobObject() windows.Handle {
	jobOnce.Do(func() {
		job, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		_, err = windows.SetInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err == nil {
			jobHandle = job
		}
	})
	return jobHandle
}

func bindProcessToJob(pid int) {
	if pid <= 0 {
		return
	}
	job := getJobObject()
	if job == 0 {
		return
	}
	hProcess, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(hProcess)
	_ = windows.AssignProcessToJobObject(job, hProcess)
}

func prepare(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x08000000,
		HideWindow:    true,
	}

	env := os.Environ()
	rocmPaths := []string{
		`C:\Users\fallo\miniconda3\Lib\site-packages\_rocm_sdk_devel\bin`,
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		rocmPaths = append(rocmPaths, appData+`\Python\Python313\site-packages\_rocm_sdk_devel\bin`)
	}
	var extraPath string
	for _, p := range rocmPaths {
		if _, err := os.Stat(p); err == nil {
			extraPath = p
			break
		}
	}
	if extraPath != "" {
		for i, e := range env {
			if len(e) >= 5 && (e[:5] == "PATH=" || e[:5] == "Path=" || e[:5] == "path=") {
				env[i] = "PATH=" + extraPath + ";" + e[5:]
				break
			}
		}
		env = append(env, "GPU_MAX_HW_QUEUES=4")
		cmd.Env = env
	}
}

func killTree(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	if pid > 0 {
		// Forcibly kill process and all descendants via taskkill
		killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
		killCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = killCmd.Run()
	}
	_ = cmd.Process.Kill()
}
