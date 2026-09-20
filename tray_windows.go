// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

//go:build windows

package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	shell32           = windows.NewLazySystemDLL("shell32.dll")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pLoadIconW        = user32.NewProc("LoadIconW")
	pShell_NotifyIconW= shell32.NewProc("Shell_NotifyIconW")
	pCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	pAppendMenuW      = user32.NewProc("AppendMenuW")
	pDestroyMenu      = user32.NewProc("DestroyMenu")
	pTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetCursorPos     = user32.NewProc("GetCursorPos")
)

const (
	NIM_ADD          = 0x00000000
	NIM_MODIFY       = 0x00000001
	NIM_DELETE       = 0x00000002
	NIF_MESSAGE      = 0x00000001
	NIF_ICON         = 0x00000002
	NIF_TIP          = 0x00000004
	WM_USER          = 0x0400
	WM_TRAY          = WM_USER + 100
	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	TPM_RETURNCMD    = 0x0100
	MF_SEPARATOR     = 0x0800
)

type NOTIFYICONDATAW struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	TimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

type POINT struct {
	X, Y int32
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type TrayManager struct {
	app     *App
	hwnd    uintptr
	mu      sync.Mutex
	running bool
}

var globalTray *TrayManager

func initTray(a *App) *TrayManager {
	tm := &TrayManager{app: a}
	globalTray = tm
	go tm.runMessageLoop()
	return tm
}

func updateTrayStatus(tip string) {
	if globalTray != nil {
		globalTray.updateIcon(tip)
	}
}

func (tm *TrayManager) runMessageLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	className, _ := windows.UTF16PtrFromString("ReflexGateTrayClass")
	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		LpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0, 0, 0, 0, 0,
		0, 0, 0, 0,
	)
	tm.hwnd = hwnd

	tm.updateIcon("ReflexGate (Stopped)")

	var msg MSG
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	tm.removeIcon()
}

func (tm *TrayManager) updateIcon(tip string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.hwnd == 0 {
		return
	}
	hIcon, _, _ := pLoadIconW.Call(0, 32512) // IDI_APPLICATION
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = tm.hwnd
	nid.UID = 1001
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_TRAY
	nid.HIcon = hIcon

	tip16, _ := windows.UTF16FromString(tip)
	for i, c := range tip16 {
		if i < len(nid.SzTip)-1 {
			nid.SzTip[i] = c
		}
	}

	op := NIM_ADD
	if tm.running {
		op = NIM_MODIFY
	}
	pShell_NotifyIconW.Call(uintptr(op), uintptr(unsafe.Pointer(&nid)))
	tm.running = true
}

func (tm *TrayManager) removeIcon() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.hwnd == 0 {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = tm.hwnd
	nid.UID = 1001
	pShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
}

func cleanupTray() {
	if globalTray != nil {
		globalTray.removeIcon()
	}
}

func trayWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAY:
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			if globalTray != nil && globalTray.app != nil && globalTray.app.ctx != nil {
				wruntime.WindowShow(globalTray.app.ctx)
				wruntime.WindowUnminimise(globalTray.app.ctx)
			}
		case WM_RBUTTONUP:
			if globalTray != nil && globalTray.app != nil {
				showTrayMenu(hwnd)
			}
		}
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func showTrayMenu(hwnd uintptr) {
	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	hMenu, _, _ := pCreatePopupMenu.Call()
	defer pDestroyMenu.Call(hMenu)

	isServerRunning := false
	isJa := true
	if globalTray != nil && globalTray.app != nil {
		globalTray.app.mu.Lock()
		isServerRunning = globalTray.app.apiRunning
		isJa = globalTray.app.cfg.Lang != "en"
		globalTray.app.mu.Unlock()
	}

	strOpen := "ウィンドウを表示"
	strHide := "ウィンドウを隠す"
	strStart := "サーバー起動"
	strStop := "サーバー停止"
	strExit := "終了"

	if !isJa {
		strOpen = "Show Window"
		strHide = "Hide Window"
		strStart = "Start Server"
		strStop = "Stop Server"
		strExit = "Exit"
	}

	pAppendMenuW.Call(hMenu, 0, 1, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(strOpen))))
	pAppendMenuW.Call(hMenu, 0, 2, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(strHide))))
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	if isServerRunning {
		pAppendMenuW.Call(hMenu, 0, 3, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(strStop))))
	} else {
		pAppendMenuW.Call(hMenu, 0, 4, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(strStart))))
	}
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
	pAppendMenuW.Call(hMenu, 0, 5, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(strExit))))

	pSetForegroundWindow.Call(hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(hMenu, TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)

	if globalTray == nil || globalTray.app == nil || globalTray.app.ctx == nil {
		return
	}

	switch cmd {
	case 1:
		wruntime.WindowShow(globalTray.app.ctx)
		wruntime.WindowUnminimise(globalTray.app.ctx)
	case 2:
		wruntime.WindowHide(globalTray.app.ctx)
	case 3:
		_ = globalTray.app.StopServer()
	case 4:
		go func() {
			globalTray.app.mu.Lock()
			cfg := globalTray.app.cfg
			globalTray.app.mu.Unlock()
			dto := ConfigDTO{
				LlamaServer: cfg.LlamaServer,
				Model:       cfg.Model,
				Backend:     cfg.Backend,
				Context:     cfg.Context,
				GPULayers:   cfg.GPULayers,
				JevPort:     cfg.JevPort,
				LlamaPort:   cfg.LlamaPort,
				Host:        cfg.Host,
				Lang:        cfg.Lang,
			}
			_ = globalTray.app.StartServer(dto)
		}()
	case 5:
		if globalTray != nil && globalTray.app != nil {
			_ = globalTray.app.StopServer()
		}
		wruntime.Quit(globalTray.app.ctx)
	}
}
