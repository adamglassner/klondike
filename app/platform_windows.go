//go:build windows

// The Windows side: the game window, input, display scaling, full screen and
// sound output (waveOut). Pure Win32 through syscall - no cgo.
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"math"
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	winmm    = syscall.NewLazyDLL("winmm.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")

	pRegisterClassExW          = user32.NewProc("RegisterClassExW")
	pCreateWindowExW           = user32.NewProc("CreateWindowExW")
	pDefWindowProcW            = user32.NewProc("DefWindowProcW")
	pDestroyWindow             = user32.NewProc("DestroyWindow")
	pShowWindow                = user32.NewProc("ShowWindow")
	pPeekMessageW              = user32.NewProc("PeekMessageW")
	pTranslateMessage          = user32.NewProc("TranslateMessage")
	pDispatchMessageW          = user32.NewProc("DispatchMessageW")
	pPostQuitMessage           = user32.NewProc("PostQuitMessage")
	pMsgWaitForMultipleObjEx   = user32.NewProc("MsgWaitForMultipleObjectsEx")
	pBeginPaint                = user32.NewProc("BeginPaint")
	pEndPaint                  = user32.NewProc("EndPaint")
	pGetDC                     = user32.NewProc("GetDC")
	pReleaseDC                 = user32.NewProc("ReleaseDC")
	pLoadCursorW               = user32.NewProc("LoadCursorW")
	pSetCursor                 = user32.NewProc("SetCursor")
	pLoadImageW                = user32.NewProc("LoadImageW")
	pSetCapture                = user32.NewProc("SetCapture")
	pReleaseCapture            = user32.NewProc("ReleaseCapture")
	pGetKeyState               = user32.NewProc("GetKeyState")
	pGetClientRect             = user32.NewProc("GetClientRect")
	pSetWindowPos              = user32.NewProc("SetWindowPos")
	pGetWindowLongPtrW         = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtrW         = user32.NewProc("SetWindowLongPtrW")
	pGetWindowPlacement        = user32.NewProc("GetWindowPlacement")
	pSetWindowPlacement        = user32.NewProc("SetWindowPlacement")
	pMonitorFromWindow         = user32.NewProc("MonitorFromWindow")
	pGetMonitorInfoW           = user32.NewProc("GetMonitorInfoW")
	pIsZoomed                  = user32.NewProc("IsZoomed")
	pIsIconic                  = user32.NewProc("IsIconic")
	pFindWindowW               = user32.NewProc("FindWindowW")
	pSetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	pMessageBoxW               = user32.NewProc("MessageBoxW")
	pGetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem           = user32.NewProc("GetDpiForSystem")
	pAdjustWindowRectExForDpi  = user32.NewProc("AdjustWindowRectExForDpi")
	pSetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	pSystemParametersInfoW     = user32.NewProc("SystemParametersInfoW")
	pSetTimer                  = user32.NewProc("SetTimer")
	pKillTimer                 = user32.NewProc("KillTimer")

	pGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW        = kernel32.NewProc("CreateMutexW")
	pCreateEventW        = kernel32.NewProc("CreateEventW")
	pWaitForSingleObject = kernel32.NewProc("WaitForSingleObject")
	pVirtualAlloc        = kernel32.NewProc("VirtualAlloc")

	pTimeBeginPeriod   = winmm.NewProc("timeBeginPeriod")
	pWaveOutOpen       = winmm.NewProc("waveOutOpen")
	pWaveOutPrepHeader = winmm.NewProc("waveOutPrepareHeader")
	pWaveOutWrite      = winmm.NewProc("waveOutWrite")

	pSetDIBitsToDevice = gdi32.NewProc("SetDIBitsToDevice")
	pDwmFlush          = dwmapi.NewProc("DwmFlush")
)

