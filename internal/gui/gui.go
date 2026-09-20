//go:build windows

package gui

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"local-jev/internal/api"
	"local-jev/internal/config"
	"local-jev/internal/download"
	"local-jev/internal/hw"
	"local-jev/internal/proc"
)

const (
	WS_OVERLAPPED       = 0x00000000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_MINIMIZEBOX      = 0x00020000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_BORDER           = 0x00800000
	WS_TABSTOP          = 0x00010000
	WS_VSCROLL          = 0x00200000

	BS_PUSHBUTTON       = 0x00000000
	BS_DEFPUSHBUTTON    = 0x00000001
	BS_GROUPBOX         = 0x00000007
	BS_OWNERDRAW        = 0x0000000B

	ES_LEFT             = 0x00000000
	ES_AUTOHSCROLL      = 0x00000080

	SS_LEFT             = 0x00000000

	CBS_DROPDOWNLIST    = 0x00000003
	LBS_NOTIFY          = 0x0001

	WM_DESTROY          = 0x0002
	WM_CLOSE            = 0x0010
	WM_PAINT            = 0x000F
	WM_ERASEBKGND       = 0x0014
	WM_SETFONT          = 0x0030
	WM_COMMAND          = 0x0111
	WM_SYSCOMMAND       = 0x0112
	WM_TIMER            = 0x0113
	WM_DROPFILES        = 0x0233
	WM_DRAWITEM         = 0x002B

	WM_CTLCOLORMSGBOX   = 0x0132
	WM_CTLCOLOREDIT     = 0x0133
	WM_CTLCOLORLISTBOX  = 0x0134
	WM_CTLCOLORBTN      = 0x0135
	WM_CTLCOLORDLG      = 0x0136
	WM_CTLCOLORSCROLLBAR= 0x0137
	WM_CTLCOLORSTATIC   = 0x0138

	WM_LBUTTONUP        = 0x0202
	WM_LBUTTONDBLCLK    = 0x0203
	WM_RBUTTONUP        = 0x0205

	SC_MINIMIZE         = 0xF020

	CB_ADDSTRING        = 0x0143
	CB_SETCURSEL        = 0x014E
	CB_GETCURSEL        = 0x0147
	CB_RESETCONTENT     = 0x014B

	LB_ADDSTRING        = 0x0180
	LB_DELETESTRING     = 0x0182
	LB_RESETCONTENT     = 0x0184
	LB_GETTEXT          = 0x0189
	LB_GETTEXTLEN       = 0x018A
	LB_GETCOUNT         = 0x018B
	LB_SETTOPINDEX      = 0x0197

	SW_HIDE             = 0
	SW_SHOWNORMAL       = 1
	SW_SHOW             = 5

	OFN_FILEMUSTEXIST   = 0x00001000
	OFN_PATHMUSTEXIST   = 0x00000800

	MB_OK               = 0x00000000
	MB_ICONWARNING      = 0x00000030
	MB_ICONERROR        = 0x00000010
	MB_ICONINFORMATION  = 0x00000040

	SM_CXSCREEN         = 0
	SM_CYSCREEN         = 1

	SRCCOPY             = 0x00CC0020
	PS_SOLID            = 0

	// Tray constants
	NIM_ADD             = 0x00000000
	NIM_MODIFY          = 0x00000001
	NIM_DELETE          = 0x00000002
	NIF_MESSAGE         = 0x00000001
	NIF_ICON            = 0x00000002
	NIF_TIP             = 0x00000004

	TPM_RETURNCMD       = 0x0100
	MF_SEPARATOR        = 0x0800

	IDI_APPLICATION     = 32512

	WM_APP_STATUS       = 0x8001
	WM_APP_READY        = 0x8002
	WM_APP_FAILED       = 0x8003
	WM_APP_DL_PROG      = 0x8004
	WM_APP_DL_DONE      = 0x8005
	WM_APP_DL_FAIL      = 0x8006
	WM_APP_EVENT        = 0x8007
	WM_APP_TRAY         = 0x8008
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	procRegisterClassExW     = user32.NewProc("RegisterClassExW")
	procCreateWindowExW      = user32.NewProc("CreateWindowExW")
	procDefWindowProcW       = user32.NewProc("DefWindowProcW")
	procDestroyWindow        = user32.NewProc("DestroyWindow")
	procPostQuitMessage      = user32.NewProc("PostQuitMessage")
	procShowWindow           = user32.NewProc("ShowWindow")
	procUpdateWindow         = user32.NewProc("UpdateWindow")
	procGetMessageW          = user32.NewProc("GetMessageW")
	procTranslateMessage     = user32.NewProc("TranslateMessage")
	procDispatchMessageW     = user32.NewProc("DispatchMessageW")
	procSendMessageW         = user32.NewProc("SendMessageW")
	procPostMessageW         = user32.NewProc("PostMessageW")
	procSetWindowTextW       = user32.NewProc("SetWindowTextW")
	procGetWindowTextW       = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW = user32.NewProc("GetWindowTextLengthW")
	procEnableWindow         = user32.NewProc("EnableWindow")
	procMessageBoxW          = user32.NewProc("MessageBoxW")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procBeginPaint           = user32.NewProc("BeginPaint")
	procEndPaint             = user32.NewProc("EndPaint")
	procFillRect             = user32.NewProc("FillRect")
	procInvalidateRect       = user32.NewProc("InvalidateRect")
	procGetClientRect        = user32.NewProc("GetClientRect")
	procSetTimer             = user32.NewProc("SetTimer")
	procKillTimer            = user32.NewProc("KillTimer")
	procOpenClipboard        = user32.NewProc("OpenClipboard")
	procCloseClipboard       = user32.NewProc("CloseClipboard")
	procEmptyClipboard       = user32.NewProc("EmptyClipboard")
	procSetClipboardData     = user32.NewProc("SetClipboardData")
	procCreatePopupMenu      = user32.NewProc("CreatePopupMenu")
	procAppendMenuW          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu       = user32.NewProc("TrackPopupMenu")
	procDestroyMenu          = user32.NewProc("DestroyMenu")
	procGetCursorPos         = user32.NewProc("GetCursorPos")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procLoadIconW            = user32.NewProc("LoadIconW")
	procDrawTextW            = user32.NewProc("DrawTextW")

	procGetModuleHandleW     = kernel32.NewProc("GetModuleHandleW")
	procGlobalAlloc          = kernel32.NewProc("GlobalAlloc")
	procGlobalLock           = kernel32.NewProc("GlobalLock")
	procGlobalUnlock         = kernel32.NewProc("GlobalUnlock")
	procGlobalFree           = kernel32.NewProc("GlobalFree")
	procRtlMoveMemory        = kernel32.NewProc("RtlMoveMemory")

	procCreateFontW          = gdi32.NewProc("CreateFontW")
	procDeleteObject         = gdi32.NewProc("DeleteObject")
	procCreateSolidBrush     = gdi32.NewProc("CreateSolidBrush")
	procSetTextColor         = gdi32.NewProc("SetTextColor")
	procSetBkColor           = gdi32.NewProc("SetBkColor")
	procSetBkMode            = gdi32.NewProc("SetBkMode")
	procCreatePen            = gdi32.NewProc("CreatePen")
	procSelectObject         = gdi32.NewProc("SelectObject")
	procPolyline             = gdi32.NewProc("Polyline")
	procRectangle            = gdi32.NewProc("Rectangle")
	procEllipse              = gdi32.NewProc("Ellipse")
	procRoundRect            = gdi32.NewProc("RoundRect")
	procCreateCompatibleDC   = gdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	procBitBlt               = gdi32.NewProc("BitBlt")
	procDeleteDC             = gdi32.NewProc("DeleteDC")
	procTextOutW             = gdi32.NewProc("TextOutW")
	procMoveToEx             = gdi32.NewProc("MoveToEx")
	procLineTo               = gdi32.NewProc("LineTo")

	procDragAcceptFiles      = shell32.NewProc("DragAcceptFiles")
	procDragQueryFileW       = shell32.NewProc("DragQueryFileW")
	procDragFinish           = shell32.NewProc("DragFinish")
	procShellExecuteW        = shell32.NewProc("ShellExecuteW")
	procShell_NotifyIconW    = shell32.NewProc("Shell_NotifyIconW")

	procGetOpenFileNameW     = comdlg32.NewProc("GetOpenFileNameW")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

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

type MSG struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       struct{ X, Y int32 }
	LPrivate uint32
}

type PAINTSTRUCT struct {
	Hdc         uintptr
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type POINT struct {
	X, Y int32
}

type DRAWITEMSTRUCT struct {
	CtlType    uint32
	CtlID      uint32
	ItemID     uint32
	ItemAction uint32
	ItemState  uint32
	_          uint32
	HwndItem   uintptr
	HDC        uintptr
	RcItem     RECT
	ItemData   uintptr
}

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
	UTimeoutOrVersion uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
}

type OPENFILENAMEW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}

func rgb(r, g, b byte) uintptr {
	return uintptr(r) | (uintptr(g) << 8) | (uintptr(b) << 16)
}

