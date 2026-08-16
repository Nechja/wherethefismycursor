//go:build windows

package main

import (
	"fmt"
	"math"
	"syscall"
	"unsafe"
)

type widgetKind int

const (
	wTitle widgetKind = iota
	wHeader
	wLabel
	wCard
	wSwatch
	wSlider
	wButton
	wToggle
	wFooter
	wVersion
)

type widget struct {
	kind       widgetKind
	x, y, w, h int
	label      string
	muted      bool
	textFlags  uintptr
	style      string
	get        func() bool
	set        func(bool)
	hex        *string
	val        func() int
	setVal     func(int)
	min, max   int
	valueFmt   string
	onClick    func()
	dim        func() bool
	wake       func()
	visible    func() bool
}

const (
	uiClientW  = 372
	uiMargin   = 18
	uiRowH     = 30
	uiHeaderH  = 22
	uiTitleH   = 30
	uiSliderH  = 46
	uiSwatchW  = 52
	uiSwatchH  = 24
	uiCaptionH = 14
	uiToggleW  = 42
	uiToggleH  = 20
	uiPillW    = 36
	uiPillH    = 18
	uiButtonH  = 32
	uiGap      = 10
	uiCorner   = 6
	uiKnobR    = 7
	uiValueW   = 64

	cardW    = 51
	cardGap  = 6
	cardH    = 58
	cardPrev = 36

	miniCanvas   = 128
	miniHaloSize = 22.0

	lblTitleA = "WHERE THE "
	lblTitleB = "F"
	lblTitleC = " IS MY CURSOR"

	lblHaloSection = "HALO"
	lblRingSection = "RINGS"
	lblSysSection  = "SYSTEM"

	lblSolidCard   = "SOLID"
	lblDualCard    = "DUAL"
	lblSplitCard   = "SPLIT"
	lblCrossCard   = "CROSS"
	lblRainbowCard = "RAINBOW"
	lblPrismCard   = "PRISM"

	lblAutoToggle = "Start with Windows"
	lblColors     = "Colors"
	lblColor      = "Color"
	lblAutoCycle  = "auto-cycling"
	lblMainCap    = "MAIN"
	lblRimCap     = "RIM"
	lblAltCap     = "ALT"
	lblTopCap     = "TOP"
	lblBottomCap  = "BOTTOM"
	lblRingCap    = "RING"
	lblSize       = "Size"
	lblSens       = "Sensitivity"
	lblSizeFmt    = "%d px"
	lblSensFmt    = "%d / 10"
	lblTestButton = "TEST RINGS"
	lblHotkeys    = "Ctrl+Alt+F find   Ctrl+Alt+H halo   Ctrl+Alt+Q quit"
	lblUpdateFmt  = "%s  ·  get %s on GitHub"
)

var (
	settingsHwnd uintptr
	settingsPxW  int32
	settingsPxH  int32
	backDC       uintptr

	miniDC    uintptr
	miniBuf   []uint32
	miniProf  []uint32
	miniProf2 []uint32

	widgets []widget
	hotIdx  = -1
	dragIdx = -1

	customColors [16]uint32
)

func wrect(w *widget) rect {
	return rect{sci(w.x), sci(w.y), sci(w.x + w.w), sci(w.y + w.h)}
}

func invalidateSettings() {
	if settingsHwnd != 0 {
		pInvalidateRect.Call(settingsHwnd, 0, 0)
	}
}

func isDim(w *widget) bool { return w.dim != nil && w.dim() }

func isVisible(w *widget) bool { return w.visible == nil || w.visible() }