const (
	className = "IroniteKlondike"

	wmDestroy        = 0x0002
	wmSize           = 0x0005
	wmPaint          = 0x000F
	wmClose          = 0x0010
	wmEraseBkgnd     = 0x0014
	wmSetCursor      = 0x0020
	wmGetMinMaxInfo  = 0x0024
	wmKeyDown        = 0x0100
	wmSysKeyDown     = 0x0104
	wmTimer          = 0x0113
	wmMouseMove      = 0x0200
	wmLButtonDown    = 0x0201
	wmLButtonUp      = 0x0202
	wmMouseWheel     = 0x020A
	wmEnterSizeMove  = 0x0231
	wmExitSizeMove   = 0x0232
	wmCaptureChanged = 0x0215
	wmDpiChanged     = 0x02E0
	wmQuit           = 0x0012

	wsOverlappedWindow = 0x00CF0000
	wsPopup            = 0x80000000
	wsVisible          = 0x10000000
	gwlStyle           = -16
)

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      [2]int32
	Private uint32
}

type rect struct{ L, T, R, B int32 }

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	Rc        rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type windowPlacement struct {
	Length    uint32
	Flags     uint32
	ShowCmd   uint32
	MinPos    [2]int32
	MaxPos    [2]int32
	NormalPos rect
}

type monitorInfo struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
}

func fatal(text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(productName)
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x10)
	os.Exit(1)
}

// osPtr turns an address Windows gave us (a message parameter, memory from
// VirtualAlloc) into a pointer. That memory isn't managed by Go.
func osPtr(u uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&u)) }

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

/* ---------------------------------------------------------------- window */

type win struct {
	hwnd       uintptr
	app        *App
	scale      float64
	arrow      uintptr
	hand       uintptr
	wantHand   bool
	last       time.Time
	fullscreen bool
	wasZoomed  bool
	prevPlace  windowPlacement
	normalW    int // last restored client size, in 96-DPI pixels
	normalH    int
	sizing     bool
	bmi        bitmapInfoHeader
}

var W *win

func run() {
	runtime.LockOSThread()
	pSetProcessDpiAwarenessCtx.Call(^uintptr(3)) // PER_MONITOR_AWARE_V2 (-4); the manifest says so too
	pTimeBeginPeriod.Call(1)

	// One Klondike at a time: bring the running one to the front instead.
	pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(u16(`Local\IroniteKlondike`))))
	if e := syscall.GetLastError(); e == syscall.Errno(183) { // ERROR_ALREADY_EXISTS
		if h, _, _ := pFindWindowW.Call(uintptr(unsafe.Pointer(u16(className))), 0); h != 0 {
			pShowWindow.Call(h, 9) // SW_RESTORE
			pSetForegroundWindow.Call(h)
		}
		return
	}

	inst, _, _ := pGetModuleHandleW.Call(0)
	w := &win{}
	W = w
	w.arrow, _, _ = pLoadCursorW.Call(0, 32512)
	w.hand, _, _ = pLoadCursorW.Call(0, 32649)
	icon, _, _ := pLoadImageW.Call(inst, uintptr(unsafe.Pointer(u16("APPICON"))), 1, 0, 0, 0x8040)
	iconSm, _, _ := pLoadImageW.Call(inst, uintptr(unsafe.Pointer(u16("APPICON"))), 1, 16, 16, 0x8000)
	wc := wndClassEx{
		Style:   0x3, // CS_HREDRAW | CS_VREDRAW
		WndProc: syscall.NewCallback(wndProc), Instance: inst, Icon: icon, IconSm: iconSm,
		Cursor: w.arrow, ClassName: u16(className),
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		fatal("Klondike couldn't open its window:\n" + err.Error())
	}

	sv := loadSave()
	dpi := uint32(96)
	if pGetDpiForSystem.Find() == nil {
		d, _, _ := pGetDpiForSystem.Call()
		dpi = uint32(d)
	}
	s := float64(dpi) / 96
	cw, ch := 1440, 920
	if sv.WinW >= 640 && sv.WinH >= 480 {
		cw, ch = sv.WinW, sv.WinH
	}
	var work rect
	pSystemParametersInfoW.Call(0x30, 0, uintptr(unsafe.Pointer(&work)), 0) // SPI_GETWORKAREA
	ww, wh := float64(work.R-work.L), float64(work.B-work.T)
	pw := int(math.Min(float64(cw)*s, ww*.94))
	ph := int(math.Min(float64(ch)*s, wh*.92))
	r := rect{0, 0, int32(pw), int32(ph)}
	if pAdjustWindowRectExForDpi.Find() == nil {
		pAdjustWindowRectExForDpi.Call(uintptr(unsafe.Pointer(&r)), wsOverlappedWindow, 0, 0, uintptr(dpi))
	}
	fw, fh := r.R-r.L, r.B-r.T
	x := work.L + (int32(ww)-fw)/2
	y := work.T + (int32(wh)-fh)/2
	w.normalW, w.normalH = cw, ch
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(u16(className))),
		uintptr(unsafe.Pointer(u16("Klondike - Gold Rush Solitaire"))), wsOverlappedWindow,
		uintptr(x), uintptr(y), uintptr(fw), uintptr(fh), 0, 0, inst, 0)
	if hwnd == 0 {
		fatal("Klondike couldn't open its window:\n" + err.Error())
	}
	w.hwnd = hwnd
	w.scale = w.dpiScale()
	var cr rect
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
	a := newApp(int(cr.R), int(cr.B), w.scale)
	a.SetCursor = func(h bool) {
		w.wantHand = h
		if h {
			pSetCursor.Call(w.hand)
		} else {
			pSetCursor.Call(w.arrow)
		}
	}
	a.ToggleFullscreen = w.toggleFullscreen
	w.app = a
	startAudio()
	show := uintptr(1) // SW_SHOWNORMAL
	if sv.Maximized {
		show = 3 // SW_MAXIMIZE
	}
	pShowWindow.Call(hwnd, show)
	w.last = time.Now()
	w.loop()
}

