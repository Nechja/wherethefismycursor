//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

const (
	uiFontBody  = "Segoe UI"
	uiFontTitle = "Bahnschrift"
)

var (
	uiBg       = rgb{15, 15, 23}
	uiCard     = rgb{24, 24, 37}
	uiLine     = rgb{45, 45, 68}
	uiText     = rgb{230, 230, 240}
	uiTextDim  = rgb{130, 130, 152}
	uiAccent   = rgb{0, 216, 255}
	uiAccentDk = rgb{0, 70, 88}
	uiOff      = rgb{50, 50, 74}
	uiKnob     = rgb{242, 242, 248}
)

var (
	titleFont  uintptr
	headerFont uintptr
	bodyFont   uintptr
	smallFont  uintptr

	cursorArrow uintptr
	cursorHand  uintptr

	settingsDPI = fallbackDPI
)

func sc(v int) int { return v * settingsDPI / fallbackDPI }

func sci(v int) int32 { return int32(sc(v)) }

func dimmed(c rgb) rgb {
	return rgb{
		uint8((int(c.r) + 2*int(uiBg.r)) / 3),
		uint8((int(c.g) + 2*int(uiBg.g)) / 3),
		uint8((int(c.b) + 2*int(uiBg.b)) / 3),
	}
}

func createFont(name string, pt, weight int) uintptr {
	f, _, _ := pCreateFontW.Call(
		uintptr(-(pt * settingsDPI / 72)),
		0, 0, 0, uintptr(weight), 0, 0, 0, defaultCharset, 0, 0, cleartypeQuality, 0,
		uintptr(strPtr(name)))
	return f
}

func loadUIResources() {
	titleFont = createFont(uiFontTitle, 15, 700)
	headerFont = createFont(uiFontTitle, 10, 700)
	bodyFont = createFont(uiFontBody, 9, 400)
	smallFont = createFont(uiFontBody, 7, 400)
	cursorArrow, _, _ = pLoadCursorW.Call(0, idcArrow)
	cursorHand, _, _ = pLoadCursorW.Call(0, idcHand)
}

func solidBrush(c rgb) uintptr {
	b, _, _ := pCreateSolidBrush.Call(colorref(c))
	return b
}

func fillRect(dc uintptr, r rect, c rgb) {
	b := solidBrush(c)
	pFillRect.Call(dc, uintptr(unsafe.Pointer(&r)), b)
	pDeleteObject.Call(b)
}

func fillRound(dc uintptr, r rect, radius int, fill rgb, border rgb, borderW int) {
	b := solidBrush(fill)
	var pn uintptr
	if borderW > 0 {
		pn, _, _ = pCreatePen.Call(penSolid, uintptr(borderW), colorref(border))
	} else {
		pn, _, _ = pGetStockObject.Call(nullPen)
	}
	ob, _, _ := pSelectObject.Call(dc, b)
	op, _, _ := pSelectObject.Call(dc, pn)
	d := uintptr(sc(radius) * 2)
	pRoundRect.Call(dc, uintptr(r.left), uintptr(r.top), uintptr(r.right), uintptr(r.bottom), d, d)
	pSelectObject.Call(dc, ob)
	pSelectObject.Call(dc, op)
	pDeleteObject.Call(b)
	if borderW > 0 {
		pDeleteObject.Call(pn)
	}
}

func fillCircle(dc uintptr, cx, cy, radius int32, c rgb) {
	b := solidBrush(c)
	pn, _, _ := pGetStockObject.Call(nullPen)
	ob, _, _ := pSelectObject.Call(dc, b)
	op, _, _ := pSelectObject.Call(dc, pn)
	pEllipse.Call(dc, uintptr(cx-radius), uintptr(cy-radius), uintptr(cx+radius+1), uintptr(cy+radius+1))
	pSelectObject.Call(dc, ob)
	pSelectObject.Call(dc, op)
	pDeleteObject.Call(b)
}

func drawText(dc uintptr, r rect, s string, font uintptr, c rgb, flags uintptr) {
	pSelectObject.Call(dc, font)
	pSetTextColor.Call(dc, colorref(c))
	pSetBkMode.Call(dc, bkTransparent)
	pDrawTextW.Call(dc, uintptr(strPtr(s)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
}

func textWidth(dc uintptr, s string, font uintptr) int32 {
	pSelectObject.Call(dc, font)
	u, _ := syscall.UTF16FromString(s)
	var ext sizeT
	pGetTextExtentPoint32W.Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&ext)))
	return ext.cx
}

func textOut(dc uintptr, x, y int32, s string, c rgb) int32 {
	pSetTextColor.Call(dc, colorref(c))
	u, _ := syscall.UTF16FromString(s)
	pTextOutW.Call(dc, uintptr(x), uintptr(y), uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1))
	var ext sizeT
	pGetTextExtentPoint32W.Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&ext)))
	return ext.cx
}

func drawPill(dc uintptr, pill rect, on, hot bool) {
	fill := uiOff
	knob := uiTextDim
	if on {
		fill = uiAccent
		knob = uiKnob
	}
	borderW := 0
	if hot {
		borderW = 1
	}
	h := pill.bottom - pill.top
	fillRound(dc, pill, int(h)/2, fill, uiKnob, borderW)
	knobR := h/2 - sci(3)
	kx := pill.left + h/2
	if on {
		kx = pill.right - h/2
	}
	fillCircle(dc, kx, (pill.top+pill.bottom)/2, knobR, knob)
}