func buildWidgets() int {
	widgets = widgets[:0]
	rowW := uiClientW - 2*uiMargin
	add := func(w widget) { widgets = append(widgets, w) }

	haloOff := func() bool { return !cfg.HaloEnabled }
	wakeHalo := func() { cfg.HaloEnabled = true }
	shakeOff := func() bool { return !cfg.ShakeEnabled }
	wakeShake := func() { cfg.ShakeEnabled = true }
	forStyles := func(styles ...string) func() bool {
		return func() bool {
			for _, s := range styles {
				if cfg.HaloStyle == s {
					return true
				}
			}
			return false
		}
	}
	haloSwatch := func(slot int, caption string, hex *string, visible func() bool) widget {
		return widget{kind: wSwatch,
			x: uiClientW - uiMargin - slot*uiSwatchW - (slot-1)*12, y: 0,
			w: uiSwatchW, h: uiSwatchH + uiCaptionH,
			label: caption, hex: hex, dim: haloOff, wake: wakeHalo, visible: visible}
	}

	y := 14
	add(widget{kind: wTitle, x: uiMargin, y: y, w: rowW, h: uiTitleH})
	y += uiTitleH + uiGap

	add(widget{kind: wHeader, x: uiMargin, y: y, w: rowW, h: uiHeaderH, label: lblHaloSection,
		get: func() bool { return cfg.HaloEnabled },
		set: func(v bool) { cfg.HaloEnabled = v; saveConfig() }})
	y += uiHeaderH + 6
	cards := []struct{ style, caption string }{
		{styleSolid, lblSolidCard},
		{styleDual, lblDualCard},
		{styleSplit, lblSplitCard},
		{styleCross, lblCrossCard},
		{styleRainbow, lblRainbowCard},
		{stylePrism, lblPrismCard},
	}
	for i, c := range cards {
		add(widget{kind: wCard, x: uiMargin + i*(cardW+cardGap), y: y, w: cardW, h: cardH,
			label: c.caption, style: c.style, dim: haloOff, wake: wakeHalo})
	}
	y += cardH + uiGap
	add(widget{kind: wLabel, x: uiMargin, y: y, w: 120, h: uiSwatchH, label: lblColors, dim: haloOff})
	add(widget{kind: wLabel, x: uiClientW - uiMargin - 120, y: y, w: 120, h: uiSwatchH,
		label: lblAutoCycle, muted: true, textFlags: dtRight,
		dim: haloOff, visible: forStyles(styleRainbow, stylePrism)})
	for _, sw := range []widget{
		haloSwatch(2, lblMainCap, &cfg.HaloColor, forStyles(styleSolid, styleDual, styleCross)),
		haloSwatch(1, lblRimCap, &cfg.HaloColor2, forStyles(styleDual)),
		haloSwatch(1, lblAltCap, &cfg.HaloColor2, forStyles(styleCross)),
		haloSwatch(2, lblTopCap, &cfg.HaloColor, forStyles(styleSplit)),
		haloSwatch(1, lblBottomCap, &cfg.HaloColor2, forStyles(styleSplit)),
	} {
		sw.y = y
		add(sw)
	}
	y += uiSwatchH + uiCaptionH + uiGap
	add(widget{kind: wSlider, x: uiMargin, y: y, w: rowW, h: uiSliderH,
		label: lblSize, valueFmt: lblSizeFmt, min: haloSizeMin, max: haloSizeMax,
		val: func() int { return cfg.HaloSize }, setVal: func(v int) { cfg.HaloSize = v },
		dim: haloOff, wake: wakeHalo})
	y += uiSliderH + uiGap + 4

	add(widget{kind: wHeader, x: uiMargin, y: y, w: rowW, h: uiHeaderH, label: lblRingSection,
		get: func() bool { return cfg.ShakeEnabled },
		set: func(v bool) { cfg.ShakeEnabled = v; saveConfig() }})
	y += uiHeaderH + 6
	add(widget{kind: wLabel, x: uiMargin, y: y, w: 120, h: uiSwatchH, label: lblColor, dim: shakeOff})
	add(widget{kind: wSwatch, x: uiClientW - uiMargin - uiSwatchW, y: y, w: uiSwatchW, h: uiSwatchH + uiCaptionH,
		label: lblRingCap, hex: &cfg.RingColor, dim: shakeOff, wake: wakeShake})
	y += uiSwatchH + uiCaptionH + uiGap
	add(widget{kind: wSlider, x: uiMargin, y: y, w: rowW, h: uiSliderH,
		label: lblSens, valueFmt: lblSensFmt, min: sensitivityMin, max: sensitivityMax,
		val: func() int { return cfg.Sensitivity }, setVal: func(v int) { cfg.Sensitivity = v },
		dim: shakeOff, wake: wakeShake})
	y += uiSliderH + uiGap
	add(widget{kind: wButton, x: uiMargin, y: y, w: rowW, h: uiButtonH,
		label: lblTestButton, onClick: triggerRings})
	y += uiButtonH + uiGap + 4

	add(widget{kind: wHeader, x: uiMargin, y: y, w: rowW, h: uiHeaderH, label: lblSysSection})
	y += uiHeaderH + 6
	add(widget{kind: wToggle, x: uiMargin, y: y, w: rowW, h: uiRowH, label: lblAutoToggle,
		get: autostartEnabled, set: setAutostart})
	y += uiRowH + 8

	add(widget{kind: wFooter, x: uiMargin, y: y, w: rowW, h: 16, label: lblHotkeys})
	y += 16 + 4
	add(widget{kind: wVersion, x: uiMargin, y: y, w: rowW, h: 14})
	return y + 14 + 10
}

