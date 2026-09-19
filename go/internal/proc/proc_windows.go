//go:build windows

package proc

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

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
	if cmd.Process == nil {
		return
	}
	c := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid))
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = c.Run()
}

func KillProcessOnPort(port int) {
	if port <= 0 {
		return
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		strconv.Itoa(port)+` | ForEach-Object { (Get-NetTCPConnection -LocalPort $_ -State Listen -ErrorAction SilentlyContinue).OwningProcess }`).Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			line = strings.TrimSpace(line)
			if pid, err := strconv.Atoi(line); err == nil && pid > 0 {
				c := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
				c.SysProcAttr = &syscall.SysProcAttr{
					HideWindow:    true,
					CreationFlags: 0x08000000,
				}
				_ = c.Run()
			}
		}
	}
}

func KillAllLlama() {
	c := exec.Command("taskkill", "/F", "/IM", "llama-server.exe")
	c.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	_ = c.Run()
}
