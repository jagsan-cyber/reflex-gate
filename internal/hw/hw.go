// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

package hw

import (
	"fmt"
	"strings"
)

type Info struct {
	Name      string
	Vendor    string // nvidia, amd, intel, cpu
	VRAMBytes uint64
	TotalRAM  uint64
	FreeRAM   uint64
	Backend   string // cuda, vulkan, cpu
	Context   int
	GPULayers int
}

func (i Info) Summary() string {
	vramGB := float64(i.VRAMBytes) / (1024 * 1024 * 1024)
	freeGB := float64(i.FreeRAM) / (1024 * 1024 * 1024)
	gpu := i.Name
	if gpu == "" {
		gpu = "GPUなし / CPU"
	}
	opt := fmt.Sprintf("%s / %dK", strings.ToUpper(i.Backend), i.Context/1024)
	if vramGB >= 0.1 {
		return fmt.Sprintf("環境検知: %s (VRAM %.0fGB, 空きRAM %.1fGB) - 推奨 %s", gpu, vramGB, freeGB, opt)
	}
	return fmt.Sprintf("環境検知: %s (空きRAM %.1fGB) - 推奨 %s", gpu, freeGB, opt)
}

func (i Info) SummaryEN() string {
	vramGB := float64(i.VRAMBytes) / (1024 * 1024 * 1024)
	freeGB := float64(i.FreeRAM) / (1024 * 1024 * 1024)
	gpu := i.Name
	if gpu == "" {
		gpu = "No GPU / CPU"
	}
	opt := fmt.Sprintf("%s / %dK", strings.ToUpper(i.Backend), i.Context/1024)
	if vramGB >= 0.1 {
		return fmt.Sprintf("Detected: %s (VRAM %.0fGB, Free RAM %.1fGB) - Rec: %s", gpu, vramGB, freeGB, opt)
	}
	return fmt.Sprintf("Detected: %s (Free RAM %.1fGB) - Rec: %s", gpu, freeGB, opt)
}

func (i Info) ModeLabel() string {
	return fmt.Sprintf("自動判定 (推奨: %dK / %s)", i.Context/1024, strings.ToUpper(i.Backend))
}

func Recommend(name string, vram, totalRAM, freeRAM uint64) Info {
	n := strings.ToLower(name)
	info := Info{Name: name, VRAMBytes: vram, TotalRAM: totalRAM, FreeRAM: freeRAM, GPULayers: 99}
	switch {
	case strings.Contains(n, "nvidia") || strings.Contains(n, "geforce") || strings.Contains(n, "quadro") || strings.Contains(n, "rtx ") || strings.Contains(n, "gtx "):
		info.Vendor = "nvidia"
		info.Backend = "cuda"
	case strings.Contains(n, "amd") || strings.Contains(n, "radeon") || strings.Contains(n, "rx ") || strings.Contains(n, "vega"):
		info.Vendor = "amd"
		info.Backend = "vulkan"
	case strings.Contains(n, "intel") || strings.Contains(n, "arc") || strings.Contains(n, "iris") || strings.Contains(n, "uhd"):
		info.Vendor = "intel"
		info.Backend = "vulkan"
	case n == "" || strings.Contains(n, "basic display") || strings.Contains(n, "hyper-v") || strings.Contains(n, "remote desktop"):
		info.Vendor = "cpu"
		info.Backend = "cpu"
		info.GPULayers = 0
	default:
		info.Vendor = "cpu"
		info.Backend = "cpu"
		info.GPULayers = 0
	}

	freeGB := float64(freeRAM) / (1024 * 1024 * 1024)
	vramGB := float64(vram) / (1024 * 1024 * 1024)
	switch {
	case info.Backend == "cpu":
		info.Context = 8192
	case vramGB >= 4 || freeGB >= 8:
		info.Context = 32768
	case vramGB >= 2 || freeGB >= 4:
		info.Context = 16384
	default:
		info.Context = 8192
		if vramGB < 0.5 {
			info.GPULayers = 0
			info.Backend = "cpu"
		}
	}
	return info
}