func drawTitle(dc uintptr, w *widget) {
	r := wrect(w)
	pSetBkMode.Call(dc, bkTransparent)
	pSelectObject.Call(dc, titleFont)
	x := r.left
	for _, seg := range []struct {
		s string
		c rgb
	}{{lblTitleA, uiText}, {lblTitleB, uiAccent}, {lblTitleC, uiText}} {
		x += textOut(dc, x, r.top, seg.s, seg.c)
	}
	fillRect(dc, rect{r.left, r.bottom - 3, r.left + sci(44), r.bottom - 1}, uiAccent)
	fillRect(dc, rect{r.left + sci(48), r.bottom - 2, r.right, r.bottom - 1}, uiLine)
}

func drawHeader(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	drawText(dc, r, w.label, headerFont, uiAccent, dtSingleLine|dtVCenter)
	lineX := r.left + textWidth(dc, w.label, headerFont) + sci(10)
	lineEnd := r.right
	if w.get != nil {
		lineEnd = r.right - sci(uiPillW+8)
	}
	cy := (r.top + r.bottom) / 2
	fillRect(dc, rect{lineX, cy, lineEnd, cy + 1}, uiLine)
	if w.get != nil {
		ph := sci(uiPillH)
		drawPill(dc, rect{r.right - sci(uiPillW), cy - ph/2, r.right, cy + ph/2}, w.get(), hot)
	}
}

func drawToggle(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	drawText(dc, r, w.label, bodyFont, uiText, dtSingleLine|dtVCenter)
	pw := sci(uiToggleW)
	ph := sci(uiToggleH)
	cy := (r.top + r.bottom) / 2
	drawPill(dc, rect{r.right - pw, cy - ph/2, r.right, cy + ph/2}, w.get(), hot)
}

func clearMini() {
	for i := range miniBuf {
		miniBuf[i] = 0
	}
}

func drawMiniHalo(dc uintptr, dst rect, style string, dim bool) {
	main, second := haloRGB, halo2RGB
	if dim {
		main = dimmed(main)
		second = dimmed(second)
	}
	clearMini()
	p := effectParams{
		haloAlpha: 1, haloSize: miniHaloSize, pulse: 0.6,
		haloColor: main, haloColor2: second, haloDual: style == styleDual, ringElapsed: -1,
	}
	mode := splitNone
	switch style {
	case styleSplit:
		mode = splitHorizontal
	case styleCross:
		mode = splitDiagonal
	}
	top, ext, hole := buildProfile(p, miniProf)
	miniProf = top
	bottom := top
	if mode != splitNone {
		p2 := p
		p2.haloColor = second
		bottom, _, _ = buildProfile(p2, miniProf2)
		miniProf2 = bottom
	}
	if ext > miniCanvas/2-1 {
		ext = miniCanvas/2 - 1
	}
	rasterizeRadial(miniBuf, miniCanvas, miniCanvas/2, miniCanvas/2, top, bottom, ext, hole, mode)
	pAlphaBlend.Call(dc, uintptr(dst.left), uintptr(dst.top),
		uintptr(dst.right-dst.left), uintptr(dst.bottom-dst.top),
		miniDC, 0, 0, miniCanvas, miniCanvas, blendPremult)
}