type App struct {
	hwndMain        uintptr
	hEditLlama      uintptr
	hEditModel      uintptr
	hComboCtx       uintptr
	hComboNGL       uintptr
	hEditJevPort    uintptr
	hEditLlamaPort  uintptr
	hComboHost      uintptr
	hBtnCopyURL     uintptr
	hBtnAutoDL      uintptr
	hLblDLProgress  uintptr
	hBtnStart       uintptr
	hBtnStop        uintptr
	hBtnDemo        uintptr
	hLblStatus      uintptr

	hFont           uintptr
	hFontBold       uintptr
	hFontMeter      uintptr
	hFontSmall      uintptr

	// Dark Theme Brushes & Pens
	hBrushBg        uintptr // #141416
	hBrushPanel     uintptr // #1C1C20
	hBrushCtrl      uintptr // #101012
	hBrushCanvasBg  uintptr // #0D0D10
	hBrushFast      uintptr // #00FF66
	hBrushCyan      uintptr // #00E5FF
	hBrushWarn      uintptr // #FFD600
	hBrushSlow      uintptr // #FF3366
	hBrushLedIdle   uintptr // #183020
	hPenGrid        uintptr // #282830
	hPenFast        uintptr // #00FF66 (width 2)
	hPenCyan        uintptr // #00E5FF (width 2)
	hPenTrack       uintptr // #303038

	// Owner-draw button brushes & pens
	hBrushBtn           uintptr
	hBrushBtnPress      uintptr
	hBrushBtnDisabled   uintptr
	hBrushBtnStart      uintptr
	hBrushBtnStartPress uintptr
	hBrushBtnStop       uintptr
	hBrushBtnStopPress  uintptr
	hBrushBtnLang       uintptr
	hBrushBtnDemo       uintptr
	hPenBtnBorder       uintptr
	hPenBtnBorderLight  uintptr
	hPenBtnDisabled     uintptr
	hPenBtnGreen        uintptr
	hPenBtnRed          uintptr
	hPenBtnBlue         uintptr

	// Metric Displays
	hLblLatencyVal  uintptr
	hLblTTFTVal     uintptr
	hLblSpeedVal    uintptr
	hLblCallsVal    uintptr

	// Visual Canvas
	hCanvas         uintptr
	slot0Active     bool
	slot1Active     bool
	lastLatencyMs   int64
	latencyHistory  []int64 // up to 30 values

	// Quick Test Buttons
	hBtnTestStop    uintptr
	hBtnTestExtract uintptr
	hBtnTestScan    uintptr

	// Activity Log
	hBtnLogCopy     uintptr
	hBtnLogClear    uintptr
	hListLog        uintptr

	// Controls for I18n text updates
	hTitle          uintptr
	hLblSummary     uintptr
	hBtnLang        uintptr
	hGrpServer      uintptr
	hLblLlama       uintptr
	hLblModel       uintptr
	hBtnBrowseLlama uintptr
	hBtnBrowseModel uintptr
	hGrpExec        uintptr
	hLblContext     uintptr
	hLblGPU         uintptr
	hLblJevPort     uintptr
	hLblLlamaPort   uintptr
	hLblQuickTest   uintptr
	hGrpMonitor     uintptr
	hLblLatencyHdr  uintptr
	hLblTTFTHdr     uintptr
	hLblSpeedHdr    uintptr
	hLblCallsHdr    uintptr
	hLblRecentLogs  uintptr

	// State
	totalCalls      int
	eventMu         sync.Mutex
	pendingEvents   []api.RequestEvent
	inTray          bool

	downloadedLlama string
	downloadedModel string

	cfg             config.Config
	hw              hw.Info
	runner          proc.Runner
	api             *api.Server
	apiRunning      bool
	starting        bool
}

var globalApp *App

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func getCtrlText(hwnd uintptr) string {
	lenVal, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if lenVal == 0 {
		return ""
	}
	buf := make([]uint16, lenVal+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}

func setCtrlText(hwnd uintptr, s string) {
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(s))))
	procInvalidateRect.Call(hwnd, 0, 1)
}

func enableCtrl(hwnd uintptr, enabled bool) {
	val := uintptr(0)
	if enabled {
		val = 1
	}
	procEnableWindow.Call(hwnd, val)
	procInvalidateRect.Call(hwnd, 0, 1)
}

func copyToClipboard(owner uintptr, text string) bool {
	u16 := syscall.StringToUTF16(text)
	cb := len(u16) * 2
	hMem, _, _ := procGlobalAlloc.Call(0x0042 /* GMEM_MOVEABLE | GMEM_ZEROINIT */, uintptr(cb))
	if hMem == 0 {
		return false
	}
	pMem, _, _ := procGlobalLock.Call(hMem)
	if pMem == 0 {
		procGlobalFree.Call(hMem)
		return false
	}
	procRtlMoveMemory.Call(pMem, uintptr(unsafe.Pointer(&u16[0])), uintptr(cb))
	procGlobalUnlock.Call(hMem)

	r, _, _ := procOpenClipboard.Call(owner)
	if r == 0 {
		procGlobalFree.Call(hMem)
		return false
	}
	procEmptyClipboard.Call()
	procSetClipboardData.Call(13 /* CF_UNICODETEXT */, hMem)
	procCloseClipboard.Call()
	return true
}

func openFileDialog(owner uintptr, title string, filter string) (string, bool) {
	buf := make([]uint16, 1024)
	var ofn OPENFILENAMEW
	ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
	ofn.hwndOwner = owner
	ofn.lpstrFilter = utf16Ptr(filter)
	ofn.lpstrFile = &buf[0]
	ofn.nMaxFile = uint32(len(buf))
	ofn.lpstrTitle = utf16Ptr(title)
	ofn.Flags = OFN_FILEMUSTEXIST | OFN_PATHMUSTEXIST

	ret, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ret == 0 {
		return "", false
	}
	return syscall.UTF16ToString(buf), true
}

func (a *App) openLlamaDialog(owner uintptr) (string, bool) {
	title := "llama-server.exe を選択"
	filter := "実行ファイル (*.exe)\x00*.exe\x00すべてのファイル (*.*)\x00*.*\x00\x00"
	if a.cfg.Lang == "en" {
		title = "Select llama-server.exe"
		filter = "Executable (*.exe)\x00*.exe\x00All Files (*.*)\x00*.*\x00\x00"
	}
	return openFileDialog(owner, title, filter)
}

func (a *App) openModelDialog(owner uintptr) (string, bool) {
	title := "Qwen3.5-0.8B GGUF モデルを選択"
	filter := "GGUF モデル (*.gguf)\x00*.gguf\x00すべてのファイル (*.*)\x00*.*\x00\x00"
	if a.cfg.Lang == "en" {
		title = "Select Qwen3.5-0.8B GGUF Model"
		filter = "GGUF Model (*.gguf)\x00*.gguf\x00All Files (*.*)\x00*.*\x00\x00"
	}
	return openFileDialog(owner, title, filter)
}

func autoDetectCandidates(cfg *config.Config) {
	if cfg.LlamaServer == "" {
		candidates := []string{
			`..\..\build-hip\bin\llama-server.exe`,
			`..\..\build-vulkan\bin\llama-server.exe`,
			`..\..\build\bin\llama-server.exe`,
			`bin\llama.cpp\llama-server.exe`,
			`llama-server.exe`,
		}
		for _, cand := range candidates {
			abs, err := filepath.Abs(filepath.Join(config.DataDir(), cand))
			if err == nil {
				if _, err := os.Stat(abs); err == nil {
					cfg.LlamaServer = abs
					break
				}
			}
		}
	}
	if cfg.Model == "" {
		userProfile := os.Getenv("USERPROFILE")
		candidates := []string{
			filepath.Join(userProfile, `.lmstudio\models\unsloth\Qwen3.5-0.8B-GGUF\Qwen3.5-0.8B-Q8_0.gguf`),
			filepath.Join(config.DataDir(), `models\Qwen3.5-0.8B-Q8_0.gguf`),
			filepath.Join(config.DataDir(), `qwen3.5-0.8b-instruct-q4_k_m.gguf`),
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				cfg.Model = cand
				break
			}
		}
	}
}

func canvasWndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := globalApp
	switch msg {
	case WM_ERASEBKGND:
		return 1

	case WM_PAINT:
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		if hdc == 0 {
			return 0
		}
		var rc RECT
		procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		w := rc.Right - rc.Left
		h := rc.Bottom - rc.Top

		memDC, _, _ := procCreateCompatibleDC.Call(hdc)
		memBmp, _, _ := procCreateCompatibleBitmap.Call(hdc, uintptr(w), uintptr(h))
		oldBmp, _, _ := procSelectObject.Call(memDC, memBmp)

		// 1. Background
		procFillRect.Call(memDC, uintptr(unsafe.Pointer(&rc)), a.hBrushCanvasBg)
		procRectangle.Call(memDC, 0, 0, uintptr(w), uintptr(h))

		procSetBkMode.Call(memDC, 1 /* TRANSPARENT */)
		if a.hFontSmall != 0 {
			procSelectObject.Call(memDC, a.hFontSmall)
		}

		t := getI18n(a.cfg.Lang)

		// 2. Dual Slot LEDs (Top Row, Y: 8..26)
		// Slot 0 LED
		s0Color := rgb(96, 112, 104)
		s0Brush := a.hBrushLedIdle
		s0Text := t.Slot0Idle
		if a.slot0Active {
			s0Color = rgb(0, 255, 102)
			s0Brush = a.hBrushFast
			s0Text = t.Slot0Active
		}
		oldBrush, _, _ := procSelectObject.Call(memDC, s0Brush)
		procEllipse.Call(memDC, 16, 9, 28, 21)
		procSetTextColor.Call(memDC, s0Color)
		s0U16 := utf16Ptr(s0Text)
		procTextOutW.Call(memDC, 34, 7, uintptr(unsafe.Pointer(s0U16)), uintptr(len(s0Text)))

		// Slot 1 LED
		s1Color := rgb(96, 112, 104)
		s1Brush := a.hBrushLedIdle
		s1Text := t.Slot1Idle
		if a.slot1Active {
			s1Color = rgb(0, 229, 255)
			s1Brush = a.hBrushCyan
			s1Text = t.Slot1Active
		}
		procSelectObject.Call(memDC, s1Brush)
		procEllipse.Call(memDC, 245, 9, 257, 21)
		procSetTextColor.Call(memDC, s1Color)
		s1U16 := utf16Ptr(s1Text)
		procTextOutW.Call(memDC, 263, 7, uintptr(unsafe.Pointer(s1U16)), uintptr(len(s1Text)))

		// 3. Latency Bar Gauge (Y: 28..44)
		procSetTextColor.Call(memDC, rgb(200, 205, 215))
		lblMeter := t.MeterLabel
		procTextOutW.Call(memDC, 16, 29, uintptr(unsafe.Pointer(utf16Ptr(lblMeter))), uintptr(len(lblMeter)))

		trackRc := RECT{Left: 110, Top: 30, Right: w - 110, Bottom: 42}
		procFillRect.Call(memDC, uintptr(unsafe.Pointer(&trackRc)), a.hBrushCtrl)

		barW := int32(0)
		lat := a.lastLatencyMs
		trackW := trackRc.Right - trackRc.Left
		if lat > 0 {
			barW = int32(float64(lat) / 600.0 * float64(trackW))
			if barW > trackW {
				barW = trackW
			}
			if barW < 4 {
				barW = 4
			}
		}
		if barW > 0 {
			fillRc := RECT{Left: trackRc.Left, Top: trackRc.Top, Right: trackRc.Left + barW, Bottom: trackRc.Bottom}
			barBrush := a.hBrushFast
			if lat >= 350 {
				barBrush = a.hBrushSlow
			} else if lat >= 150 {
				barBrush = a.hBrushWarn
			}
			procFillRect.Call(memDC, uintptr(unsafe.Pointer(&fillRc)), barBrush)
		}

		// Rating tag on the right
		ratingText := "---"
		ratingColor := rgb(120, 120, 130)
		if lat > 0 {
			if lat < 150 {
				ratingText = fmt.Sprintf("%dms (FAST)", lat)
				ratingColor = rgb(0, 255, 102)
			} else if lat < 350 {
				ratingText = fmt.Sprintf("%dms (NORM)", lat)
				ratingColor = rgb(255, 214, 0)
			} else {
				ratingText = fmt.Sprintf("%dms (SLOW)", lat)
				ratingColor = rgb(255, 51, 102)
			}
		}
		procSetTextColor.Call(memDC, ratingColor)
		procTextOutW.Call(memDC, uintptr(w-100), 29, uintptr(unsafe.Pointer(utf16Ptr(ratingText))), uintptr(len(ratingText)))

		// 4. Sparkline Waveform (Y: 50..132)
		gw := w - 32
		gh := int32(78)
		gx := int32(16)
		gy := int32(50)

		gRc := RECT{Left: gx, Top: gy, Right: gx + gw, Bottom: gy + gh}
		procFillRect.Call(memDC, uintptr(unsafe.Pointer(&gRc)), a.hBrushCtrl)

		// Grid lines at 500ms and 200ms
		oldPen, _, _ := procSelectObject.Call(memDC, a.hPenGrid)
		y500 := gy + gh - int32(float64(gh)*500.0/600.0)
		y200 := gy + gh - int32(float64(gh)*200.0/600.0)

		procMoveToEx.Call(memDC, uintptr(gx), uintptr(y500), 0)
		procLineTo.Call(memDC, uintptr(gx+gw), uintptr(y500))
		procMoveToEx.Call(memDC, uintptr(gx), uintptr(y200), 0)
		procLineTo.Call(memDC, uintptr(gx+gw), uintptr(y200))

		procSetTextColor.Call(memDC, rgb(140, 150, 165))
		procTextOutW.Call(memDC, uintptr(gx+4), uintptr(y500-11), uintptr(unsafe.Pointer(utf16Ptr("500ms"))), 5)
		procTextOutW.Call(memDC, uintptr(gx+4), uintptr(y200-11), uintptr(unsafe.Pointer(utf16Ptr("200ms"))), 5)

		// Plot points
		nPoints := len(a.latencyHistory)
		if nPoints > 1 {
			pts := make([]POINT, nPoints)
			xStep := float64(gw-20) / 29.0
			for i, v := range a.latencyHistory {
				px := gx + 10 + int32(float64(i)*xStep)
				py := gy + gh - int32(float64(v)/600.0*float64(gh-6))
				if py < gy+2 {
					py = gy + 2
				}
				if py > gy+gh-2 {
					py = gy + gh - 2
				}
				pts[i] = POINT{X: px, Y: py}
			}
			procSelectObject.Call(memDC, a.hPenFast)
			procPolyline.Call(memDC, uintptr(unsafe.Pointer(&pts[0])), uintptr(nPoints))

			// Dot on last point
			lastPt := pts[nPoints-1]
			procSelectObject.Call(memDC, a.hBrushCyan)
			procEllipse.Call(memDC, uintptr(lastPt.X-3), uintptr(lastPt.Y-3), uintptr(lastPt.X+4), uintptr(lastPt.Y+4))
		}

		procSelectObject.Call(memDC, oldPen)
		procSelectObject.Call(memDC, oldBrush)

		// Blit to screen
		procBitBlt.Call(hdc, 0, 0, uintptr(w), uintptr(h), memDC, 0, 0, SRCCOPY)

		procSelectObject.Call(memDC, oldBmp)
		procDeleteObject.Call(memBmp)
		procDeleteDC.Call(memDC)
		procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	a := globalApp

	switch msg {
	case WM_SYSCOMMAND:
		if wParam&0xFFF0 == SC_MINIMIZE {
			if a != nil {
				a.minimizeToTray()
				return 0
			}
		}

	case WM_APP_TRAY:
		switch lParam {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK:
			if a != nil {
				a.restoreFromTray()
			}
		case WM_RBUTTONUP:
			if a != nil {
				var pt POINT
				procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
				hMenu, _, _ := procCreatePopupMenu.Call()
				t := getI18n(a.cfg.Lang)
				procAppendMenuW.Call(hMenu, 0, 4001, uintptr(unsafe.Pointer(utf16Ptr(t.TrayOpen))))
				procAppendMenuW.Call(hMenu, 0, 4002, uintptr(unsafe.Pointer(utf16Ptr(t.TrayStop))))
				procAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)
				procAppendMenuW.Call(hMenu, 0, 4003, uintptr(unsafe.Pointer(utf16Ptr(t.TrayExit))))
				procSetForegroundWindow.Call(hwnd)
				cmd, _, _ := procTrackPopupMenu.Call(hMenu, TPM_RETURNCMD, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
				procDestroyMenu.Call(hMenu)
				switch cmd {
				case 4001:
					a.restoreFromTray()
				case 4002:
					a.stop()
				case 4003:
					a.cleanup()
					procDestroyWindow.Call(hwnd)
				}
			}
		}
		return 0

	case WM_DRAWITEM:
		dis := (*DRAWITEMSTRUCT)(*(*unsafe.Pointer)(unsafe.Pointer(&lParam)))
		if dis == nil || a == nil {
			return 0
		}
		rc := dis.RcItem
		isPressed := (dis.ItemState & 0x0001) != 0
		isDisabled := (dis.ItemState & 0x0004) != 0

		btnBrush := a.hBrushBtn
		btnPen := a.hPenBtnBorder
		textColor := rgb(230, 235, 245)

		switch dis.HwndItem {
		case a.hBtnStart:
			if isDisabled {
				btnBrush = a.hBrushBtnDisabled
				btnPen = a.hPenBtnDisabled
				textColor = rgb(90, 95, 105)
			} else if isPressed {
				btnBrush = a.hBrushBtnStartPress
				btnPen = a.hPenBtnGreen
				textColor = rgb(255, 255, 255)
			} else {
				btnBrush = a.hBrushBtnStart
				btnPen = a.hPenBtnGreen
				textColor = rgb(255, 255, 255)
			}
		case a.hBtnStop:
			if isDisabled {
				btnBrush = a.hBrushBtnDisabled
				btnPen = a.hPenBtnDisabled
				textColor = rgb(90, 95, 105)
			} else if isPressed {
				btnBrush = a.hBrushBtnStopPress
				btnPen = a.hPenBtnRed
				textColor = rgb(255, 255, 255)
			} else {
				btnBrush = a.hBrushBtnStop
				btnPen = a.hPenBtnRed
				textColor = rgb(255, 255, 255)
			}
		case a.hBtnLang:
			btnBrush = a.hBrushBtnLang
			btnPen = a.hPenBtnBlue
			textColor = rgb(56, 189, 248)
			if isPressed {
				textColor = rgb(255, 255, 255)
			}
		case a.hBtnDemo:
			if isDisabled {
				btnBrush = a.hBrushBtnDisabled
				btnPen = a.hPenBtnDisabled
				textColor = rgb(90, 95, 105)
			} else {
				btnBrush = a.hBrushBtnDemo
				btnPen = a.hPenBtnBlue
				textColor = rgb(186, 230, 253)
			}
		default:
			if isDisabled {
				btnBrush = a.hBrushBtnDisabled
				btnPen = a.hPenBtnDisabled
				textColor = rgb(90, 95, 105)
			} else if isPressed {
				btnBrush = a.hBrushBtnPress
				btnPen = a.hPenBtnBorderLight
				textColor = rgb(255, 255, 255)
			}
		}

		oldBrush, _, _ := procSelectObject.Call(dis.HDC, btnBrush)
		oldPen, _, _ := procSelectObject.Call(dis.HDC, btnPen)
		procRoundRect.Call(dis.HDC, uintptr(rc.Left), uintptr(rc.Top), uintptr(rc.Right), uintptr(rc.Bottom), 6, 6)

		txt := getCtrlText(dis.HwndItem)
		if txt != "" {
			procSetBkMode.Call(dis.HDC, 1 /* TRANSPARENT */)
			procSetTextColor.Call(dis.HDC, textColor)
			fontToUse := a.hFont
			if dis.HwndItem == a.hBtnStart || dis.HwndItem == a.hBtnStop {
				fontToUse = a.hFontBold
			}
			if fontToUse != 0 {
				procSelectObject.Call(dis.HDC, fontToUse)
			}
			textRc := rc
			if isPressed {
				textRc.Top += 1
				textRc.Left += 1
			}
			procDrawTextW.Call(dis.HDC, uintptr(unsafe.Pointer(utf16Ptr(txt))), ^uintptr(0), uintptr(unsafe.Pointer(&textRc)), 0x00000001|0x00000004|0x00000020 /* DT_CENTER | DT_VCENTER | DT_SINGLELINE */)
		}

		procSelectObject.Call(dis.HDC, oldPen)
		procSelectObject.Call(dis.HDC, oldBrush)
		return 1

	case WM_CTLCOLORDLG:
		if a != nil && a.hBrushBg != 0 {
			return a.hBrushBg
		}

	case WM_CTLCOLORSTATIC:
		if a == nil {
			break
		}
		hdc := wParam
		ctrl := lParam

		switch ctrl {
		case a.hEditLlama, a.hEditModel, a.hEditJevPort, a.hEditLlamaPort:
			procSetTextColor.Call(hdc, rgb(230, 235, 245))
			procSetBkColor.Call(hdc, rgb(20, 20, 24))
			return a.hBrushCtrl
		case a.hLblLatencyVal, a.hLblTTFTVal, a.hLblSpeedVal:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(0, 229, 255))
			return a.hBrushBg
		case a.hLblCallsVal:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(0, 255, 102))
			return a.hBrushBg
		case a.hLblStatus:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			if a.apiRunning {
				procSetTextColor.Call(hdc, rgb(0, 255, 102))
			} else if a.starting {
				procSetTextColor.Call(hdc, rgb(255, 214, 0))
			} else {
				procSetTextColor.Call(hdc, rgb(160, 165, 175))
			}
			return a.hBrushBg
		case a.hTitle:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(255, 255, 255))
			return a.hBrushBg
		case a.hLblSummary:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(140, 150, 165))
			return a.hBrushBg
		case a.hLblLatencyHdr, a.hLblTTFTHdr, a.hLblSpeedHdr, a.hLblCallsHdr:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(140, 145, 160))
			return a.hBrushBg
		default:
			procSetBkMode.Call(hdc, 1 /* TRANSPARENT */)
			procSetTextColor.Call(hdc, rgb(210, 215, 225))
			return a.hBrushBg
		}

	case WM_CTLCOLOREDIT:
		if a != nil {
			hdc := wParam
			procSetTextColor.Call(hdc, rgb(230, 235, 245))
			procSetBkColor.Call(hdc, rgb(20, 20, 24))
			return a.hBrushCtrl
		}

	case WM_CTLCOLORLISTBOX:
		if a != nil {
			hdc := wParam
			procSetTextColor.Call(hdc, rgb(215, 230, 225))
			procSetBkColor.Call(hdc, rgb(14, 14, 18))
			return a.hBrushCanvasBg
		}

	case WM_CTLCOLORBTN:
		if a != nil && a.hBrushBg != 0 {
			return a.hBrushBg
		}

	case WM_TIMER:
		if wParam == 1 && a != nil {
			procKillTimer.Call(hwnd, 1)
			a.slot0Active = false
			a.slot1Active = false
			procInvalidateRect.Call(a.hCanvas, 0, 0)
			return 0
		}

	case WM_COMMAND:
		ctrlID := int(wParam & 0xFFFF)
		switch ctrlID {
		case 1001: // Browse llama-server
			if p, ok := a.openLlamaDialog(hwnd); ok {
				setCtrlText(a.hEditLlama, p)
				a.cfg.LlamaServer = p
				_ = a.cfg.Save()
			}
			return 0
		case 1002: // Browse model
			if p, ok := a.openModelDialog(hwnd); ok {
				setCtrlText(a.hEditModel, p)
				a.cfg.Model = p
				_ = a.cfg.Save()
			}
			return 0
		case 1003: // Auto download
			if a != nil {
				a.autoDownload()
			}
			return 0
		case 2001: // JEV 起動
			if a != nil && !a.starting && !a.apiRunning {
				a.start()
			}
			return 0
		case 2002: // 停止
			if a != nil {
				a.stop()
			}
			return 0
		case 2003: // デモ画面を開く
			if a != nil {
				a.openDemo()
			}
			return 0
		case 2005: // URL コピー
			if a != nil {
				host := "127.0.0.1"
				if cur, _, _ := procSendMessageW.Call(a.hComboHost, CB_GETCURSEL, 0, 0); cur == 1 {
					host = "0.0.0.0"
				}
				port := strings.TrimSpace(getCtrlText(a.hEditJevPort))
				if port == "" {
					port = "8090"
				}
				url := fmt.Sprintf("http://%s:%s", host, port)
				if copyToClipboard(hwnd, url) {
					t := getI18n(a.cfg.Lang)
					setCtrlText(a.hLblStatus, t.MsgURLCopied)
				}
			}
			return 0
		case 2006: // 言語切替 (Toggle Language)
			if a != nil {
				if a.cfg.Lang == "en" {
					a.cfg.Lang = "ja"
				} else {
					a.cfg.Lang = "en"
				}
				_ = a.cfg.Save()
				a.updateLanguageTexts()
			}
			return 0
		case 3001: // Quick Test: Stop
			go a.testStop()
			return 0
		case 3002: // Quick Test: Extract
			go a.testExtract()
			return 0
		case 3003: // Quick Test: Scan
			go a.testScan()
			return 0
		case 3005: // ログコピー
			if a != nil {
				count, _, _ := procSendMessageW.Call(a.hListLog, LB_GETCOUNT, 0, 0)
				var lines []string
				for i := uintptr(0); i < count; i++ {
					lenVal, _, _ := procSendMessageW.Call(a.hListLog, LB_GETTEXTLEN, i, 0)
					if lenVal > 0 {
						buf := make([]uint16, lenVal+1)
						procSendMessageW.Call(a.hListLog, LB_GETTEXT, i, uintptr(unsafe.Pointer(&buf[0])))
						lines = append(lines, syscall.UTF16ToString(buf))
					}
				}
				if len(lines) > 0 {
					copyToClipboard(hwnd, strings.Join(lines, "\r\n"))
					t := getI18n(a.cfg.Lang)
					setCtrlText(a.hLblStatus, t.MsgLogCopied)
				}
			}
			return 0
		case 3006: // ログクリア
			if a != nil {
				procSendMessageW.Call(a.hListLog, LB_RESETCONTENT, 0, 0)
				a.latencyHistory = nil
				procInvalidateRect.Call(a.hCanvas, 0, 1)
				t := getI18n(a.cfg.Lang)
				setCtrlText(a.hLblStatus, t.MsgCleared)
			}
			return 0
		}

	case WM_DROPFILES:
		hDrop := wParam
		numFiles, _, _ := procDragQueryFileW.Call(hDrop, 0xFFFFFFFF, 0, 0)
		for i := uintptr(0); i < numFiles; i++ {
			buf := make([]uint16, 1024)
			procDragQueryFileW.Call(hDrop, i, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
			p := syscall.UTF16ToString(buf)
			low := strings.ToLower(p)
			if strings.HasSuffix(low, ".gguf") {
				setCtrlText(a.hEditModel, p)
				a.cfg.Model = p
				_ = a.cfg.Save()
			} else if strings.Contains(strings.ToLower(filepath.Base(p)), "llama-server") || strings.HasSuffix(low, ".exe") {
				setCtrlText(a.hEditLlama, p)
				a.cfg.LlamaServer = p
				_ = a.cfg.Save()
			}
		}
		procDragFinish.Call(hDrop)
		return 0

	case WM_APP_STATUS:
		if wParam != 0 {
			procSetWindowTextW.Call(a.hLblStatus, wParam)
		}
		return 0

	case WM_APP_READY:
		a.starting = false
		a.apiRunning = true
		port := a.cfg.JevPort
		if port <= 0 {
			port = 8090
		}
		t := getI18n(a.cfg.Lang)
		setCtrlText(a.hLblStatus, fmt.Sprintf(t.LblStatusRun, port))
		enableCtrl(a.hBtnStart, false)
		enableCtrl(a.hBtnStop, true)
		enableCtrl(a.hBtnDemo, true)
		enableCtrl(a.hBtnTestStop, true)
		enableCtrl(a.hBtnTestExtract, true)
		enableCtrl(a.hBtnTestScan, true)
		a.openDemo()
		return 0

	case WM_APP_FAILED:
		a.starting = false
		a.apiRunning = false
		t := getI18n(a.cfg.Lang)
		setCtrlText(a.hLblStatus, t.LblStatusFail)
		enableCtrl(a.hBtnStart, true)
		enableCtrl(a.hBtnStop, false)
		enableCtrl(a.hBtnDemo, false)
		enableCtrl(a.hBtnTestStop, false)
		enableCtrl(a.hBtnTestExtract, false)
		enableCtrl(a.hBtnTestScan, false)
		errMsg := "llama-server did not respond. Check configuration or logs."
		errTitle := "Launch Error"
		if a.cfg.Lang == "ja" {
			errMsg = "llama-server が応答しませんでした。設定またはログを確認してください。"
			errTitle = "起動エラー"
		}
		procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(errMsg))), uintptr(unsafe.Pointer(utf16Ptr(errTitle))), MB_OK|MB_ICONERROR)
		return 0

	case WM_APP_DL_PROG:
		if wParam != 0 {
			procSetWindowTextW.Call(a.hLblDLProgress, wParam)
		}
		return 0

	case WM_APP_DL_DONE:
		t := getI18n(a.cfg.Lang)
		enableCtrl(a.hBtnAutoDL, true)
		setCtrlText(a.hLblDLProgress, t.MsgDLDone)
		if a.downloadedLlama != "" {
			setCtrlText(a.hEditLlama, a.downloadedLlama)
			a.cfg.LlamaServer = a.downloadedLlama
		}
		if a.downloadedModel != "" {
			setCtrlText(a.hEditModel, a.downloadedModel)
			a.cfg.Model = a.downloadedModel
		}
		_ = a.cfg.Save()
		procMessageBoxW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(t.MsgDLDone))), uintptr(unsafe.Pointer(utf16Ptr("local-jev"))), MB_OK|MB_ICONINFORMATION)
		return 0

	case WM_APP_DL_FAIL:
		t := getI18n(a.cfg.Lang)
		enableCtrl(a.hBtnAutoDL, true)
		setCtrlText(a.hLblDLProgress, t.MsgDLFail)
		errTitle := "Error"
		if a.cfg.Lang == "ja" {
			errTitle = "エラー"
		}
		msgPtr := uintptr(unsafe.Pointer(utf16Ptr(t.MsgDLFail)))
		if wParam != 0 {
			msgPtr = wParam
		}
		procMessageBoxW.Call(hwnd, msgPtr, uintptr(unsafe.Pointer(utf16Ptr(errTitle))), MB_OK|MB_ICONERROR)
		return 0

	case WM_APP_EVENT:
		if a == nil {
			return 0
		}
		a.eventMu.Lock()
		events := a.pendingEvents
		a.pendingEvents = nil
		a.eventMu.Unlock()

		for _, ev := range events {
			a.totalCalls++
			a.lastLatencyMs = ev.LatencyMs
			a.latencyHistory = append(a.latencyHistory, ev.LatencyMs)
			if len(a.latencyHistory) > 30 {
				a.latencyHistory = a.latencyHistory[1:]
			}

			// Update Slot LEDs
			if ev.SlotID == 0 {
				a.slot0Active = true
			} else if ev.SlotID == 1 {
				a.slot1Active = true
			}
			procSetTimer.Call(a.hwndMain, 1, 250, 0)
			procInvalidateRect.Call(a.hCanvas, 0, 0)

			// Update Text Meters
			setCtrlText(a.hLblLatencyVal, fmt.Sprintf("%d ms", ev.LatencyMs))
			if ev.TTFTMs > 0 {
				setCtrlText(a.hLblTTFTVal, fmt.Sprintf("%d ms", ev.TTFTMs))
			} else {
				setCtrlText(a.hLblTTFTVal, "0 ms")
			}
			if ev.TokPerSec > 0 {
				setCtrlText(a.hLblSpeedVal, fmt.Sprintf("%.1f tok/s", ev.TokPerSec))
			} else {
				setCtrlText(a.hLblSpeedVal, "---")
			}
			t := getI18n(a.cfg.Lang)
			setCtrlText(a.hLblCallsVal, fmt.Sprintf(t.CallsFormat, a.totalCalls))

			// Add to Activity Log ListBox
			logLine := fmt.Sprintf("[%s] %s | %dms (TTFT %dms, %.0ft/s) -> %s",
				ev.Timestamp.Format("15:04:05"), ev.Task, ev.LatencyMs, ev.TTFTMs, ev.TokPerSec, ev.Summary)
			procSendMessageW.Call(a.hListLog, LB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(logLine))))

			// Cap listbox entries at 50
			count, _, _ := procSendMessageW.Call(a.hListLog, LB_GETCOUNT, 0, 0)
			if int(count) > 50 {
				procSendMessageW.Call(a.hListLog, LB_DELETESTRING, 0, 0)
				count--
			}
			if count > 0 {
				procSendMessageW.Call(a.hListLog, LB_SETTOPINDEX, count-1, 0)
			}
		}
		return 0

	case WM_CLOSE:
		if a != nil {
			a.cleanup()
		}
		procDestroyWindow.Call(hwnd)
		return 0

	case WM_DESTROY:
		if a != nil {
			a.removeTrayIcon()
			if a.hFont != 0 {
				procDeleteObject.Call(a.hFont)
			}
			if a.hFontBold != 0 {
				procDeleteObject.Call(a.hFontBold)
			}
			if a.hFontMeter != 0 {
				procDeleteObject.Call(a.hFontMeter)
			}
			if a.hFontSmall != 0 {
				procDeleteObject.Call(a.hFontSmall)
			}
			if a.hBrushBg != 0 {
				procDeleteObject.Call(a.hBrushBg)
			}
			if a.hBrushPanel != 0 {
				procDeleteObject.Call(a.hBrushPanel)
			}
			if a.hBrushCtrl != 0 {
				procDeleteObject.Call(a.hBrushCtrl)
			}
			if a.hBrushCanvasBg != 0 {
				procDeleteObject.Call(a.hBrushCanvasBg)
			}
			if a.hBrushFast != 0 {
				procDeleteObject.Call(a.hBrushFast)
			}
			if a.hBrushCyan != 0 {
				procDeleteObject.Call(a.hBrushCyan)
			}
			if a.hBrushWarn != 0 {
				procDeleteObject.Call(a.hBrushWarn)
			}
			if a.hBrushSlow != 0 {
				procDeleteObject.Call(a.hBrushSlow)
			}
			if a.hBrushLedIdle != 0 {
				procDeleteObject.Call(a.hBrushLedIdle)
			}
			if a.hPenGrid != 0 {
				procDeleteObject.Call(a.hPenGrid)
			}
			if a.hPenFast != 0 {
				procDeleteObject.Call(a.hPenFast)
			}
			if a.hPenCyan != 0 {
				procDeleteObject.Call(a.hPenCyan)
			}
			if a.hBrushBtn != 0 {
				procDeleteObject.Call(a.hBrushBtn)
			}
			if a.hBrushBtnPress != 0 {
				procDeleteObject.Call(a.hBrushBtnPress)
			}
			if a.hBrushBtnDisabled != 0 {
				procDeleteObject.Call(a.hBrushBtnDisabled)
			}
			if a.hBrushBtnStart != 0 {
				procDeleteObject.Call(a.hBrushBtnStart)
			}
			if a.hBrushBtnStartPress != 0 {
				procDeleteObject.Call(a.hBrushBtnStartPress)
			}
			if a.hBrushBtnStop != 0 {
				procDeleteObject.Call(a.hBrushBtnStop)
			}
			if a.hBrushBtnStopPress != 0 {
				procDeleteObject.Call(a.hBrushBtnStopPress)
			}
			if a.hBrushBtnLang != 0 {
				procDeleteObject.Call(a.hBrushBtnLang)
			}
			if a.hBrushBtnDemo != 0 {
				procDeleteObject.Call(a.hBrushBtnDemo)
			}
			if a.hPenBtnBorder != 0 {
				procDeleteObject.Call(a.hPenBtnBorder)
			}
			if a.hPenBtnBorderLight != 0 {
				procDeleteObject.Call(a.hPenBtnBorderLight)
			}
			if a.hPenBtnDisabled != 0 {
				procDeleteObject.Call(a.hPenBtnDisabled)
			}
			if a.hPenBtnGreen != 0 {
				procDeleteObject.Call(a.hPenBtnGreen)
			}
			if a.hPenBtnRed != 0 {
				procDeleteObject.Call(a.hPenBtnRed)
			}
			if a.hPenBtnBlue != 0 {
				procDeleteObject.Call(a.hPenBtnBlue)
			}
		}
		procPostQuitMessage.Call(0)
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (a *App) minimizeToTray() {
	if a.inTray {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwndMain
	nid.UID = 1
	nid.UFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.UCallbackMessage = WM_APP_TRAY
	hIcon, _, _ := procLoadIconW.Call(0, uintptr(IDI_APPLICATION))
	nid.HIcon = hIcon
	tip := utf16Ptr(fmt.Sprintf("local-jev (Port %d)", a.cfg.JevPort))
	for i, c := range syscall.StringToUTF16(fmt.Sprintf("local-jev (Port %d)", a.cfg.JevPort)) {
		if i < len(nid.SzTip) {
			nid.SzTip[i] = c
		}
	}
	_ = tip
	procShell_NotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	procShowWindow.Call(a.hwndMain, SW_HIDE)
	a.inTray = true
}

func (a *App) restoreFromTray() {
	if !a.inTray {
		return
	}
	a.removeTrayIcon()
	procShowWindow.Call(a.hwndMain, SW_SHOWNORMAL)
	procSetForegroundWindow.Call(a.hwndMain)
}

func (a *App) removeTrayIcon() {
	if !a.inTray {
		return
	}
	var nid NOTIFYICONDATAW
	nid.CbSize = uint32(unsafe.Sizeof(nid))
	nid.HWnd = a.hwndMain
	nid.UID = 1
	procShell_NotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	a.inTray = false
}

func (a *App) updateLanguageTexts() {
	t := getI18n(a.cfg.Lang)
	setCtrlText(a.hwndMain, t.WindowTitle)
	setCtrlText(a.hTitle, t.HeaderTitle)
	setCtrlText(a.hBtnLang, t.BtnLangToggle)
	if a.cfg.Lang == "en" {
		setCtrlText(a.hLblSummary, a.hw.SummaryEN())
	} else {
		setCtrlText(a.hLblSummary, a.hw.Summary())
	}
	setCtrlText(a.hGrpServer, t.GroupServer)
	setCtrlText(a.hLblLlama, t.LblLlama)
	setCtrlText(a.hLblModel, t.LblModel)
	setCtrlText(a.hBtnBrowseLlama, t.BtnBrowse)
	setCtrlText(a.hBtnBrowseModel, t.BtnBrowse)
	setCtrlText(a.hBtnAutoDL, t.BtnAutoDL)

	setCtrlText(a.hGrpExec, t.GroupExec)
	setCtrlText(a.hLblContext, t.LblContext)
	setCtrlText(a.hLblGPU, t.LblGPU)

	curNGL, _, _ := procSendMessageW.Call(a.hComboNGL, CB_GETCURSEL, 0, 0)
	procSendMessageW.Call(a.hComboNGL, CB_RESETCONTENT, 0, 0)
	procSendMessageW.Call(a.hComboNGL, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(t.OptGPUAll))))
	procSendMessageW.Call(a.hComboNGL, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(t.OptGPUCPU))))
	procSendMessageW.Call(a.hComboNGL, CB_SETCURSEL, curNGL, 0)
	setCtrlText(a.hLblJevPort, t.LblJevPort)
	setCtrlText(a.hLblLlamaPort, t.LblLlamaPort)
	setCtrlText(a.hBtnStart, t.BtnStart)
	setCtrlText(a.hBtnStop, t.BtnStop)
	setCtrlText(a.hBtnDemo, t.BtnDemo)
	setCtrlText(a.hBtnCopyURL, t.BtnCopyURL)
	setCtrlText(a.hLblQuickTest, t.LblQuickTest)
	setCtrlText(a.hBtnTestStop, t.BtnTestStop)
	setCtrlText(a.hBtnTestExtract, t.BtnTestExtract)
	setCtrlText(a.hBtnTestScan, t.BtnTestScan)

	setCtrlText(a.hGrpMonitor, t.GroupMonitor)
	setCtrlText(a.hLblLatencyHdr, t.LblLatency)
	setCtrlText(a.hLblTTFTHdr, t.LblTTFT)
	setCtrlText(a.hLblSpeedHdr, t.LblSpeed)
	setCtrlText(a.hLblCallsHdr, t.LblCalls)
	setCtrlText(a.hLblCallsVal, fmt.Sprintf(t.CallsFormat, a.totalCalls))
	setCtrlText(a.hLblRecentLogs, t.LblRecentLogs)
	setCtrlText(a.hBtnLogCopy, t.BtnLogCopy)
	setCtrlText(a.hBtnLogClear, t.BtnLogClear)

	if !a.apiRunning && !a.starting {
		setCtrlText(a.hLblStatus, t.LblStatusStop)
	} else if a.apiRunning {
		setCtrlText(a.hLblStatus, fmt.Sprintf(t.LblStatusRun, a.cfg.JevPort))
	} else if a.starting {
		setCtrlText(a.hLblStatus, t.LblStatusStart)
	}

	procInvalidateRect.Call(a.hCanvas, 0, 1)
}

