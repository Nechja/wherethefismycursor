//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
	dwmapi   = syscall.NewLazyDLL("dwmapi.dll")
	msimg32  = syscall.NewLazyDLL("msimg32.dll")

	pRegisterClassExW              = user32.NewProc("RegisterClassExW")
	pCreateWindowExW               = user32.NewProc("CreateWindowExW")
	pDefWindowProcW                = user32.NewProc("DefWindowProcW")
	pGetCursorPos                  = user32.NewProc("GetCursorPos")
	pGetMessageW                   = user32.NewProc("GetMessageW")
	pTranslateMessage              = user32.NewProc("TranslateMessage")
	pDispatchMessageW              = user32.NewProc("DispatchMessageW")
	pUpdateLayeredWindow           = user32.NewProc("UpdateLayeredWindow")
	pShowWindow                    = user32.NewProc("ShowWindow")
	pSetWindowPos                  = user32.NewProc("SetWindowPos")
	pRegisterHotKey                = user32.NewProc("RegisterHotKey")
	pPostQuitMessage               = user32.NewProc("PostQuitMessage")
	pPostMessageW                  = user32.NewProc("PostMessageW")
	pSendMessageW                  = user32.NewProc("SendMessageW")
	pSetForegroundWindow           = user32.NewProc("SetForegroundWindow")
	pCreatePopupMenu               = user32.NewProc("CreatePopupMenu")
	pAppendMenuW                   = user32.NewProc("AppendMenuW")
	pTrackPopupMenu                = user32.NewProc("TrackPopupMenu")
	pDestroyMenu                   = user32.NewProc("DestroyMenu")
	pInvalidateRect                = user32.NewProc("InvalidateRect")
	pCreateIconIndirect            = user32.NewProc("CreateIconIndirect")
	pMessageBoxW                   = user32.NewProc("MessageBoxW")
	pSetTimer                      = user32.NewProc("SetTimer")
	pAdjustWindowRectEx            = user32.NewProc("AdjustWindowRectEx")
	pGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	pGetDpiForSystem               = user32.NewProc("GetDpiForSystem")
	pGetDC                         = user32.NewProc("GetDC")
	pSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")
	pSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	pBeginPaint                    = user32.NewProc("BeginPaint")
	pEndPaint                      = user32.NewProc("EndPaint")
	pDrawTextW                     = user32.NewProc("DrawTextW")
	pFillRect                      = user32.NewProc("FillRect")
	pSetCapture                    = user32.NewProc("SetCapture")
	pReleaseCapture                = user32.NewProc("ReleaseCapture")
	pTrackMouseEvent               = user32.NewProc("TrackMouseEvent")
	pLoadCursorW                   = user32.NewProc("LoadCursorW")
	pSetCursor                     = user32.NewProc("SetCursor")

	pCreateCompatibleDC     = gdi32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBitmap = gdi32.NewProc("CreateCompatibleBitmap")
	pCreateDIBSection       = gdi32.NewProc("CreateDIBSection")
	pSelectObject           = gdi32.NewProc("SelectObject")
	pCreateSolidBrush       = gdi32.NewProc("CreateSolidBrush")
	pCreateBitmap           = gdi32.NewProc("CreateBitmap")
	pDeleteObject           = gdi32.NewProc("DeleteObject")
	pCreateFontW            = gdi32.NewProc("CreateFontW")
	pCreatePen              = gdi32.NewProc("CreatePen")
	pRoundRect              = gdi32.NewProc("RoundRect")
	pEllipse                = gdi32.NewProc("Ellipse")
	pBitBlt                 = gdi32.NewProc("BitBlt")
	pSetTextColor           = gdi32.NewProc("SetTextColor")
	pSetBkMode              = gdi32.NewProc("SetBkMode")
	pGetStockObject         = gdi32.NewProc("GetStockObject")
	pTextOutW               = gdi32.NewProc("TextOutW")
	pGetTextExtentPoint32W  = gdi32.NewProc("GetTextExtentPoint32W")
	pArc                    = gdi32.NewProc("Arc")
	pSetStretchBltMode      = gdi32.NewProc("SetStretchBltMode")
	pSetBrushOrgEx          = gdi32.NewProc("SetBrushOrgEx")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW     = kernel32.NewProc("CreateMutexW")
	pCreateActCtxW    = kernel32.NewProc("CreateActCtxW")
	pActivateActCtx   = kernel32.NewProc("ActivateActCtx")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")

	pChooseColorW = comdlg32.NewProc("ChooseColorW")

	pRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	pRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	pRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	pRegCloseKey      = advapi32.NewProc("RegCloseKey")

	pDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")

	pAlphaBlend = msimg32.NewProc("AlphaBlend")
)