func drawRainbowRing(dc uintptr, cx, cy, radius, penW int32, phase float64, dim bool) {
	for i := 0; i < 6; i++ {
		c := cycleColor(phase + float64(i)/6)
		if dim {
			c = dimmed(c)
		}
		pn, _, _ := pCreatePen.Call(penSolid, uintptr(penW), colorref(c))
		op, _, _ := pSelectObject.Call(dc, pn)
		a0 := float64(i) * math.Pi / 3
		a1 := a0 + math.Pi/3
		x1 := cx + int32(math.Cos(a0)*64)
		y1 := cy - int32(math.Sin(a0)*64)
		x2 := cx + int32(math.Cos(a1)*64)
		y2 := cy - int32(math.Sin(a1)*64)
		pArc.Call(dc, uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius+1), uintptr(cy+radius+1),
			uintptr(x1), uintptr(y1), uintptr(x2), uintptr(y2))
		pSelectObject.Call(dc, op)
		pDeleteObject.Call(pn)
	}
}

func drawCard(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	dim := isDim(w)
	sel := cfg.HaloStyle == w.style
	border := uiLine
	borderW := 1
	caption := uiTextDim
	if sel {
		border = uiAccent
		borderW = 2
		caption = uiAccent
	} else if hot {
		border = uiTextDim
	}
	if dim {
		border = dimmed(border)
		caption = dimmed(caption)
	}
	fillRound(dc, r, uiCorner, uiCard, border, borderW)

	prev := sci(cardPrev)
	px := (r.left + r.right - prev) / 2
	pv := rect{px, r.top + sci(4), px + prev, r.top + sci(4) + prev}
	cx := (pv.left + pv.right) / 2
	cy := (pv.top + pv.bottom) / 2
	switch w.style {
	case styleSolid, styleDual, styleSplit, styleCross:
		drawMiniHalo(dc, pv, w.style, dim)
	case styleRainbow:
		drawRainbowRing(dc, cx, cy, sci(12), sci(4), 0, dim)
	case stylePrism:
		drawRainbowRing(dc, cx, cy, sci(13), sci(3), 0, dim)
		drawRainbowRing(dc, cx, cy, sci(7), sci(3), 0.5, dim)
	}
	capR := rect{r.left, r.bottom - sci(15), r.right, r.bottom - sci(3)}
	drawText(dc, capR, w.label, smallFont, caption, dtSingleLine|dtCenter)
}

func drawSwatch(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	dim := isDim(w)
	box := rect{r.left, r.top, r.right, r.top + sci(uiSwatchH)}
	c, _ := parseColor(*w.hex)
	caption := uiTextDim
	border := uiLine
	if dim {
		c = dimmed(c)
		caption = dimmed(caption)
	}
	if hot {
		border = uiAccent
	}
	fillRound(dc, box, uiCorner, c, border, 1)
	capR := rect{r.left, box.bottom + 1, r.right, r.bottom}
	drawText(dc, capR, w.label, smallFont, caption, dtSingleLine|dtCenter)
}

func sliderGeom(w *widget) (x0, x1, cy int32) {
	r := wrect(w)
	x0 = r.left
	x1 = r.right - sci(uiValueW)
	cy = r.bottom - sci(12)
	return
}

