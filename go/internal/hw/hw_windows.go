//go:build windows

package hw

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func Detect() Info {
	name, vram := video()
	total, free := memory()
	return Recommend(name, vram, total, free)
}

func video() (string, uint64) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`Get-CimInstance Win32_VideoController | Select-Object Name,AdapterRAM | ConvertTo-Json -Compress`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", 0
	}
	raw := strings.TrimSpace(string(out))
	type row struct {
		Name       string
		AdapterRAM uint64
	}
	var many []row
	if err := json.Unmarshal([]byte(raw), &many); err != nil {
		var one row
		if json.Unmarshal([]byte(raw), &one) != nil {
			return "", 0
		}
		many = []row{one}
	}
	best := row{}
	for _, r := range many {
		ln := strings.ToLower(r.Name)
		if strings.Contains(ln, "basic display") || strings.Contains(ln, "remote desktop") {
			continue
		}
		if r.AdapterRAM >= best.AdapterRAM {
			best = r
		}
	}
	if best.Name == "" && len(many) > 0 {
		best = many[0]
	}
	return best.Name, best.AdapterRAM
}

func memory() (total, free uint64) {
	cmd := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize,FreePhysicalMemory | ConvertTo-Json -Compress)`)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return 0, 0
	}
	var row struct {
		TotalVisibleMemorySize json.Number
		FreePhysicalMemory     json.Number
	}
	if json.Unmarshal(out, &row) != nil {
		return 0, 0
	}
	tkb, _ := strconv.ParseUint(row.TotalVisibleMemorySize.String(), 10, 64)
	fkb, _ := strconv.ParseUint(row.FreePhysicalMemory.String(), 10, 64)
	return tkb * 1024, fkb * 1024
}
