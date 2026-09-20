// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build windows

package hw

import (
	"encoding/binary"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func Detect() Info {
	name, vram := video()
	total, free := memory()
	return Recommend(name, vram, total, free)
}

func video() (string, uint64) {
	// Query Windows Display Adapter class key in Registry directly: 0 subprocesses, 0 ms.
	classKey, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Class\{4d36e968-e325-11ce-bfc1-08002be10318}`, registry.READ)
	if err != nil {
		return "", 0
	}
	defer classKey.Close()

	subkeys, err := classKey.ReadSubKeyNames(-1)
	if err != nil {
		return "", 0
	}

	bestName := ""
	var bestVRAM uint64

	for _, sub := range subkeys {
		if !strings.HasPrefix(sub, "0") {
			continue
		}
		k, err := registry.OpenKey(classKey, sub, registry.READ)
		if err != nil {
			continue
		}
		desc, _, err := k.GetStringValue("DriverDesc")
		if err != nil || desc == "" {
			_ = k.Close()
			continue
		}
		low := strings.ToLower(desc)
		if strings.Contains(low, "basic display") || strings.Contains(low, "remote desktop") {
			_ = k.Close()
			continue
		}

		var vram uint64
		// Try qwMemorySize (64-bit QWORD or binary)
		if val, _, err := k.GetIntegerValue("HardwareInformation.qwMemorySize"); err == nil && val > 0 {
			vram = val
		} else if bVal, _, err := k.GetBinaryValue("HardwareInformation.qwMemorySize"); err == nil && len(bVal) >= 8 {
			vram = binary.LittleEndian.Uint64(bVal)
		} else if val32, _, err := k.GetIntegerValue("HardwareInformation.MemorySize"); err == nil && val32 > 0 {
			vram = val32
		} else if bVal32, _, err := k.GetBinaryValue("HardwareInformation.MemorySize"); err == nil && len(bVal32) >= 4 {
			vram = uint64(binary.LittleEndian.Uint32(bVal32))
		}
		_ = k.Close()

		if vram >= bestVRAM {
			bestName = desc
			bestVRAM = vram
		}
	}
	return bestName, bestVRAM
}

type memoryStatusEx struct {
	cbSize                  uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

var (
	modkernel32             = windows.NewLazySystemDLL("kernel32.dll")
	procGlobalMemoryStatusEx = modkernel32.NewProc("GlobalMemoryStatusEx")
)

func memory() (total, free uint64) {
	var ms memoryStatusEx
	ms.cbSize = uint32(unsafe.Sizeof(ms))
	r1, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return 0, 0
	}
	return ms.ullTotalPhys, ms.ullAvailPhys
}