func drawSlider(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	dim := isDim(w)
	text, value, fill, knob := uiText, uiAccent, uiAccent, uiKnob
	if dim {
		text, value, fill, knob = dimmed(uiText), dimmed(uiAccent), uiAccentDk, uiTextDim
	}
	labelR := rect{r.left, r.top, r.right, r.top + sci(18)}
	drawText(dc, labelR, w.label, bodyFont, text, dtSingleLine|dtVCenter)
	drawText(dc, labelR, fmt.Sprintf(w.valueFmt, w.val()), bodyFont, value,
		dtSingleLine|dtVCenter|dtRight)

	x0, x1, cy := sliderGeom(w)
	th := sci(3)
	frac := float64(w.val()-w.min) / float64(w.max-w.min)
	kx := x0 + int32(frac*float64(x1-x0))
	fillRound(dc, rect{x0, cy - th, x1, cy + th}, 3, uiOff, uiOff, 0)
	if kx > x0 {
		fillRound(dc, rect{x0, cy - th, kx, cy + th}, 3, fill, fill, 0)
	}
	if !dim && (hot || dragIdx >= 0 && &widgets[dragIdx] == w) {
		knob = uiAccent
	}
	fillCircle(dc, kx, cy, sci(uiKnobR), knob)
}

func drawButton(dc uintptr, w *widget, hot bool) {
	r := wrect(w)
	fill := uiBg
	text := uiAccent
	if hot {
		fill = uiAccentDk
		text = uiKnob
	}
	fillRound(dc, r, uiCorner, fill, uiAccent, 1)
	drawText(dc, r, w.label, headerFont, text, dtSingleLine|dtVCenter|dtCenter)
}

func renderSettings(dc uintptr) {
	fillRect(dc, rect{0, 0, settingsPxW, settingsPxH}, uiBg)
	for i := range widgets {
		w := &widgets[i]
		if !isVisible(w) {
			continue
		}
		hot := i == hotIdx
		switch w.kind {
		case wTitle:
			drawTitle(dc, w)
		case wHeader:
			drawHeader(dc, w, hot)
		case wLabel:
			c := uiText
			if w.muted {
				c = uiTextDim
			}
			if isDim(w) {
				c = dimmed(c)
			}
			drawText(dc, wrect(w), w.label, bodyFont, c, dtSingleLine|dtVCenter|w.textFlags)
		case wCard:
			drawCard(dc, w, hot)
		case wToggle:
			drawToggle(dc, w, hot)
		case wSwatch:
			drawSwatch(dc, w, hot)
		case wSlider:
			drawSlider(dc, w, hot)
		case wButton:
			drawButton(dc, w, hot)
		case wFooter:
			drawText(dc, wrect(w), w.label, smallFont, uiTextDim, dtSingleLine|dtCenter)
		case wVersion:
			if tag, ok := newerVersion(); ok {
				c := uiAccent
				if hot {
					c = uiKnob
				}
				drawText(dc, wrect(w), fmt.Sprintf(lblUpdateFmt, appVersion, tag), smallFont, c,
					dtSingleLine|dtCenter)
			} else {
				drawText(dc, wrect(w), appVersion, smallFont, uiTextDim, dtSingleLine|dtCenter)
			}
		}
	}
}

func widgetAt(mx, my int32) int {
	for i := range widgets {
		w := &widgets[i]
		switch w.kind {
		case wCard, wToggle, wSwatch, wSlider, wButton:
		case wHeader:
			if w.set == nil {
				continue
			}
		case wVersion:
			if _, ok := newerVersion(); !ok {
				continue
			}
		default:
			continue
		}
		if !isVisible(w) {
			continue
		}
		r := wrect(w)
		if mx >= r.left && mx < r.right && my >= r.top && my < r.bottom {
			return i
		}
	}
	return -1
}

func runWake(w *widget) {
	if w.wake != nil && isDim(w) {
		w.wake()
	}
}

func sliderDrag(i int, mx int32) {
	w := &widgets[i]
	x0, x1, _ := sliderGeom(w)
	frac := float64(mx-x0) / float64(x1-x0)
	if frac < 0 {
		frac = 0
	}
	if frac > 1 {
		frac = 1
	}
	v := w.min + int(frac*float64(w.max-w.min)+0.5)
	if v != w.val() {
		w.setVal(v)
		configGen++
		invalidateSettings()
	}
}