func (w *win) dpiScale() float64 {
	if pGetDpiForWindow.Find() == nil {
		if d, _, _ := pGetDpiForWindow.Call(w.hwnd); d > 0 {
			return float64(d) / 96
		}
	}
	return 1
}

// loop runs until the window closes. While anything moves it draws in step
// with the display (DwmFlush); otherwise it sleeps until there's input.
func (w *win) loop() {
	var m msg
	for {
		for {
			r, _, _ := pPeekMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0, 1) // PM_REMOVE
			if r == 0 {
				break
			}
			if m.Message == wmQuit {
				return
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
		}
		if w.tick() {
			if pDwmFlush.Find() != nil || func() bool { r, _, _ := pDwmFlush.Call(); return r != 0 }() {
				pMsgWaitForMultipleObjEx.Call(0, 0, 15, 0x4FF, 0x4) // no compositor: ~60 fps
			}
			continue
		}
		pMsgWaitForMultipleObjEx.Call(0, 0, 100, 0x4FF, 0x4) // QS_ALLINPUT, MWMO_INPUTAVAILABLE
	}
}

// tick advances the game and draws a frame if anything changed.
func (w *win) tick() bool {
	now := time.Now()
	dt := now.Sub(w.last).Seconds()
	w.last = now
	if r, _, _ := pIsIconic.Call(w.hwnd); r != 0 {
		return false
	}
	if !w.app.update(dt) {
		return false
	}
	w.present()
	return true
}

func (w *win) present() {
	a := w.app
	a.render()
	dc, _, _ := pGetDC.Call(w.hwnd)
	w.blit(dc)
	pReleaseDC.Call(w.hwnd, dc)
}

func (w *win) blit(dc uintptr) {
	cv := w.app.canvas
	cw, ch := cv.Rect.Dx(), cv.Rect.Dy()
	w.bmi = bitmapInfoHeader{Size: 40, Width: int32(cw), Height: -int32(ch), Planes: 1, BitCount: 32}
	pSetDIBitsToDevice.Call(dc, 0, 0, uintptr(cw), uintptr(ch), 0, 0, 0, uintptr(ch),
		uintptr(unsafe.Pointer(&cv.Pix[0])), uintptr(unsafe.Pointer(&w.bmi)), 0)
}

func loword(l uintptr) int { return int(int16(l & 0xFFFF)) }
func hiword(l uintptr) int { return int(int16((l >> 16) & 0xFFFF)) }