func (a *App) testStop() {
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/jev/stop", port)
	payload := `{"log": "All tests passed. Task completed successfully."}`
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err == nil && resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func (a *App) testExtract() {
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/jev/extract", port)
	payload := `{"log": "Ran pytest: 3 passed. Modified internal/api/server.go and schema.go."}`
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err == nil && resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func (a *App) testScan() {
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/jev/scan", port)
	payload := `{"log": "Starting engine...\nError [E1029]: connection timed out on port 8080\nTraceback complete."}`
	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err == nil && resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func Run() {
	runtime.LockOSThread()

	cfg := config.Load()

	// Check CLI args or binary name to override language
	for i, arg := range os.Args {
		if (arg == "-lang" || arg == "--lang") && i+1 < len(os.Args) {
			cfg.Lang = strings.ToLower(os.Args[i+1])
		} else if strings.HasPrefix(arg, "-lang=") || strings.HasPrefix(arg, "--lang=") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				cfg.Lang = strings.ToLower(parts[1])
			}
		}
	}
	if strings.Contains(strings.ToLower(filepath.Base(os.Args[0])), "-en") {
		cfg.Lang = "en"
	}
	if cfg.Lang != "en" && cfg.Lang != "ja" {
		cfg.Lang = "ja"
	}

	autoDetectCandidates(&cfg)
	hwi := hw.Detect()

	if cfg.Mode == "" {
		cfg.Mode = "auto"
	}
	if cfg.Mode == "auto" {
		cfg.Backend = hwi.Backend
		cfg.GPULayers = hwi.GPULayers
	}
	if cfg.Context <= 0 {
		cfg.Context = 8192
	}
	if cfg.JevPort <= 0 {
		cfg.JevPort = 8090
	}
	if cfg.LlamaPort <= 0 {
		cfg.LlamaPort = 8080
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}

	a := &App{
		cfg: cfg,
		hw:  hwi,
	}
	globalApp = a

	t := getI18n(cfg.Lang)

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	mainClassName := utf16Ptr("LocalJevLauncherClass")
	canvasClassName := utf16Ptr("LocalJevCanvasClass")
	windowTitle := utf16Ptr(t.WindowTitle)

	// Create Theme Brushes & Pens
	a.hBrushBg, _, _ = procCreateSolidBrush.Call(rgb(20, 20, 22))        // #141416
	a.hBrushPanel, _, _ = procCreateSolidBrush.Call(rgb(28, 28, 32))     // #1C1C20
	a.hBrushCtrl, _, _ = procCreateSolidBrush.Call(rgb(24, 24, 28))      // #18181C
	a.hBrushCanvasBg, _, _ = procCreateSolidBrush.Call(rgb(14, 14, 18))  // #0E0E12
	a.hBrushFast, _, _ = procCreateSolidBrush.Call(rgb(0, 255, 102))     // #00FF66
	a.hBrushCyan, _, _ = procCreateSolidBrush.Call(rgb(0, 229, 255))     // #00E5FF
	a.hBrushWarn, _, _ = procCreateSolidBrush.Call(rgb(255, 214, 0))     // #FFD600
	a.hBrushSlow, _, _ = procCreateSolidBrush.Call(rgb(255, 51, 102))    // #FF3366
	a.hBrushLedIdle, _, _ = procCreateSolidBrush.Call(rgb(28, 48, 36))   // #1C3024

	a.hPenGrid, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(45, 55, 70))
	a.hPenFast, _, _ = procCreatePen.Call(PS_SOLID, 2, rgb(0, 255, 102))
	a.hPenCyan, _, _ = procCreatePen.Call(PS_SOLID, 2, rgb(0, 229, 255))

	// Button Brushes & Pens
	a.hBrushBtn, _, _ = procCreateSolidBrush.Call(rgb(36, 36, 44))
	a.hBrushBtnPress, _, _ = procCreateSolidBrush.Call(rgb(50, 50, 62))
	a.hBrushBtnDisabled, _, _ = procCreateSolidBrush.Call(rgb(24, 24, 28))
	a.hBrushBtnStart, _, _ = procCreateSolidBrush.Call(rgb(0, 140, 75))
	a.hBrushBtnStartPress, _, _ = procCreateSolidBrush.Call(rgb(0, 105, 55))
	a.hBrushBtnStop, _, _ = procCreateSolidBrush.Call(rgb(165, 30, 45))
	a.hBrushBtnStopPress, _, _ = procCreateSolidBrush.Call(rgb(125, 20, 35))
	a.hBrushBtnLang, _, _ = procCreateSolidBrush.Call(rgb(30, 41, 59))
	a.hBrushBtnDemo, _, _ = procCreateSolidBrush.Call(rgb(30, 58, 138))
	a.hPenBtnBorder, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(60, 60, 75))
	a.hPenBtnBorderLight, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(90, 90, 110))
	a.hPenBtnDisabled, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(38, 38, 46))
	a.hPenBtnGreen, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(0, 215, 100))
	a.hPenBtnRed, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(225, 55, 75))
	a.hPenBtnBlue, _, _ = procCreatePen.Call(PS_SOLID, 1, rgb(56, 189, 248))

	// Register Main Window Class
	var wc WNDCLASSEXW
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	wc.LpfnWndProc = syscall.NewCallback(wndProc)
	wc.HInstance = hInstance
	wc.LpszClassName = mainClassName
	wc.HbrBackground = a.hBrushBg
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Register Canvas Window Class
	var cwc WNDCLASSEXW
	cwc.CbSize = uint32(unsafe.Sizeof(cwc))
	cwc.LpfnWndProc = syscall.NewCallback(canvasWndProc)
	cwc.HInstance = hInstance
	cwc.LpszClassName = canvasClassName
	cwc.HbrBackground = a.hBrushCanvasBg
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&cwc)))

	winW := int32(740)
	winH := int32(845)
	scrW, _, _ := procGetSystemMetrics.Call(SM_CXSCREEN)
	scrH, _, _ := procGetSystemMetrics.Call(SM_CYSCREEN)
	winX := (int32(scrW) - winW) / 2
	winY := (int32(scrH) - winH) / 2

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(mainClassName)),
		uintptr(unsafe.Pointer(windowTitle)),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE,
		uintptr(winX), uintptr(winY), uintptr(winW), uintptr(winH),
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return
	}
	a.hwndMain = hwnd

	// Enable DWM Immersive Dark Mode for window title bar (Windows 10 build 17763+ / Windows 11)
	var darkMode int32 = 1
	procDwmSetWindowAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&darkMode)), 4)

	// Create Segoe UI fonts
	a.hFont, _, _ = procCreateFontW.Call(
		15, 0, 0, 0, 400, 0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)
	a.hFontBold, _, _ = procCreateFontW.Call(
		18, 0, 0, 0, 700, 0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)
	a.hFontMeter, _, _ = procCreateFontW.Call(
		22, 0, 0, 0, 700, 0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)
	a.hFontSmall, _, _ = procCreateFontW.Call(
		12, 0, 0, 0, 400, 0, 0, 0,
		1, 0, 0, 0, 0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)

	createCtrl := func(class, text string, style uint32, x, y, w, h int32, id int) uintptr {
		c, _, _ := procCreateWindowExW.Call(
			0,
			uintptr(unsafe.Pointer(utf16Ptr(class))),
			uintptr(unsafe.Pointer(utf16Ptr(text))),
			uintptr(WS_CHILD|WS_VISIBLE|style),
			uintptr(x), uintptr(y), uintptr(w), uintptr(h),
			hwnd, uintptr(id), hInstance, 0,
		)
		if c != 0 && a.hFont != 0 {
			procSendMessageW.Call(c, WM_SETFONT, a.hFont, 1)
		}
		return c
	}

	// 1. Header (Y: 10..48)
	a.hTitle = createCtrl("STATIC", t.HeaderTitle, SS_LEFT, 20, 10, 580, 22, 0)
	if a.hFontBold != 0 {
		procSendMessageW.Call(a.hTitle, WM_SETFONT, a.hFontBold, 1)
	}
	a.hBtnLang = createCtrl("BUTTON", t.BtnLangToggle, BS_OWNERDRAW|WS_TABSTOP, 600, 10, 105, 28, 2006)

	summaryText := a.hw.Summary()
	if a.cfg.Lang == "en" {
		summaryText = a.hw.SummaryEN()
	}
	a.hLblSummary = createCtrl("STATIC", summaryText, SS_LEFT, 20, 32, 690, 18, 0)

	// 2. Server & Model GroupBox (Y: 54, H: 155)
	a.hGrpServer = createCtrl("BUTTON", t.GroupServer, BS_GROUPBOX, 16, 54, 692, 155, 0)
	a.hLblLlama = createCtrl("STATIC", t.LblLlama, SS_LEFT, 32, 76, 510, 16, 0)
	a.hEditLlama = createCtrl("EDIT", a.cfg.LlamaServer, WS_BORDER|ES_LEFT|ES_AUTOHSCROLL|WS_TABSTOP, 32, 94, 545, 24, 100)
	a.hBtnBrowseLlama = createCtrl("BUTTON", t.BtnBrowse, BS_OWNERDRAW|WS_TABSTOP, 590, 93, 102, 26, 1001)

	a.hLblModel = createCtrl("STATIC", t.LblModel, SS_LEFT, 32, 124, 510, 16, 0)
	a.hEditModel = createCtrl("EDIT", a.cfg.Model, WS_BORDER|ES_LEFT|ES_AUTOHSCROLL|WS_TABSTOP, 32, 142, 545, 24, 101)
	a.hBtnBrowseModel = createCtrl("BUTTON", t.BtnBrowse, BS_OWNERDRAW|WS_TABSTOP, 590, 141, 102, 26, 1002)

	a.hBtnAutoDL = createCtrl("BUTTON", t.BtnAutoDL, BS_OWNERDRAW|WS_TABSTOP, 32, 172, 240, 28, 1003)
	a.hLblDLProgress = createCtrl("STATIC", "", SS_LEFT, 280, 178, 410, 20, 0)

	// 3. Execution Settings & Actions GroupBox (Y: 216, H: 130)
	a.hGrpExec = createCtrl("BUTTON", t.GroupExec, BS_GROUPBOX, 16, 216, 692, 130, 0)

	// Row 1: Configs (Y: 236)
	a.hLblContext = createCtrl("STATIC", t.LblContext, SS_LEFT, 32, 239, 78, 18, 0)
	a.hComboCtx = createCtrl("COMBOBOX", "", CBS_DROPDOWNLIST|WS_TABSTOP, 114, 236, 105, 120, 102)
	ctxOptions := []string{"8192 (8K)", "16384 (16K)", "32768 (32K)"}
	for _, opt := range ctxOptions {
		procSendMessageW.Call(a.hComboCtx, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(opt))))
	}
	ctxIdx := uintptr(0)
	if a.cfg.Context == 16384 {
		ctxIdx = 1
	} else if a.cfg.Context == 32768 {
		ctxIdx = 2
	}
	procSendMessageW.Call(a.hComboCtx, CB_SETCURSEL, ctxIdx, 0)

	a.hLblGPU = createCtrl("STATIC", t.LblGPU, SS_LEFT, 230, 239, 36, 18, 0)
	a.hComboNGL = createCtrl("COMBOBOX", "", CBS_DROPDOWNLIST|WS_TABSTOP, 270, 236, 115, 120, 103)
	procSendMessageW.Call(a.hComboNGL, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(t.OptGPUAll))))
	procSendMessageW.Call(a.hComboNGL, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr(t.OptGPUCPU))))
	nglIdx := uintptr(0)
	if a.cfg.GPULayers == 0 {
		nglIdx = 1
	}
	procSendMessageW.Call(a.hComboNGL, CB_SETCURSEL, nglIdx, 0)

	a.hLblJevPort = createCtrl("STATIC", t.LblJevPort, SS_LEFT, 395, 239, 65, 18, 0)
	a.hEditJevPort = createCtrl("EDIT", fmt.Sprintf("%d", a.cfg.JevPort), WS_BORDER|ES_LEFT|WS_TABSTOP, 462, 236, 48, 22, 104)

	a.hLblLlamaPort = createCtrl("STATIC", t.LblLlamaPort, SS_LEFT, 518, 239, 42, 18, 0)
	a.hEditLlamaPort = createCtrl("EDIT", fmt.Sprintf("%d", a.cfg.LlamaPort), WS_BORDER|ES_LEFT|WS_TABSTOP, 562, 236, 48, 22, 105)

	a.hComboHost = createCtrl("COMBOBOX", "", CBS_DROPDOWNLIST|WS_TABSTOP, 616, 236, 82, 120, 106)
	procSendMessageW.Call(a.hComboHost, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr("127.0.0.1"))))
	procSendMessageW.Call(a.hComboHost, CB_ADDSTRING, 0, uintptr(unsafe.Pointer(utf16Ptr("0.0.0.0"))))
	hostIdx := uintptr(0)
	if a.cfg.Host == "0.0.0.0" {
		hostIdx = 1
	}
	procSendMessageW.Call(a.hComboHost, CB_SETCURSEL, hostIdx, 0)

	// Row 2: Action Buttons (Y: 268)
	a.hBtnStart = createCtrl("BUTTON", t.BtnStart, BS_OWNERDRAW|WS_TABSTOP, 32, 268, 110, 32, 2001)
	a.hBtnStop = createCtrl("BUTTON", t.BtnStop, BS_OWNERDRAW|WS_TABSTOP, 150, 268, 85, 32, 2002)
	enableCtrl(a.hBtnStop, false)
	a.hBtnDemo = createCtrl("BUTTON", t.BtnDemo, BS_OWNERDRAW|WS_TABSTOP, 243, 268, 105, 32, 2003)
	enableCtrl(a.hBtnDemo, false)
	a.hBtnCopyURL = createCtrl("BUTTON", t.BtnCopyURL, BS_OWNERDRAW|WS_TABSTOP, 356, 268, 105, 32, 2005)

	a.hLblStatus = createCtrl("STATIC", t.LblStatusStop, SS_LEFT, 470, 274, 230, 22, 2004)

	// Row 3: Quick Tests (Y: 308)
	a.hLblQuickTest = createCtrl("STATIC", t.LblQuickTest, SS_LEFT, 32, 312, 70, 18, 0)
	a.hBtnTestStop = createCtrl("BUTTON", t.BtnTestStop, BS_OWNERDRAW|WS_TABSTOP, 114, 308, 130, 26, 3001)
	enableCtrl(a.hBtnTestStop, false)
	a.hBtnTestExtract = createCtrl("BUTTON", t.BtnTestExtract, BS_OWNERDRAW|WS_TABSTOP, 252, 308, 140, 26, 3002)
	enableCtrl(a.hBtnTestExtract, false)
	a.hBtnTestScan = createCtrl("BUTTON", t.BtnTestScan, BS_OWNERDRAW|WS_TABSTOP, 400, 308, 115, 26, 3003)
	enableCtrl(a.hBtnTestScan, false)

	// 4. Real-Time Monitor GroupBox (Y: 352, H: 425)
	a.hGrpMonitor = createCtrl("BUTTON", t.GroupMonitor, BS_GROUPBOX, 16, 352, 692, 425, 0)

	// 4 Metrics Panels (Y: 374)
	a.hLblLatencyHdr = createCtrl("STATIC", t.LblLatency, SS_LEFT, 32, 374, 145, 16, 0)
	a.hLblLatencyVal = createCtrl("STATIC", "--- ms", SS_LEFT, 32, 390, 145, 26, 0)
	if a.hFontMeter != 0 {
		procSendMessageW.Call(a.hLblLatencyVal, WM_SETFONT, a.hFontMeter, 1)
	}

	a.hLblTTFTHdr = createCtrl("STATIC", t.LblTTFT, SS_LEFT, 195, 374, 145, 16, 0)
	a.hLblTTFTVal = createCtrl("STATIC", "--- ms", SS_LEFT, 195, 390, 145, 26, 0)
	if a.hFontMeter != 0 {
		procSendMessageW.Call(a.hLblTTFTVal, WM_SETFONT, a.hFontMeter, 1)
	}

	a.hLblSpeedHdr = createCtrl("STATIC", t.LblSpeed, SS_LEFT, 360, 374, 145, 16, 0)
	a.hLblSpeedVal = createCtrl("STATIC", "--- tok/s", SS_LEFT, 360, 390, 145, 26, 0)
	if a.hFontMeter != 0 {
		procSendMessageW.Call(a.hLblSpeedVal, WM_SETFONT, a.hFontMeter, 1)
	}

	a.hLblCallsHdr = createCtrl("STATIC", t.LblCalls, SS_LEFT, 530, 374, 160, 16, 0)
	a.hLblCallsVal = createCtrl("STATIC", fmt.Sprintf(t.CallsFormat, 0), SS_LEFT, 530, 390, 160, 26, 0)
	if a.hFontMeter != 0 {
		procSendMessageW.Call(a.hLblCallsVal, WM_SETFONT, a.hFontMeter, 1)
	}

	// Canvas: LEDs + Gauge Bar + Sparkline (Y: 422, W: 660, H: 135)
	a.hCanvas, _, _ = procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(canvasClassName)),
		0,
		uintptr(WS_CHILD|WS_VISIBLE),
		32, 422, 660, 135,
		hwnd, 0, hInstance, 0,
	)

	// Activity Log Header & Buttons (Y: 566)
	a.hLblRecentLogs = createCtrl("STATIC", t.LblRecentLogs, SS_LEFT, 32, 569, 400, 18, 0)
	a.hBtnLogCopy = createCtrl("BUTTON", t.BtnLogCopy, BS_OWNERDRAW|WS_TABSTOP, 490, 565, 105, 26, 3005)
	a.hBtnLogClear = createCtrl("BUTTON", t.BtnLogClear, BS_OWNERDRAW|WS_TABSTOP, 605, 565, 85, 26, 3006)

	// Activity Log ListBox (Y: 595, W: 660, H: 165)
	a.hListLog = createCtrl("LISTBOX", "", WS_BORDER|WS_VSCROLL|LBS_NOTIFY, 32, 595, 660, 165, 3004)

	// Enable Drag & Drop
	procDragAcceptFiles.Call(hwnd, 1)

	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)

	var msg MSG
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func (a *App) autoDownload() {
	enableCtrl(a.hBtnAutoDL, false)
	prepMsg := "ダウンロード準備中…"
	if a.cfg.Lang == "en" {
		prepMsg = "Preparing download..."
	}
	setCtrlText(a.hLblDLProgress, prepMsg)

	go func() {
		base := config.DataDir()
		backend := a.hw.Backend
		if a.cfg.Backend != "" {
			backend = a.cfg.Backend
		}
		llama, model, err := download.FetchAll(base, backend, func(label string, done, total int64) {
			var text string
			if total > 0 {
				pct := float64(done) / float64(total) * 100.0
				mbDone := float64(done) / (1024 * 1024)
				mbTotal := float64(total) / (1024 * 1024)
				text = fmt.Sprintf("%s: %.0f%% (%.1f/%.1f MB)", label, pct, mbDone, mbTotal)
			} else {
				if a.cfg.Lang == "en" {
					text = label + " fetching..."
				} else {
					text = label + " 取得中…"
				}
			}
			uStr := utf16Ptr(text)
			procPostMessageW.Call(a.hwndMain, WM_APP_DL_PROG, uintptr(unsafe.Pointer(uStr)), 0)
		})

		if err != nil {
			errStr := utf16Ptr(err.Error())
			procPostMessageW.Call(a.hwndMain, WM_APP_DL_FAIL, uintptr(unsafe.Pointer(errStr)), 0)
			return
		}

		a.downloadedLlama = llama
		a.downloadedModel = model
		procPostMessageW.Call(a.hwndMain, WM_APP_DL_DONE, 0, 0)
	}()
}

