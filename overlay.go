//go:build windows

package main

import (
	"fmt"
	"math"
	"unsafe"
)

const (
	overlaySize   = 680
	overlayCenter = overlaySize / 2

	ringCount       = 3
	ringStagger     = 0.12
	ringLife        = 0.72
	ringDur         = ringStagger*(ringCount-1) + ringLife
	flashDur        = 0.45
	ringOuterRadius = 310.0
	ringSpacing     = 58.0
	ringEndRadius   = 18.0
	ringFadeInFrac  = 0.10
	ringPeakAlpha   = 0.85
	ringWidthMin    = 4.0
	ringWidthExtra  = 5.0

	haloPulseHz       = 0.9
	haloPulseSteps    = 24
	haloCycleHz       = 1.0 / 6
	haloHueSteps      = 48
	haloCycleSat      = 0.85
	haloPulseGrowth   = 0.16
	haloRingAlpha     = 0.55
	haloCoreAlpha     = 0.22
	haloWidthFactor   = 0.36
	haloWidthMin      = 6.0
	haloCoreFactor    = 0.65
	haloMinBrightness = 0.72

	haloRimOffsetSigmas = 2.0
	haloRimWidthFactor  = 0.7
	haloRimAlpha        = 0.6

	gaussCutoffSigmas = 3.5
	profileScale      = 8

	shakeSpeedBase           = 12
	shakeSpeedStepPxPerSec   = 62.5
	shakeJitterFloorPxPerSec = 187.5
	shakeWindowSec           = 0.6
	shakeReversalsNeeded     = 4
)

type shakeDetector struct {
	haveLast  bool
	lastT     float64
	lastX     int32
	lastY     int32
	pvx, pvy  float64
	pSpeed    float64
	reversals []float64
	minSpeed  float64
}

func newShakeDetector() *shakeDetector {
	return &shakeDetector{minSpeed: (shakeSpeedBase - defaultSensitivity) * shakeSpeedStepPxPerSec}
}

func (s *shakeDetector) update(now float64, x, y int32) bool {
	if !s.haveLast {
		s.haveLast = true
		s.lastT, s.lastX, s.lastY = now, x, y
		return false
	}
	dt := now - s.lastT
	if dt <= 0 {
		return false
	}
	vx := float64(x-s.lastX) / dt
	vy := float64(y-s.lastY) / dt
	s.lastT, s.lastX, s.lastY = now, x, y
	speed := math.Hypot(vx, vy)

	reversed := vx*s.pvx+vy*s.pvy < 0
	if speed > s.minSpeed && s.pSpeed > s.minSpeed && reversed {
		s.reversals = append(s.reversals, now)
	}
	if speed > shakeJitterFloorPxPerSec {
		s.pvx, s.pvy = vx, vy
		s.pSpeed = speed
	}

	keep := s.reversals[:0]
	for _, t := range s.reversals {
		if now-t <= shakeWindowSec {
			keep = append(keep, t)
		}
	}
	s.reversals = keep

	if len(s.reversals) >= shakeReversalsNeeded {
		s.reversals = s.reversals[:0]
		return true
	}
	return false
}

type overlay struct {
	hwnd     uintptr
	screenDC uintptr
	memDC    uintptr
	buf32    []uint32
	zero32   []uint32
	profBuf  []uint32
	prevExt  int
	shown    bool
}

