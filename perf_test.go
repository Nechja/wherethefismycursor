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
	o.rasterize(prof, ext, hole)
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