func (a *App) getLaunchOpts() (llamaPath, modelPath string, opts proc.Options, err error) {
	llamaPath = strings.TrimSpace(getCtrlText(a.hEditLlama))
	modelPath = strings.TrimSpace(getCtrlText(a.hEditModel))

	if llamaPath == "" || modelPath == "" {
		if a.cfg.Lang == "en" {
			err = fmt.Errorf("Please specify both llama-server and GGUF model paths.")
		} else {
			err = fmt.Errorf("llama-server と GGUF モデルの両方を指定してください。")
		}
		return
	}
	if _, statErr := os.Stat(llamaPath); statErr != nil {
		if a.cfg.Lang == "en" {
			err = fmt.Errorf("llama-server not found:\n%s", llamaPath)
		} else {
			err = fmt.Errorf("llama-server が見つかりません:\n%s", llamaPath)
		}
		return
	}
	if _, statErr := os.Stat(modelPath); statErr != nil {
		if a.cfg.Lang == "en" {
			err = fmt.Errorf("Model file not found:\n%s", modelPath)
		} else {
			err = fmt.Errorf("モデルファイルが見つかりません:\n%s", modelPath)
		}
		return
	}

	ctx := 8192
	curCtx, _, _ := procSendMessageW.Call(a.hComboCtx, CB_GETCURSEL, 0, 0)
	switch curCtx {
	case 1:
		ctx = 16384
	case 2:
		ctx = 32768
	default:
		ctx = 8192
	}

	ngl := 99
	curNGL, _, _ := procSendMessageW.Call(a.hComboNGL, CB_GETCURSEL, 0, 0)
	if curNGL == 1 {
		ngl = 0
	}

	jevPort, _ := strconv.Atoi(strings.TrimSpace(getCtrlText(a.hEditJevPort)))
	if jevPort <= 0 {
		jevPort = 8090
	}
	llamaPort, _ := strconv.Atoi(strings.TrimSpace(getCtrlText(a.hEditLlamaPort)))
	if llamaPort <= 0 {
		llamaPort = 8080
	}

	host := "127.0.0.1"
	if curHost, _, _ := procSendMessageW.Call(a.hComboHost, CB_GETCURSEL, 0, 0); curHost == 1 {
		host = "0.0.0.0"
	}

	a.cfg.LlamaServer = llamaPath
	a.cfg.Model = modelPath
	a.cfg.Context = ctx
	a.cfg.GPULayers = ngl
	a.cfg.JevPort = jevPort
	a.cfg.LlamaPort = llamaPort
	a.cfg.Host = host
	_ = a.cfg.Save()

	opts = proc.Options{
		Context:   ctx,
		Parallel:  2,
		GPULayers: ngl,
		LlamaPort: llamaPort,
		Host:      host,
	}
	return
}

