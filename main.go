//go:build windows

package main

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

const (
	appName           = "wherethefismycursor"
	appWindowClass    = "WhereTheFIsMyCursorApp"
	overlayClass      = "WhereTheFIsMyCursorOverlay"
	settingsClass     = "WhereTheFIsMyCursorSettings"
	mutexName         = `Local\` + appName + `.single`
	trayTipHint       = "shake the mouse to find it"
	msgAlreadyRunning = appName + " is already running. Look for the halo icon in the tray."
)

const (
	hkFire = iota + 1
	hkQuit
	hkToggleHalo
)

const (
	renderTimerID         = 1
	frameIntervalActiveMs = 15
	frameIntervalIdleMs   = 30
)

var appVersion = "dev"

type drawKey struct {
	gen    int
	bucket int
	hue    int
	spin   int
}

var (
	cfg      appConfig
	haloRGB  rgb
	halo2RGB rgb
	ringRGB  rgb

	ov        *overlay
	shake     = newShakeDetector()
	startT    = time.Now()
	ringStart = math.Inf(-1)

	hInst    uintptr
	appHwnd  uintptr
	hAppIcon uintptr

	uiModalDepth int

	configGen int
	lastKey   = drawKey{gen: -1}
	lastPt    point
	lastExt   int
	timerMs   int
)

func applyColors() {
	haloRGB, _ = parseColor(cfg.HaloColor)
	halo2RGB, _ = parseColor(cfg.HaloColor2)
	ringRGB, _ = parseColor(cfg.RingColor)
}

func nowSec() float64 { return time.Since(startT).Seconds() }

func triggerRings() { ringStart = nowSec() }

func quitApp() {
	removeTrayIcon()
	pPostQuitMessage.Call(0)
}

func toggleHalo() {
	cfg.HaloEnabled = !cfg.HaloEnabled
	saveConfig()
	invalidateSettings()
}

func onHotkey(id uintptr) {
	switch id {
	case hkFire:
		triggerRings()
	case hkQuit:
		quitApp()
	case hkToggleHalo:
		toggleHalo()
	}
}

func pulseAt(now float64) float64 {
	return 0.5 + 0.5*math.Sin(now*2*math.Pi*haloPulseHz)
}

func pulseBucket(now float64) int {
	phase := now * haloPulseHz
	return int((phase - math.Floor(phase)) * haloPulseSteps)
}

func bucketPulse(bucket int) float64 {
	phase := (float64(bucket) + 0.5) / haloPulseSteps
	return 0.5 + 0.5*math.Sin(phase*2*math.Pi)
}

func hueBucket(now float64) int {
	phase := now * haloCycleHz
	return int((phase - math.Floor(phase)) * haloHueSteps)
}

func spinBucket(now float64) int {
	phase := now * prismSpinHz
	return int((phase - math.Floor(phase)) * spinSteps)
}

func haloCycles() bool {
	return cfg.HaloStyle == styleRainbow
}

func haloHasRim() bool {
	return cfg.HaloStyle == styleDual
}

func haloSpins() bool {
	return cfg.HaloEnabled && cfg.HaloStyle == stylePrism
}

func haloSplitMode() splitMode {
	switch cfg.HaloStyle {
	case styleSplit:
		return splitHorizontal
	case styleCross:
		return splitDiagonal
	}
	return splitNone
}

func haloDisplayColor(now float64, continuous bool) rgb {
	if !haloCycles() {
		return haloRGB
	}
	phase := now * haloCycleHz
	if !continuous {
		phase = (float64(hueBucket(now)) + 0.5) / haloHueSteps
	}
	return cycleColor(phase)
}

func setFrameInterval(ms int) {
	if ms == timerMs {
		return
	}
	timerMs = ms
	pSetTimer.Call(appHwnd, renderTimerID, uintptr(ms), 0)
}

func frame() {
	if uiModalDepth > 0 {
		ov.hide()
		return
	}
	var pt point
	if r, _, _ := pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt))); r == 0 {
		return
	}
	now := nowSec()

	shake.minSpeed = float64(shakeSpeedBase-cfg.Sensitivity) * shakeSpeedStepPxPerSec
	animating := now-ringStart >= 0 && now-ringStart <= ringDur+flashDur
	if cfg.ShakeEnabled && !animating {
		if shake.update(now, pt.x, pt.y) {
			ringStart = now
			animating = true
		}
	} else {
		shake.update(now, pt.x, pt.y)
		shake.reversals = shake.reversals[:0]
	}

	if !cfg.HaloEnabled && !animating {
		ov.hide()
		setFrameInterval(frameIntervalIdleMs)
		return
	}
	setFrameInterval(frameIntervalActiveMs)

	if !animating {
		key := drawKey{gen: configGen, bucket: pulseBucket(now), hue: -1, spin: -1}
		if haloCycles() {
			key.hue = hueBucket(now)
		}
		if haloSpins() {
			key.spin = spinBucket(now)
		}
		if ov.shown && key == lastKey {
			if pt != lastPt {
				ov.moveTo(pt.x, pt.y, lastExt)
				lastPt = pt
			}
			return
		}
		lastKey = key
	} else {
		lastKey = drawKey{gen: -1}
	}

	p := effectParams{ringElapsed: -1, ringColor: ringRGB}
	if cfg.HaloEnabled {
		p.haloAlpha = 1
		p.haloSize = float64(cfg.HaloSize)
		p.haloColor = haloDisplayColor(now, animating)
		p.haloDual = haloHasRim()
		p.haloCompact = cfg.HaloStyle == styleCompact
		p.haloColor2 = halo2RGB
	}
	if animating {
		p.pulse = pulseAt(now)
		elapsed := now - ringStart
		if elapsed <= ringDur {
			p.ringElapsed = elapsed
		} else if !cfg.HaloEnabled {
			p.haloAlpha = 1 - (elapsed-ringDur)/flashDur
			p.haloSize = float64(cfg.HaloSize)
			p.haloColor = ringRGB
		}
	} else {
		p.pulse = bucketPulse(lastKey.bucket)
	}

	if haloSpins() && p.haloAlpha > 0 {
		renderSpin(p, pt, now, animating)
		return
	}

	mode := splitNone
	if cfg.HaloEnabled && p.haloAlpha > 0 {
		mode = haloSplitMode()
	}
	top, ext, hole := buildProfile(p, ov.profBuf)
	ov.profBuf = top
	bottom := top
	if mode != splitNone {
		p2 := p
		p2.haloColor = halo2RGB
		bottom, _, _ = buildProfile(p2, ov.profBuf2)
		ov.profBuf2 = bottom
	}
	if ext == 0 {
		ov.hide()
		return
	}
	ov.clearPrev()
	ov.rasterize(top, bottom, ext, hole, mode)
	ov.presentRect(pt.x, pt.y, ext)
	lastPt = pt
	lastExt = ext
}

func renderSpin(p effectParams, pt point, now float64, animating bool) {
	phase := now * prismSpinHz
	phase -= math.Floor(phase)
	if !animating {
		phase = (float64(spinBucket(now)) + 0.5) / spinSteps
	}

	var base []uint32
	ext := 0
	if p.ringElapsed >= 0 {
		pr := p
		pr.haloAlpha = 0
		base, ext, _ = buildProfile(pr, ov.profBuf)
		ov.profBuf = base
	}
	aMain, aRim, hExt := buildSpinProfiles(p, ov.spinMain, ov.spinRim)
	ov.spinMain, ov.spinRim = aMain, aRim
	if hExt > ext {
		ext = hExt
	}

	ov.clearPrev()
	rasterizeSpin(ov.buf32, overlaySize, overlayCenter, overlayCenter, base, aMain, aRim, ext, phase)
	ov.presentRect(pt.x, pt.y, ext)
	lastPt = pt
	lastExt = ext
}

func appWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmTimer:
		frame()
		return 0
	case trayCallbackMsg:
		onTrayMessage(lParam)
		return 0
	case wmHotkey:
		onHotkey(wParam)
		return 0
	case wmAppUpdateChecked:
		invalidateSettings()
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func overlayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func messageBox(text string, flags uintptr) {
	pMessageBoxW.Call(0, uintptr(strPtr(text)), uintptr(strPtr(appName)), flags)
}

func createAppWindow() {
	className := utf16Ptr(appWindowClass)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(appWndProc),
		hInstance:     hInst,
		lpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	appHwnd, _, _ = pCreateWindowExW.Call(0,
		uintptr(unsafe.Pointer(className)),
		uintptr(strPtr(appName)),
		wsOverlapped, 0, 0, 0, 0, 0, 0, hInst, 0)
}

func registerHotkeys() {
	pRegisterHotKey.Call(appHwnd, hkFire, modControl|modAlt|modNoRepeat, 'F')
	pRegisterHotKey.Call(appHwnd, hkQuit, modControl|modAlt|modNoRepeat, 'Q')
	pRegisterHotKey.Call(appHwnd, hkToggleHalo, modControl|modAlt|modNoRepeat, 'H')
}

func runMessageLoop() {
	var msg winMsg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

func main() {
	runtime.LockOSThread()

	_, _, mutexErr := pCreateMutexW.Call(0, 0, uintptr(strPtr(mutexName)))
	if mutexErr == syscall.ERROR_ALREADY_EXISTS {
		messageBox(msgAlreadyRunning, mbIconInformation)
		return
	}

	setDPIAware()

	manifestPath := filepath.Join(configDir(), manifestFileName)
	os.WriteFile(manifestPath, []byte(comctlManifest), 0o644)
	enableVisualStyles(manifestPath)

	cfg = loadConfig()
	applyColors()

	hInst, _, _ = pGetModuleHandleW.Call(0)
	createAppWindow()

	var err error
	ov, err = newOverlay(syscall.NewCallback(overlayWndProc))
	if err != nil {
		messageBox("Failed to create overlay: "+err.Error(), mbIconError)
		return
	}

	hAppIcon = makeAppIcon(haloRGB)
	addTrayIcon(appHwnd, hAppIcon, appName+" "+appVersion+": "+trayTipHint)
	defer removeTrayIcon()

	registerHotkeys()
	setFrameInterval(frameIntervalActiveMs)
	startUpdateChecker()

	runMessageLoop()
}
