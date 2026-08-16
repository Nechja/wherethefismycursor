//go:build windows

package main

import "testing"

var benchColor = rgb{255, 210, 90}

func benchOverlay() *overlay {
	return &overlay{
		buf32:  make([]uint32, overlaySize*overlaySize),
		zero32: make([]uint32, overlaySize),
	}
}

func benchFrame(o *overlay, p effectParams) {
	prof, ext, hole := buildProfile(p, o.profBuf)
	o.profBuf = prof
	o.clearPrev()
	o.rasterize(prof, prof, ext, hole, splitNone)
	o.prevExt = ext
}

func BenchmarkIdleHaloFrame(b *testing.B) {
	o := benchOverlay()
	p := effectParams{haloAlpha: 1, haloSize: defaultHaloSize, haloColor: benchColor, ringElapsed: -1}
	for i := 0; i < b.N; i++ {
		p.pulse = bucketPulse(i % haloPulseSteps)
		benchFrame(o, p)
	}
}

func BenchmarkPrismSpinFrame(b *testing.B) {
	o := benchOverlay()
	p := effectParams{haloAlpha: 1, haloSize: defaultHaloSize, ringElapsed: -1}
	for i := 0; i < b.N; i++ {
		p.pulse = bucketPulse(i % haloPulseSteps)
		aMain, aRim, ext := buildSpinProfiles(p, o.spinMain, o.spinRim)
		o.spinMain, o.spinRim = aMain, aRim
		o.clearPrev()
		rasterizeSpin(o.buf32, overlaySize, overlayCenter, overlayCenter, nil, aMain, aRim, ext,
			float64(i%spinSteps)/spinSteps)
		o.prevExt = ext
	}
}

func BenchmarkRingsWorstFrame(b *testing.B) {
	o := benchOverlay()
	p := effectParams{
		haloAlpha: 1, haloSize: defaultHaloSize, haloColor: benchColor,
		ringElapsed: 0.30, ringColor: benchColor,
	}
	for i := 0; i < b.N; i++ {
		p.pulse = pulseAt(float64(i) * 0.016)
		benchFrame(o, p)
	}
}
