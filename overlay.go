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

	prismSpinHz = 0.25
	spinSteps   = 48
	hueSteps    = 64

	compactHoleFactor         = 0.48
	compactMidFactor          = 0.78
	compactOutlineWidthFactor = 0.02
	compactOutlineWidthMin    = 1.0
	compactAlpha              = 0.95
	compactEdgePx             = 1.0

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
	profBuf2 []uint32
	spinMain []uint8
	spinRim  []uint8
	prevExt  int
	shown    bool
}

var hueLUT [hueSteps]rgb

func init() {
	for i := range hueLUT {
		hueLUT[i] = cycleColor(float64(i) / hueSteps)
	}
}

func createCanvas(refDC uintptr, size int) (uintptr, []uint32, error) {
	memDC, _, _ := pCreateCompatibleDC.Call(refDC)
	bmi := bitmapInfoHeader{
		size:     uint32(unsafe.Sizeof(bitmapInfoHeader{})),
		width:    int32(size),
		height:   -int32(size),
		planes:   1,
		bitCount: 32,
	}
	var bits unsafe.Pointer
	hbm, _, err := pCreateDIBSection.Call(refDC,
		uintptr(unsafe.Pointer(&bmi)), 0,
		uintptr(unsafe.Pointer(&bits)), 0, 0)
	if hbm == 0 || bits == nil {
		return 0, nil, fmt.Errorf("CreateDIBSection: %v", err)
	}
	pSelectObject.Call(memDC, hbm)
	return memDC, unsafe.Slice((*uint32)(bits), size*size), nil
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
	memDC, buf32, err := createCanvas(screenDC, overlaySize)
	if err != nil {
		return nil, err
	}

	return &overlay{
		hwnd:     hwnd,
		screenDC: screenDC,
		memDC:    memDC,
		buf32:    buf32,
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

type splitMode int

const (
	splitNone splitMode = iota
	splitHorizontal
	splitDiagonal
)

func (o *overlay) rasterize(a, b []uint32, ext int, hole float64, mode splitMode) {
	rasterizeRadial(o.buf32, overlaySize, overlayCenter, overlayCenter, a, b, ext, hole, mode)
}

func rasterizeRadial(buf []uint32, stride, cx, cy int, a, b []uint32, ext int, hole float64, mode splitMode) {
	extSq := float64(ext) * float64(ext)
	holeSq := hole * hole
	for dy := -ext; dy <= ext; dy++ {
		rowProf := a
		if mode == splitHorizontal && dy >= 0 {
			rowProf = b
		}
		ady := dy
		if ady < 0 {
			ady = -ady
		}
		dySq := float64(dy) * float64(dy)
		chord := int(math.Sqrt(extSq - dySq))
		holeChord := 0
		if holeSq > dySq {
			holeChord = int(math.Sqrt(holeSq - dySq))
		}
		row := (cy+dy)*stride + cx
		if holeChord > 0 {
			emitSpan(buf, a, b, rowProf, mode, row, dySq, ady, -chord, -holeChord)
			emitSpan(buf, a, b, rowProf, mode, row, dySq, ady, holeChord, chord)
		} else {
			emitSpan(buf, a, b, rowProf, mode, row, dySq, ady, -chord, chord)
		}
	}
}

func emitSpan(buf, a, b, rowProf []uint32, mode splitMode, row int, dySq float64, ady, x0, x1 int) {
	if mode != splitDiagonal {
		radialSpan(buf, rowProf, row, dySq, x0, x1)
		return
	}
	if x0 <= -ady {
		hi := x1
		if hi > -ady {
			hi = -ady
		}
		radialSpan(buf, a, row, dySq, x0, hi)
	}
	lo := x0
	if lo < -ady+1 {
		lo = -ady + 1
	}
	hi := x1
	if hi > ady-1 {
		hi = ady - 1
	}
	if lo <= hi {
		radialSpan(buf, b, row, dySq, lo, hi)
	}
	if x1 >= ady {
		lo = x0
		if lo < ady {
			lo = ady
		}
		radialSpan(buf, a, row, dySq, lo, x1)
	}
}

func radialSpan(buf, prof []uint32, row int, dySq float64, x0, x1 int) {
	maxIdx := len(prof) - 1
	for dx := x0; dx <= x1; dx++ {
		dxF := float64(dx)
		idx := int(math.Sqrt(dySq+dxF*dxF) * profileScale)
		if idx > maxIdx {
			idx = maxIdx
		}
		buf[row+dx] = prof[idx]
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

func buildSpinProfiles(p effectParams, mainBuf, rimBuf []uint8) ([]uint8, []uint8, int) {
	haloR := p.haloSize * (1 + haloPulseGrowth*p.pulse)
	sigma := math.Max(haloWidthMin, p.haloSize*haloWidthFactor)
	core := p.haloSize * haloCoreFactor
	gain := p.haloAlpha * (haloMinBrightness + (1-haloMinBrightness)*p.pulse)
	rimR := haloR + sigma*haloRimOffsetSigmas
	rimSigma := sigma * haloRimWidthFactor
	ext := clampExtent(int(rimR+rimSigma*gaussCutoffSigmas+2) + 1)

	n := ext*profileScale + 2
	if cap(mainBuf) < n {
		mainBuf = make([]uint8, n)
	}
	if cap(rimBuf) < n {
		rimBuf = make([]uint8, n)
	}
	aMain := mainBuf[:n]
	aRim := rimBuf[:n]
	for i := range aMain {
		d := float64(i) / profileScale
		am := gain * (haloRingAlpha*gauss(d-haloR, sigma) + haloCoreAlpha*gauss(d, core))
		aMain[i] = uint8(sat255(am * 255))
		aRim[i] = uint8(sat255(gain * haloRimAlpha * gauss(d-rimR, rimSigma) * 255))
	}
	return aMain, aRim, ext
}

func addSat32(pix, b, g, r, a uint32) uint32 {
	b += pix & 0xFF
	g += (pix >> 8) & 0xFF
	r += (pix >> 16) & 0xFF
	a += (pix >> 24) & 0xFF
	if b > 255 {
		b = 255
	}
	if g > 255 {
		g = 255
	}
	if r > 255 {
		r = 255
	}
	if a > 255 {
		a = 255
	}
	return b | g<<8 | r<<16 | a<<24
}

func rasterizeSpin(buf []uint32, stride, cx, cy int, base []uint32, aMain, aRim []uint8, ext int, phase float64) {
	extSq := float64(ext) * float64(ext)
	aMax := len(aMain) - 1
	bMax := len(base) - 1
	for dy := -ext; dy <= ext; dy++ {
		dyF := float64(dy)
		dySq := dyF * dyF
		chord := int(math.Sqrt(extSq - dySq))
		row := (cy+dy)*stride + cx
		for dx := -chord; dx <= chord; dx++ {
			dxF := float64(dx)
			idx := int(math.Sqrt(dySq+dxF*dxF) * profileScale)
			ai := idx
			if ai > aMax {
				ai = aMax
			}
			am := uint32(aMain[ai])
			ar := uint32(aRim[ai])
			var pix uint32
			if bMax >= 0 {
				bi := idx
				if bi > bMax {
					bi = bMax
				}
				pix = base[bi]
			}
			if am|ar != 0 {
				h := int((math.Atan2(dyF, dxF)/(2*math.Pi)+phase+1)*hueSteps) & (hueSteps - 1)
				c1 := hueLUT[h]
				c2 := hueLUT[(h+hueSteps/2)&(hueSteps-1)]
				bb := (am*uint32(c1.b) + ar*uint32(c2.b)) / 255
				gg := (am*uint32(c1.g) + ar*uint32(c2.g)) / 255
				rr := (am*uint32(c1.r) + ar*uint32(c2.r)) / 255
				pix = addSat32(pix, bb, gg, rr, am+ar)
			}
			buf[row+dx] = pix
		}
	}
}

type effectParams struct {
	haloAlpha   float64
	haloSize    float64
	haloColor   rgb
	haloColor2  rgb
	haloDual    bool
	haloCompact bool
	pulse       float64
	ringElapsed float64
	ringColor   rgb
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func compactSample(p effectParams, d float64) (b, g, r, a float64) {
	holeR := p.haloSize * compactHoleFactor
	midR := p.haloSize * compactMidFactor
	outR := p.haloSize
	ow := math.Max(compactOutlineWidthMin, p.haloSize*compactOutlineWidthFactor)

	innerBand := clamp01((d-holeR)/compactEdgePx) * clamp01((midR-d)/compactEdgePx)
	outerBand := clamp01((d-midR)/compactEdgePx) * clamp01((outR-d)/compactEdgePx)
	outline := 0.0
	for _, edge := range [3]float64{holeR, midR, outR} {
		outline = math.Max(outline, clamp01((ow-math.Abs(d-edge))/compactEdgePx))
	}
	fill := 1 - outline

	gain := compactAlpha * p.haloAlpha
	b = gain * fill * (innerBand*float64(p.haloColor2.b) + outerBand*float64(p.haloColor.b))
	g = gain * fill * (innerBand*float64(p.haloColor2.g) + outerBand*float64(p.haloColor.g))
	r = gain * fill * (innerBand*float64(p.haloColor2.r) + outerBand*float64(p.haloColor.r))
	a = gain * 255 * (outline + fill*(innerBand+outerBand))
	return
}

func buildProfile(p effectParams, buf []uint32) (prof []uint32, ext int, hole float64) {
	var haloR, haloSigma, haloCore, haloGain float64
	var rimR, rimSigma float64
	rimColor := p.haloColor2
	outer := 0.0
	hole = math.Inf(1)
	if p.haloAlpha > 0 {
		haloGain = p.haloAlpha * (haloMinBrightness + (1-haloMinBrightness)*p.pulse)
		hole = 0
		if p.haloCompact {
			ow := math.Max(compactOutlineWidthMin, p.haloSize*compactOutlineWidthFactor)
			outer = p.haloSize + ow + compactEdgePx + 2
			hole = math.Max(0, p.haloSize*compactHoleFactor-ow-compactEdgePx)
		} else {
			haloR = p.haloSize * (1 + haloPulseGrowth*p.pulse)
			haloSigma = math.Max(haloWidthMin, p.haloSize*haloWidthFactor)
			haloCore = p.haloSize * haloCoreFactor
			outer = haloR + haloSigma*gaussCutoffSigmas + 2
			if p.haloDual {
				rimR = haloR + haloSigma*haloRimOffsetSigmas
				rimSigma = haloSigma * haloRimWidthFactor
				outer = rimR + rimSigma*gaussCutoffSigmas + 2
			}
		}
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
		var cb, cg, cr, ca float64
		if p.haloAlpha > 0 {
			if p.haloCompact {
				cb, cg, cr, ca = compactSample(p, d)
			} else {
				ah := haloGain * (haloRingAlpha*gauss(d-haloR, haloSigma) + haloCoreAlpha*gauss(d, haloCore))
				rim := 0.0
				if p.haloDual {
					rim = haloGain * haloRimAlpha * gauss(d-rimR, rimSigma)
				}
				cb = ah*float64(p.haloColor.b) + rim*float64(rimColor.b)
				cg = ah*float64(p.haloColor.g) + rim*float64(rimColor.g)
				cr = ah*float64(p.haloColor.r) + rim*float64(rimColor.r)
				ca = (ah + rim) * 255
			}
		}
		ar := 0.0
		for _, rg := range rings {
			ar += rg.amp * gauss(d-rg.r, rg.sigma)
		}
		b := sat255(cb + ar*float64(p.ringColor.b))
		g := sat255(cg + ar*float64(p.ringColor.g))
		r := sat255(cr + ar*float64(p.ringColor.r))
		a := sat255(ca + ar*255)
		prof[i] = b | g<<8 | r<<16 | a<<24
	}
	return prof, ext, hole
}