func newOverlay(wndProc uintptr) (*overlay, error) {
	className := utf16Ptr(overlayClass)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProc,
		hInstance:     hInst,
		lpszClassName: className,
	}
	if atom, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); atom == 0 {
		return nil, fmt.Errorf("RegisterClassEx: %v", err)
	}

	exStyle := uintptr(wsExLayered | wsExTransparent | wsExTopmost | wsExToolwindow | wsExNoActivate)
	hwnd, _, err := pCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(strPtr(appName+" overlay")),
		wsPopup,
		0, 0, overlaySize, overlaySize,
		0, 0, hInst, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowEx: %v", err)
	}

	screenDC, _, _ := pGetDC.Call(0)
	memDC, _, _ := pCreateCompatibleDC.Call(screenDC)

	bmi := bitmapInfoHeader{
		size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		width:    overlaySize,
		height:   -overlaySize,
		planes:   1,
		bitCount: 32,
	}
	var bits unsafe.Pointer
	hbm, _, err := pCreateDIBSection.Call(screenDC,
		uintptr(unsafe.Pointer(&bmi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return nil, fmt.Errorf("CreateDIBSection: %v", err)
	}
	pSelectObject.Call(memDC, hbm)

	return &overlay{
		hwnd:     hwnd,
		screenDC: screenDC,
		memDC:    memDC,
		buf32:    unsafe.Slice((*uint32)(bits), overlaySize*overlaySize),
		zero32:   make([]uint32, overlaySize),
	}, nil
}

func (o *overlay) clearPrev() {
	e := o.prevExt
	if e == 0 {
		return
	}
	w := 2*e + 1
	for y := overlayCenter - e; y <= overlayCenter+e; y++ {
		start := y*overlaySize + overlayCenter - e
		copy(o.buf32[start:start+w], o.zero32[:w])
	}
}

func (o *overlay) rasterize(prof []uint32, ext int, hole float64) {
	maxIdx := len(prof) - 1
	extSq := float64(ext) * float64(ext)
	holeSq := hole * hole
	for dy := -ext; dy <= ext; dy++ {
		dySq := float64(dy) * float64(dy)
		chord := int(math.Sqrt(extSq - dySq))
		holeChord := 0
		if holeSq > dySq {
			holeChord = int(math.Sqrt(holeSq - dySq))
		}
		row := (overlayCenter + dy) * overlaySize
		if holeChord > 0 {
			o.span(prof, maxIdx, row, dySq, -chord, -holeChord)
			o.span(prof, maxIdx, row, dySq, holeChord, chord)
		} else {
			o.span(prof, maxIdx, row, dySq, -chord, chord)
		}
	}
}

func (o *overlay) span(prof []uint32, maxIdx, row int, dySq float64, x0, x1 int) {
	for dx := x0; dx <= x1; dx++ {
		dxF := float64(dx)
		idx := int(math.Sqrt(dySq+dxF*dxF) * profileScale)
		if idx > maxIdx {
			idx = maxIdx
		}
		o.buf32[row+overlayCenter+dx] = prof[idx]
	}
}

func (o *overlay) presentRect(cx, cy int32, ext int) {
	side := int32(2*ext + 1)
	ptDst := point{cx - int32(ext), cy - int32(ext)}
	sz := sizeT{side, side}
	ptSrc := point{overlayCenter - int32(ext), overlayCenter - int32(ext)}
	blend := blendFunc{op: acSrcOver, alpha: 255, format: acSrcAlpha}
	pUpdateLayeredWindow.Call(o.hwnd, o.screenDC,
		uintptr(unsafe.Pointer(&ptDst)), uintptr(unsafe.Pointer(&sz)),
		o.memDC, uintptr(unsafe.Pointer(&ptSrc)),
		0, uintptr(unsafe.Pointer(&blend)), ulwAlpha)
	if !o.shown {
		pShowWindow.Call(o.hwnd, swShowNoActivate)
		pSetWindowPos.Call(o.hwnd, hwndTopmost, 0, 0, 0, 0,
			swpNoMove|swpNoSize|swpNoActivate)
		o.shown = true
	}
	o.prevExt = ext
}

func (o *overlay) moveTo(cx, cy int32, ext int) {
	pSetWindowPos.Call(o.hwnd, 0,
		uintptr(int(cx)-ext), uintptr(int(cy)-ext), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate)
}

func (o *overlay) hide() {
	if o.shown {
		pShowWindow.Call(o.hwnd, swHide)
		o.shown = false
	}
}

func easeOutCubic(t float64) float64 {
	u := 1 - t
	return 1 - u*u*u
}

func gauss(x, sigma float64) float64 {
	return math.Exp(-x * x / (2 * sigma * sigma))
}

func clampExtent(v int) int {
	if v > overlayCenter-1 {
		return overlayCenter - 1
	}
	return v
}

func cycleColor(phase float64) rgb {
	h := (phase - math.Floor(phase)) * 6
	sector := int(h)
	f := h - float64(sector)
	floor := 255 * (1 - haloCycleSat)
	lo := uint8(floor)
	down := uint8(255 * (1 - haloCycleSat*f))
	up := uint8(255 * (1 - haloCycleSat*(1-f)))
	switch sector % 6 {
	case 0:
		return rgb{255, up, lo}
	case 1:
		return rgb{down, 255, lo}
	case 2:
		return rgb{lo, 255, up}
	case 3:
		return rgb{lo, down, 255}
	case 4:
		return rgb{up, lo, 255}
	default:
		return rgb{255, lo, down}
	}
}

func sat255(v float64) uint32 {
	if v > 255 {
		return 255
	}
	if v < 0 {
		return 0
	}
	return uint32(v)
}

type activeRing struct{ r, amp, sigma float64 }

func ringsAt(elapsed float64) []activeRing {
	var rings []activeRing
	for i := 0; i < ringCount; i++ {
		lt := (elapsed - float64(i)*ringStagger) / ringLife
		if lt < 0 || lt > 1 {
			continue
		}
		start := ringOuterRadius - float64(i)*ringSpacing
		fadeIn := math.Min(1, lt/ringFadeInFrac)
		rings = append(rings, activeRing{
			r:     start + (ringEndRadius-start)*easeOutCubic(lt),
			amp:   ringPeakAlpha * fadeIn * (1 - lt*lt*lt),
			sigma: ringWidthMin + ringWidthExtra*(1-lt),
		})
	}
	return rings
}

func complement(c rgb) rgb {
	return rgb{255 - c.r, 255 - c.g, 255 - c.b}
}

type effectParams struct {
	haloAlpha   float64
	haloSize    float64
	haloColor   rgb
	haloDual    bool
	pulse       float64
	ringElapsed float64
	ringColor   rgb
}

func buildProfile(p effectParams, buf []uint32) (prof []uint32, ext int, hole float64) {
	var haloR, haloSigma, haloCore, haloGain float64
	var rimR, rimSigma float64
	rimColor := complement(p.haloColor)
	outer := 0.0
	hole = math.Inf(1)
	if p.haloAlpha > 0 {
		haloR = p.haloSize * (1 + haloPulseGrowth*p.pulse)
		haloSigma = math.Max(haloWidthMin, p.haloSize*haloWidthFactor)
		haloCore = p.haloSize * haloCoreFactor
		haloGain = p.haloAlpha * (haloMinBrightness + (1-haloMinBrightness)*p.pulse)
		outer = haloR + haloSigma*gaussCutoffSigmas + 2
		if p.haloDual {
			rimR = haloR + haloSigma*haloRimOffsetSigmas
			rimSigma = haloSigma * haloRimWidthFactor
			outer = rimR + rimSigma*gaussCutoffSigmas + 2
		}
		hole = 0
	}
	var rings []activeRing
	if p.ringElapsed >= 0 {
		rings = ringsAt(p.ringElapsed)
	}
	for _, rg := range rings {
		outer = math.Max(outer, rg.r+rg.sigma*gaussCutoffSigmas)
		hole = math.Min(hole, math.Max(0, rg.r-rg.sigma*gaussCutoffSigmas))
	}
	if outer == 0 {
		return buf, 0, 0
	}
	ext = clampExtent(int(outer) + 1)
	if math.IsInf(hole, 1) {
		hole = 0
	}

	n := ext*profileScale + 2
	if cap(buf) < n {
		buf = make([]uint32, n)
	}
	prof = buf[:n]
	for i := range prof {
		d := float64(i) / profileScale
		ah := 0.0
		rim := 0.0
		if p.haloAlpha > 0 {
			ah = haloGain * (haloRingAlpha*gauss(d-haloR, haloSigma) + haloCoreAlpha*gauss(d, haloCore))
			if p.haloDual {
				rim = haloGain * haloRimAlpha * gauss(d-rimR, rimSigma)
			}
		}
		ar := 0.0
		for _, rg := range rings {
			ar += rg.amp * gauss(d-rg.r, rg.sigma)
		}
		b := sat255(ah*float64(p.haloColor.b) + rim*float64(rimColor.b) + ar*float64(p.ringColor.b))
		g := sat255(ah*float64(p.haloColor.g) + rim*float64(rimColor.g) + ar*float64(p.ringColor.g))
		r := sat255(ah*float64(p.haloColor.r) + rim*float64(rimColor.r) + ar*float64(p.ringColor.r))
		a := sat255((ah + rim + ar) * 255)
		prof[i] = b | g<<8 | r<<16 | a<<24
	}
	return prof, ext, hole
}