func wndProc(hwnd, m, wp, lp uintptr) uintptr {
	w := W
	if w == nil || w.app == nil || hwnd != w.hwnd {
		r, _, _ := pDefWindowProcW.Call(hwnd, m, wp, lp)
		return r
	}
	a := w.app
	switch uint32(m) {
	case wmPaint:
		var ps paintStruct
		dc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		a.render()
		w.blit(dc)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		return 1
	case wmSize:
		cw, ch := int(lp&0xFFFF), int((lp>>16)&0xFFFF)
		if wp == 1 || cw == 0 || ch == 0 { // minimized
			return 0
		}
		if wp == 0 && !w.fullscreen { // restored: remember the size
			w.normalW, w.normalH = int(math.Round(float64(cw)/w.scale)), int(math.Round(float64(ch)/w.scale))
		}
		a.resize(cw, ch, w.scale)
		a.update(0)
		w.present()
		return 0
	case wmDpiChanged:
		w.scale = float64(wp&0xFFFF) / 96
		r := (*rect)(osPtr(lp))
		pSetWindowPos.Call(hwnd, 0, uintptr(r.L), uintptr(r.T), uintptr(r.R-r.L), uintptr(r.B-r.T), 0x14) // NOZORDER|NOACTIVATE
		var cr rect
		pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&cr)))
		a.resize(int(cr.R), int(cr.B), w.scale)
		w.present()
		return 0
	case wmGetMinMaxInfo:
		mm := (*[10]int32)(osPtr(lp))
		mm[6], mm[7] = int32(660*w.scale), int32(500*w.scale) // ptMinTrackSize
		return 0
	case wmEnterSizeMove: // keep animating while the window is dragged
		w.sizing = true
		pSetTimer.Call(hwnd, 1, 15, 0)
		return 0
	case wmExitSizeMove:
		w.sizing = false
		pKillTimer.Call(hwnd, 1)
		return 0
	case wmTimer:
		w.tick()
		return 0
	case wmSetCursor:
		if lp&0xFFFF == 1 { // HTCLIENT
			if w.wantHand {
				pSetCursor.Call(w.hand)
			} else {
				pSetCursor.Call(w.arrow)
			}
			return 1
		}
	case wmMouseMove:
		a.pointerMove(float64(loword(lp)), float64(hiword(lp)))
		return 0
	case wmLButtonDown:
		pSetCapture.Call(hwnd)
		a.pointerDown(float64(loword(lp)), float64(hiword(lp)))
		return 0
	case wmLButtonUp:
		pReleaseCapture.Call()
		a.pointerUp(float64(loword(lp)), float64(hiword(lp)))
		return 0
	case wmCaptureChanged:
		if a.drag != nil || a.slider { // lost the mouse mid-drag: drop it
			a.pointerUp(-1e6, -1e6)
		}
		return 0
	case wmMouseWheel:
		a.wheel(float64(int16(wp>>16)) / 120)
		return 0
	case wmKeyDown, wmSysKeyDown:
		switch wp {
		case 0x71: // F2
			a.key(keyF2)
		case 0x7A: // F11
			a.key(keyF11)
		case 0x1B: // Esc
			if w.fullscreen && !a.ddOpen && a.panel != panelAbout {
				w.toggleFullscreen()
			} else {
				a.key(keyEscape)
			}
		case 'Z':
			if r, _, _ := pGetKeyState.Call(0x11); int16(r) < 0 {
				a.key(keyUndo)
			}
		case 0x0D: // Alt+Enter: full screen
			if m == wmSysKeyDown {
				w.toggleFullscreen()
				return 0
			}
		}
		if m == wmSysKeyDown {
			break
		}
		return 0
	case wmClose:
		zoomed, _, _ := pIsZoomed.Call(hwnd)
		a.saveWindow(w.normalW, w.normalH, zoomed != 0 || (w.fullscreen && w.wasZoomed))
		pDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, m, wp, lp)
	return r
}

