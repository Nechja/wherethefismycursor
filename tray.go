//go:build windows

package main

import (
	"math"
	"syscall"
	"unsafe"
)

const trayCallbackMsg = wmApp + 1

const (
	cmdSettings = iota + 1
	cmdFire
	cmdHalo
	cmdShake
	cmdAutostart
	cmdQuit
)

const (
	iconSize       = 32
	iconCenter     = (iconSize - 1) / 2.0
	iconRingRadius = 10.0
	iconRingWidth  = 2.4
	iconCoreRadius = 4.5
	iconCoreAlpha  = 0.6
)

type trayMenuItem struct {
	id      int
	label   string
	checked func() bool
	sep     bool
}

var trayData notifyIconData

func makeAppIcon(c rgb) uintptr {
	buf := make([]byte, iconSize*iconSize*4)
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			dx := float64(x) - iconCenter
			dy := float64(y) - iconCenter
			d := math.Sqrt(dx*dx + dy*dy)
			a := math.Min(1, gauss(d-iconRingRadius, iconRingWidth)+iconCoreAlpha*gauss(d, iconCoreRadius))
			i := (y*iconSize + x) * 4
			buf[i+0] = byte(float64(c.b) * a)
			buf[i+1] = byte(float64(c.g) * a)
			buf[i+2] = byte(float64(c.r) * a)
			buf[i+3] = byte(255 * a)
		}
	}
	hbmColor, _, _ := pCreateBitmap.Call(iconSize, iconSize, 1, 32, uintptr(unsafe.Pointer(&buf[0])))
	mask := make([]byte, iconSize*iconSize/8)
	hbmMask, _, _ := pCreateBitmap.Call(iconSize, iconSize, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	ii := iconInfo{fIcon: 1, hbmMask: hbmMask, hbmColor: hbmColor}
	hIcon, _, _ := pCreateIconIndirect.Call(uintptr(unsafe.Pointer(&ii)))
	pDeleteObject.Call(hbmColor)
	pDeleteObject.Call(hbmMask)
	return hIcon
}

func addTrayIcon(hwnd, hIcon uintptr, tip string) {
	trayData = notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             hwnd,
		uID:              1,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: trayCallbackMsg,
		hIcon:            hIcon,
	}
	t, _ := syscall.UTF16FromString(tip)
	copy(trayData.szTip[:len(trayData.szTip)-1], t)
	pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
}

func removeTrayIcon() {
	if trayData.hWnd != 0 {
		pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayData)))
	}
}

func onTrayMessage(lParam uintptr) {
	switch lParam & 0xffff {
	case wmLButtonDblClk:
		showSettings()
	case wmRButtonUp:
		showTrayMenu()
	}
}

func trayMenuItems() []trayMenuItem {
	haloChecked := func() bool { return cfg.HaloEnabled }
	shakeChecked := func() bool { return cfg.ShakeEnabled }
	return []trayMenuItem{
		{id: cmdSettings, label: "Settings…"},
		{id: cmdFire, label: "Find my cursor now\tCtrl+Alt+F"},
		{sep: true},
		{id: cmdHalo, label: "Glowing halo\tCtrl+Alt+H", checked: haloChecked},
		{id: cmdShake, label: "Shake to find", checked: shakeChecked},
		{sep: true},
		{id: cmdAutostart, label: "Start with Windows", checked: autostartEnabled},
		{sep: true},
		{id: cmdQuit, label: "Quit\tCtrl+Alt+Q"},
	}
}

func showTrayMenu() {
	menu, _, _ := pCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	for _, item := range trayMenuItems() {
		flags := uintptr(mfString)
		if item.sep {
			flags = mfSeparator
		}
		if item.checked != nil && item.checked() {
			flags |= mfChecked
		}
		pAppendMenuW.Call(menu, flags, uintptr(item.id), uintptr(strPtr(item.label)))
	}

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(appHwnd)

	uiModalDepth++
	cmd, _, _ := pTrackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd,
		uintptr(pt.x), uintptr(pt.y), 0, appHwnd, 0)
	uiModalDepth--

	pPostMessageW.Call(appHwnd, wmNull, 0, 0)
	pDestroyMenu.Call(menu)
	runTrayCommand(int(cmd))
}

func runTrayCommand(cmd int) {
	switch cmd {
	case cmdSettings:
		showSettings()
	case cmdFire:
		triggerRings()
	case cmdHalo:
		cfg.HaloEnabled = !cfg.HaloEnabled
		saveConfig()
		syncSettingsControls()
	case cmdShake:
		cfg.ShakeEnabled = !cfg.ShakeEnabled
		saveConfig()
		syncSettingsControls()
	case cmdAutostart:
		setAutostart(!autostartEnabled())
		syncSettingsControls()
	case cmdQuit:
		quitApp()
	}
}
