//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	idcChkHalo = iota + 101
	idcChkShake
	idcBtnHaloColor
	idcBtnRingColor
	idcChkAutostart
	idcBtnTest
	idcChkCycle
	idcChkDual
)

const (
	uiFontName = "Segoe UI"
	uiFontPt   = 9

	settingsClientW = 350
	settingsClientH = 312

	lblChange      = "Change…"
	lblHaloSizeFmt = "Halo size: %d px"
	lblSensFmt     = "Shake sensitivity: %d"
	lblHaloCheck   = "Glowing halo follows the cursor"
	lblShakeCheck  = "Shake the mouse to find it (big rings)"
	lblColor       = "Color"
	lblCycle       = "Cycle colors"
	lblDual        = "Dual hue"
	lblTest        = "Test"
	lblAutostart   = "Start with Windows"
	lblFooter      = "Changes apply instantly. Closing this window keeps the app running in the tray."
)

var (
	settingsHwnd uintptr
	uiFont       uintptr

	cbHalo, cbShake, cbAuto, cbCycle, cbDual uintptr
	swHalo, swRing                           uintptr
	brHalo, brRing                           uintptr
	tbHalo, tbSens                           uintptr
	lblHaloSize, lblSens                     uintptr

	customColors [16]uint32
	settingsDPI  = fallbackDPI
)

func sc(v int) int { return v * settingsDPI / fallbackDPI }

func mkCtl(parent uintptr, class, text string, style uintptr, x, y, w, h, id int) uintptr {
	hwnd, _, _ := pCreateWindowExW.Call(0,
		uintptr(strPtr(class)),
		uintptr(strPtr(text)),
		wsChild|wsVisible|style,
		uintptr(sc(x)), uintptr(sc(y)), uintptr(sc(w)), uintptr(sc(h)),
		parent, uintptr(id), hInst, 0)
	if uiFont != 0 {
		pSendMessageW.Call(hwnd, wmSetFont, uiFont, 1)
	}
	return hwnd
}

func setChecked(hwnd uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = bstChecked
	}
	pSendMessageW.Call(hwnd, bmSetCheck, v, 0)
}

func isChecked(hwnd uintptr) bool {
	r, _, _ := pSendMessageW.Call(hwnd, bmGetCheck, 0, 0)
	return r == bstChecked
}

func setText(hwnd uintptr, s string) {
	pSetWindowTextW.Call(hwnd, uintptr(strPtr(s)))
}

func trackbarPos(hwnd uintptr) int {
	r, _, _ := pSendMessageW.Call(hwnd, tbmGetPos, 0, 0)
	return int(r)
}

func updateSwatchBrushes() {
	if brHalo != 0 {
		pDeleteObject.Call(brHalo)
	}
	if brRing != 0 {
		pDeleteObject.Call(brRing)
	}
	brHalo, _, _ = pCreateSolidBrush.Call(colorref(haloRGB))
	brRing, _, _ = pCreateSolidBrush.Call(colorref(ringRGB))
}

func changeColor(target *string, swatch uintptr) {
	cur, _ := parseColor(*target)
	c, ok := pickColor(cur)
	if !ok {
		return
	}
	*target = hexColor(c)
	applyColors()
	updateSwatchBrushes()
	pInvalidateRect.Call(swatch, 0, 1)
	saveConfig()
}

func settingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		switch loWord(wParam) {
		case idcChkHalo:
			cfg.HaloEnabled = isChecked(cbHalo)
			saveConfig()
		case idcChkShake:
			cfg.ShakeEnabled = isChecked(cbShake)
			saveConfig()
		case idcChkCycle:
			cfg.HaloCycle = isChecked(cbCycle)
			saveConfig()
		case idcChkDual:
			cfg.HaloDual = isChecked(cbDual)
			saveConfig()
		case idcChkAutostart:
			setAutostart(isChecked(cbAuto))
		case idcBtnHaloColor:
			changeColor(&cfg.HaloColor, swHalo)
		case idcBtnRingColor:
			changeColor(&cfg.RingColor, swRing)
		case idcBtnTest:
			triggerRings()
		}
		return 0
	case wmHScroll:
		switch lParam {
		case tbHalo:
			cfg.HaloSize = clampInt(trackbarPos(tbHalo), haloSizeMin, haloSizeMax)
			setText(lblHaloSize, fmt.Sprintf(lblHaloSizeFmt, cfg.HaloSize))
			saveConfig()
		case tbSens:
			cfg.Sensitivity = clampInt(trackbarPos(tbSens), sensitivityMin, sensitivityMax)
			setText(lblSens, fmt.Sprintf(lblSensFmt, cfg.Sensitivity))
			saveConfig()
		}
		return 0
	case wmCtlColorStatic:
		switch lParam {
		case swHalo:
			return brHalo
		case swRing:
			return brRing
		}
	case wmClose:
		pShowWindow.Call(hwnd, swHide)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func createSettingsWindow() {
	settingsDPI = systemDPI()

	fontHeight := -(uiFontPt * settingsDPI / 72)
	uiFont, _, _ = pCreateFontW.Call(
		uintptr(fontHeight),
		0, 0, 0, fontWeightNormal, 0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, 0,
		uintptr(strPtr(uiFontName)))

	className := utf16Ptr(settingsClass)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(settingsWndProc),
		hInstance:     hInst,
		hIcon:         hAppIcon,
		hbrBackground: colorBtnFace + 1,
		lpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	r := rect{0, 0, int32(sc(settingsClientW)), int32(sc(settingsClientH))}
	pAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(&r)), style, 0, wsExAppWindow)
	w := uintptr(r.right - r.left)
	h := uintptr(r.bottom - r.top)
	scrW, _, _ := pGetSystemMetrics.Call(smCxScreen)
	scrH, _, _ := pGetSystemMetrics.Call(smCyScreen)

	settingsHwnd, _, _ = pCreateWindowExW.Call(wsExAppWindow,
		uintptr(unsafe.Pointer(className)),
		uintptr(strPtr(appName)),
		style,
		(scrW-w)/2, (scrH-h)/2, w, h,
		0, 0, hInst, 0)

	pSendMessageW.Call(settingsHwnd, wmSetIcon, iconSmall, hAppIcon)
	pSendMessageW.Call(settingsHwnd, wmSetIcon, iconBig, hAppIcon)

	p := settingsHwnd
	cbHalo = mkCtl(p, clsButton, lblHaloCheck, bsAutoCheckbox|wsTabStop, 14, 12, 300, 20, idcChkHalo)
	mkCtl(p, clsStatic, lblColor, 0, 32, 42, 40, 16, 0)
	swHalo = mkCtl(p, clsStatic, "", ssSunken, 80, 40, 40, 18, 0)
	mkCtl(p, clsButton, lblChange, wsTabStop, 132, 37, 80, 24, idcBtnHaloColor)
	cbCycle = mkCtl(p, clsButton, lblCycle, bsAutoCheckbox|wsTabStop, 224, 39, 112, 20, idcChkCycle)
	cbDual = mkCtl(p, clsButton, lblDual, bsAutoCheckbox|wsTabStop, 224, 66, 112, 20, idcChkDual)
	lblHaloSize = mkCtl(p, clsStatic, "", 0, 32, 70, 160, 16, 0)
	tbHalo = mkCtl(p, clsTrackbar, "", wsTabStop, 28, 88, 292, 28, 0)

	cbShake = mkCtl(p, clsButton, lblShakeCheck, bsAutoCheckbox|wsTabStop, 14, 132, 320, 20, idcChkShake)
	mkCtl(p, clsStatic, lblColor, 0, 32, 162, 40, 16, 0)
	swRing = mkCtl(p, clsStatic, "", ssSunken, 80, 160, 40, 18, 0)
	mkCtl(p, clsButton, lblChange, wsTabStop, 132, 157, 80, 24, idcBtnRingColor)
	mkCtl(p, clsButton, lblTest, wsTabStop, 224, 157, 60, 24, idcBtnTest)
	lblSens = mkCtl(p, clsStatic, "", 0, 32, 190, 200, 16, 0)
	tbSens = mkCtl(p, clsTrackbar, "", wsTabStop, 28, 208, 292, 28, 0)

	cbAuto = mkCtl(p, clsButton, lblAutostart, bsAutoCheckbox|wsTabStop, 14, 252, 200, 20, idcChkAutostart)
	mkCtl(p, clsStatic, lblFooter, 0, 14, 282, 330, 28, 0)

	pSendMessageW.Call(tbHalo, tbmSetRangeMin, 1, haloSizeMin)
	pSendMessageW.Call(tbHalo, tbmSetRangeMax, 1, haloSizeMax)
	pSendMessageW.Call(tbSens, tbmSetRangeMin, 1, sensitivityMin)
	pSendMessageW.Call(tbSens, tbmSetRangeMax, 1, sensitivityMax)

	updateSwatchBrushes()
}

func syncSettingsControls() {
	if settingsHwnd == 0 {
		return
	}
	setChecked(cbHalo, cfg.HaloEnabled)
	setChecked(cbCycle, cfg.HaloCycle)
	setChecked(cbDual, cfg.HaloDual)
	setChecked(cbShake, cfg.ShakeEnabled)
	setChecked(cbAuto, autostartEnabled())
	pSendMessageW.Call(tbHalo, tbmSetPos, 1, uintptr(cfg.HaloSize))
	pSendMessageW.Call(tbSens, tbmSetPos, 1, uintptr(cfg.Sensitivity))
	setText(lblHaloSize, fmt.Sprintf(lblHaloSizeFmt, cfg.HaloSize))
	setText(lblSens, fmt.Sprintf(lblSensFmt, cfg.Sensitivity))
	pInvalidateRect.Call(swHalo, 0, 1)
	pInvalidateRect.Call(swRing, 0, 1)
}

func showSettings() {
	if settingsHwnd == 0 {
		createSettingsWindow()
	}
	syncSettingsControls()
	pShowWindow.Call(settingsHwnd, swShow)
	pSetForegroundWindow.Call(settingsHwnd)
}

func pickColor(cur rgb) (rgb, bool) {
	cc := chooseColorT{
		lStructSize:  uint32(unsafe.Sizeof(chooseColorT{})),
		hwndOwner:    settingsHwnd,
		rgbResult:    uint32(colorref(cur)),
		lpCustColors: uintptr(unsafe.Pointer(&customColors[0])),
		flags:        ccRGBInit | ccFullOpen,
	}
	uiModalDepth++
	r, _, _ := pChooseColorW.Call(uintptr(unsafe.Pointer(&cc)))
	uiModalDepth--
	if r == 0 {
		return cur, false
	}
	return fromColorref(cc.rgbResult), true
}