func (a *App) start() {
	llamaPath, modelPath, opts, err := a.getLaunchOpts()
	if err != nil {
		noticeTitle := "確認"
		if a.cfg.Lang == "en" {
			noticeTitle = "Notice"
		}
		procMessageBoxW.Call(a.hwndMain, uintptr(unsafe.Pointer(utf16Ptr(err.Error()))), uintptr(unsafe.Pointer(utf16Ptr(noticeTitle))), MB_OK|MB_ICONWARNING)
		return
	}

	a.starting = true
	enableCtrl(a.hBtnStart, false)
	t := getI18n(a.cfg.Lang)
	setCtrlText(a.hLblStatus, t.LblStatusStart)

	if err := a.runner.Start(llamaPath, modelPath, opts); err != nil {
		a.starting = false
		enableCtrl(a.hBtnStart, true)
		errTitle := "エラー"
		failMsg := "起動に失敗しました: " + err.Error()
		if a.cfg.Lang == "en" {
			errTitle = "Error"
			failMsg = "Failed to start: " + err.Error()
		}
		procMessageBoxW.Call(a.hwndMain, uintptr(unsafe.Pointer(utf16Ptr(failMsg))), uintptr(unsafe.Pointer(utf16Ptr(errTitle))), MB_OK|MB_ICONERROR)
		return
	}

	go func() {
		deadline := time.Now().Add(90 * time.Second)
		ok := false
		checkURL := fmt.Sprintf("http://127.0.0.1:%d/v1/models", opts.LlamaPort)
		for time.Now().Before(deadline) {
			resp, err := http.Get(checkURL)
			if err == nil && resp.StatusCode == 200 {
				_ = resp.Body.Close()
				ok = true
				break
			}
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			time.Sleep(500 * time.Millisecond)
		}

		if !ok {
			a.runner.Stop()
			procPostMessageW.Call(a.hwndMain, WM_APP_FAILED, 0, 0)
			return
		}

		llamaRoot := fmt.Sprintf("http://127.0.0.1:%d", opts.LlamaPort)
		a.api = api.NewServer(llamaRoot)
		a.api.OnEvent = func(ev api.RequestEvent) {
			a.eventMu.Lock()
			a.pendingEvents = append(a.pendingEvents, ev)
			a.eventMu.Unlock()
			procPostMessageW.Call(a.hwndMain, WM_APP_EVENT, 0, 0)
		}

		bindAddr := fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.JevPort)
		go func() {
			_ = a.api.Start(bindAddr)
		}()
		time.Sleep(300 * time.Millisecond)

		procPostMessageW.Call(a.hwndMain, WM_APP_READY, 0, 0)
	}()
}

func (a *App) stop() {
	a.cleanup()
	t := getI18n(a.cfg.Lang)
	setCtrlText(a.hLblStatus, t.LblStatusStop)
	enableCtrl(a.hBtnStart, true)
	enableCtrl(a.hBtnStop, false)
	enableCtrl(a.hBtnDemo, false)
	enableCtrl(a.hBtnTestStop, false)
	enableCtrl(a.hBtnTestExtract, false)
	enableCtrl(a.hBtnTestScan, false)
}

func (a *App) cleanup() {
	if a.api != nil {
		_ = a.api.Stop()
		a.api = nil
	}
	a.apiRunning = false
	a.starting = false
	a.runner.Stop()
}

func (a *App) openDemo() {
	port := a.cfg.JevPort
	if port <= 0 {
		port = 8090
	}
	demoURL := utf16Ptr(fmt.Sprintf("http://127.0.0.1:%d/demo", port))
	openVerb := utf16Ptr("open")
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(openVerb)), uintptr(unsafe.Pointer(demoURL)), 0, 0, SW_SHOWNORMAL)
}