const (
	wsOverlapped  = 0x00000000
	wsPopup       = 0x80000000
	wsCaption     = 0x00C00000
	wsSysMenu     = 0x00080000
	wsMinimizeBox = 0x00020000

	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExToolwindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	wsExAppWindow   = 0x00040000

	swHide           = 0
	swShow           = 5
	swShowNoActivate = 4

	wmNull          = 0x0000
	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmEraseBkgnd    = 0x0014
	wmSetCursor     = 0x0020
	wmSetIcon       = 0x0080
	wmTimer         = 0x0113
	wmMouseMove     = 0x0200
	wmLButtonDown   = 0x0201
	wmLButtonUp     = 0x0202
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmMouseLeave    = 0x02A3
	wmHotkey        = 0x0312
	wmApp           = 0x8000

	modAlt      = 0x0001
	modControl  = 0x0002
	modNoRepeat = 0x4000

	acSrcOver  = 0
	acSrcAlpha = 1
	ulwAlpha   = 2

	hwndTopmost = ^uintptr(0)

	swpNoSize     = 0x0001
	swpNoMove     = 0x0002
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010

	iccStandardClasses = 0x4000

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 0x01
	nifIcon    = 0x02
	nifTip     = 0x04

	mfString    = 0x0000
	mfSeparator = 0x0800
	mfChecked   = 0x0008

	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100

	ccRGBInit  = 0x0001
	ccFullOpen = 0x0002

	hkeyCurrentUser = 0x80000001
	keyQueryValue   = 0x0001
	keySetValue     = 0x0002
	regSz           = 1

	mbIconError       = 0x10
	mbIconInformation = 0x40

	smCxScreen = 0
	smCyScreen = 1

	iconSmall = 0
	iconBig   = 1

	defaultCharset   = 1
	cleartypeQuality = 5

	dtCenter     = 0x0001
	dtRight      = 0x0002
	dtVCenter    = 0x0004
	dtSingleLine = 0x0020

	bkTransparent = 1
	penSolid      = 0
	nullPen       = 8
	srcCopy       = 0x00CC0020
	halftone      = 4

	blendPremult = uintptr(255)<<16 | uintptr(acSrcAlpha)<<24

	tmeLeave = 0x0002

	idcArrow = 32512
	idcHand  = 32649

	dwmDarkModeAttr = 20

	invalidHandle      = ^uintptr(0)
	dpiCtxPerMonitorV2 = ^uintptr(3)
	fallbackDPI        = 96
)

type point struct{ x, y int32 }
type sizeT struct{ cx, cy int32 }
type rect struct{ left, top, right, bottom int32 }
type blendFunc struct{ op, flags, alpha, format byte }

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type winMsg struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

type guid struct {
	a uint32
	b uint16
	c uint16
	d [8]byte
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         guid
	hBalloonIcon     uintptr
}

type chooseColorT struct {
	lStructSize    uint32
	hwndOwner      uintptr
	hInstance      uintptr
	rgbResult      uint32
	lpCustColors   uintptr
	flags          uint32
	lCustData      uintptr
	lpfnHook       uintptr
	lpTemplateName uintptr
}

type initCommonControlsExT struct {
	dwSize uint32
	dwICC  uint32
}

type actCtx struct {
	cbSize                 uint32
	dwFlags                uint32
	lpSource               *uint16
	wProcessorArchitecture uint16
	wLangId                uint16
	lpAssemblyDirectory    *uint16
	lpResourceName         *uint16
	lpApplicationName      *uint16
	hModule                uintptr
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

type trackMouseEventT struct {
	cbSize      uint32
	dwFlags     uint32
	hwndTrack   uintptr
	dwHoverTime uint32
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func strPtr(s string) unsafe.Pointer {
	return unsafe.Pointer(utf16Ptr(s))
}

func colorref(c rgb) uintptr {
	return uintptr(c.r) | uintptr(c.g)<<8 | uintptr(c.b)<<16
}

func fromColorref(v uint32) rgb {
	return rgb{uint8(v), uint8(v >> 8), uint8(v >> 16)}
}

func mouseXY(lParam uintptr) (int32, int32) {
	return int32(int16(lParam & 0xffff)), int32(int16((lParam >> 16) & 0xffff))
}

func setDPIAware() {
	if pSetProcessDpiAwarenessContext.Find() == nil {
		if r, _, _ := pSetProcessDpiAwarenessContext.Call(dpiCtxPerMonitorV2); r != 0 {
			return
		}
	}
	pSetProcessDPIAware.Call()
}

func systemDPI() int {
	if pGetDpiForSystem.Find() == nil {
		if d, _, _ := pGetDpiForSystem.Call(); d != 0 {
			return int(d)
		}
	}
	return fallbackDPI
}

func enableVisualStyles(manifestPath string) {
	ctx := actCtx{
		cbSize:   uint32(unsafe.Sizeof(actCtx{})),
		lpSource: utf16Ptr(manifestPath),
	}
	h, _, _ := pCreateActCtxW.Call(uintptr(unsafe.Pointer(&ctx)))
	if h != invalidHandle {
		var cookie uintptr
		pActivateActCtx.Call(h, uintptr(unsafe.Pointer(&cookie)))
	}
	icc := initCommonControlsExT{
		dwSize: uint32(unsafe.Sizeof(initCommonControlsExT{})),
		dwICC:  iccStandardClasses,
	}
	pInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
}

const comctlManifest = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity version="1.0.0.0" processorArchitecture="*" name="wherethefismycursor" type="win32"/>
  <dependency><dependentAssembly>
    <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
  </dependentAssembly></dependency>
</assembly>
`