func editColor(target *string) {
	cur, _ := parseColor(*target)
	c, ok := pickColor(cur)
	if !ok {
		return
	}
	*target = hexColor(c)
	applyColors()
	saveConfig()
}

func settingsMouseDown(mx, my int32) {
	i := widgetAt(mx, my)
	if i < 0 {
		return
	}
	w := &widgets[i]
	switch w.kind {
	case wHeader, wToggle:
		w.set(!w.get())
	case wCard:
		runWake(w)
		cfg.HaloStyle = w.style
		saveConfig()
	case wSwatch:
		runWake(w)
		editColor(w.hex)
	case wButton:
		w.onClick()
	case wVersion:
		openReleasesPage()
	case wSlider:
		runWake(w)
		dragIdx = i
		pSetCapture.Call(settingsHwnd)
		sliderDrag(i, mx)
	}
	invalidateSettings()
}

func settingsMouseMove(mx, my int32) {
	if dragIdx >= 0 {
		sliderDrag(dragIdx, mx)
		return
	}
	i := widgetAt(mx, my)
	if i != hotIdx {
		hotIdx = i
		invalidateSettings()
	}
	tme := trackMouseEventT{
		cbSize:    uint32(unsafe.Sizeof(trackMouseEventT{})),
		dwFlags:   tmeLeave,
		hwndTrack: settingsHwnd,
	}
	pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
}

func settingsWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		renderSettings(backDC)
		pBitBlt.Call(hdc, 0, 0, uintptr(settingsPxW), uintptr(settingsPxH), backDC, 0, 0, srcCopy)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmEraseBkgnd:
		return 1
	case wmLButtonDown:
		mx, my := mouseXY(lParam)
		settingsMouseDown(mx, my)
		return 0
	case wmLButtonUp:
		if dragIdx >= 0 {
			dragIdx = -1
			pReleaseCapture.Call()
			saveConfig()
			invalidateSettings()
		}
		return 0
	case wmMouseMove:
		mx, my := mouseXY(lParam)
		settingsMouseMove(mx, my)
		return 0
	case wmMouseLeave:
		if hotIdx != -1 {
			hotIdx = -1
			invalidateSettings()
		}
		return 0
	case wmSetCursor:
		if hotIdx >= 0 {
			pSetCursor.Call(cursorHand)
			return 1
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
	clientH := buildWidgets()
	settingsPxW = sci(uiClientW)
	settingsPxH = sci(clientH)
	loadUIResources()

	className := utf16Ptr(settingsClass)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(settingsWndProc),
		hInstance:     hInst,
		hIcon:         hAppIcon,
		hCursor:       cursorArrow,
		hbrBackground: solidBrush(uiBg),
		lpszClassName: className,
	}
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	style := uintptr(wsOverlapped | wsCaption | wsSysMenu | wsMinimizeBox)
	r := rect{0, 0, settingsPxW, settingsPxH}
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

	dark := int32(1)
	pDwmSetWindowAttribute.Call(settingsHwnd, dwmDarkModeAttr,
		uintptr(unsafe.Pointer(&dark)), unsafe.Sizeof(dark))
	pSendMessageW.Call(settingsHwnd, wmSetIcon, iconSmall, hAppIcon)
	pSendMessageW.Call(settingsHwnd, wmSetIcon, iconBig, hAppIcon)

	hdc, _, _ := pGetDC.Call(settingsHwnd)
	backDC, _, _ = pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBitmap.Call(hdc, uintptr(settingsPxW), uintptr(settingsPxH))
	pSelectObject.Call(backDC, bmp)
	pSetStretchBltMode.Call(backDC, halftone)
	pSetBrushOrgEx.Call(backDC, 0, 0, 0)

	miniDC, miniBuf, _ = createCanvas(hdc, miniCanvas)
}

func showSettings() {
	if settingsHwnd == 0 {
		createSettingsWindow()
	}
	invalidateSettings()
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