func (w *win) toggleFullscreen() {
	h := w.hwnd
	if !w.fullscreen {
		z, _, _ := pIsZoomed.Call(h)
		w.wasZoomed = z != 0
		w.prevPlace = windowPlacement{}
		w.prevPlace.Length = uint32(unsafe.Sizeof(w.prevPlace))
		pGetWindowPlacement.Call(h, uintptr(unsafe.Pointer(&w.prevPlace)))
		mon, _, _ := pMonitorFromWindow.Call(h, 2) // MONITOR_DEFAULTTONEAREST
		mi := monitorInfo{}
		mi.Size = uint32(unsafe.Sizeof(mi))
		pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi)))
		w.fullscreen = true
		pSetWindowLongPtrW.Call(h, uintptr(gwlStyleU()), wsPopup|wsVisible)
		r := mi.Monitor
		pSetWindowPos.Call(h, 0, uintptr(r.L), uintptr(r.T), uintptr(r.R-r.L), uintptr(r.B-r.T), 0x0020|0x0040) // FRAMECHANGED|SHOWWINDOW
		return
	}
	w.fullscreen = false
	pSetWindowLongPtrW.Call(h, uintptr(gwlStyleU()), wsOverlappedWindow|wsVisible)
	pSetWindowPlacement.Call(h, uintptr(unsafe.Pointer(&w.prevPlace)))
	pSetWindowPos.Call(h, 0, 0, 0, 0, 0, 0x0020|0x0001|0x0002|0x0004|0x0040) // FRAMECHANGED|NOSIZE|NOMOVE|NOZORDER|SHOWWINDOW
}

func gwlStyleU() int { return gwlStyle }

/* ------------------------------------------------------------------ sound */

type waveFormatEx struct {
	FormatTag      uint16
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlign     uint16
	BitsPerSample  uint16
	CbSize         uint16
}

type waveHdr struct {
	Data          uintptr
	BufferLength  uint32
	BytesRecorded uint32
	User          uintptr
	Flags         uint32
	Loops         uint32
	Next          uintptr
	Reserved      uintptr
}

const (
	audioBufs   = 4
	audioFrames = 768 // 24 ms per buffer
)

// startAudio opens the default output and keeps it fed from the mixer.
// No sound device is fine: the game just plays silently.
func startAudio() {
	go func() {
		runtime.LockOSThread()
		ev, _, _ := pCreateEventW.Call(0, 0, 0, 0)
		f := waveFormatEx{FormatTag: 1, Channels: 2, SamplesPerSec: sampleRate, AvgBytesPerSec: sampleRate * 4, BlockAlign: 4, BitsPerSample: 16}
		var h uintptr
		if r, _, _ := pWaveOutOpen.Call(uintptr(unsafe.Pointer(&h)), 0xFFFFFFFF, uintptr(unsafe.Pointer(&f)), ev, 0, 0x50000); r != 0 {
			return
		}
		size := uintptr(audioFrames * 4)
		hsz := unsafe.Sizeof(waveHdr{})
		mem, _, _ := pVirtualAlloc.Call(0, audioBufs*(size+hsz), 0x3000, 0x04) // MEM_COMMIT|MEM_RESERVE, PAGE_READWRITE
		if mem == 0 {
			return
		}
		tmp := make([]int16, audioFrames*2)
		hdrs := make([]*waveHdr, audioBufs)
		data := make([][]int16, audioBufs)
		for i := 0; i < audioBufs; i++ {
			hp := mem + uintptr(i)*hsz
			hdrs[i] = (*waveHdr)(osPtr(hp))
			dp := mem + audioBufs*hsz + uintptr(i)*size
			data[i] = unsafe.Slice((*int16)(osPtr(dp)), audioFrames*2)
			hdrs[i].Data, hdrs[i].BufferLength = dp, uint32(size)
			pWaveOutPrepHeader.Call(h, hp, hsz)
			mix.fill(tmp)
			copy(data[i], tmp)
			pWaveOutWrite.Call(h, hp, hsz)
		}
		for {
			pWaitForSingleObject.Call(ev, 100)
			for i := 0; i < audioBufs; i++ {
				if hdrs[i].Flags&1 != 0 { // WHDR_DONE
					mix.fill(tmp)
					copy(data[i], tmp)
					hdrs[i].Flags &^= 1
					pWaveOutWrite.Call(h, uintptr(unsafe.Pointer(hdrs[i])), hsz)
				}
			}
		}
	}()
}
