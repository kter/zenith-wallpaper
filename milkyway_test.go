package main

import (
	"image"
	"image/color"
	"testing"
)

func TestLoadMilkyWay(t *testing.T) {
	mw, err := LoadMilkyWay()
	if err != nil {
		t.Fatalf("LoadMilkyWay: %v", err)
	}
	if mw.width <= 0 || mw.height <= 0 {
		t.Errorf("decoded panorama is %dx%d", mw.width, mw.height)
	}
	// An equirectangular all-sky panorama is 2:1.
	if mw.width != 2*mw.height {
		t.Errorf("panorama is %dx%d, want 2:1 equirectangular", mw.width, mw.height)
	}
}

func TestMilkyWaySampleUniform(t *testing.T) {
	want := color.RGBA{10, 20, 30, 255}
	mw := testMilkyWay(want)
	for _, c := range []struct{ l, b float64 }{
		{0, 0}, {179.9, 45}, {-180, -45}, {90, 90}, {-90, 12.3},
	} {
		if got := mw.Sample(c.l, c.b); got != want {
			t.Errorf("Sample(%v, %v) = %+v, want %+v", c.l, c.b, got, want)
		}
	}
}

func TestMilkyWaySampleWraparound(t *testing.T) {
	// A non-uniform gradient catches wraparound errors that a uniform
	// panorama would hide.
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x * 30), G: uint8(y * 60), B: 128, A: 255})
		}
	}
	mw := &MilkyWay{img: img, width: 8, height: 4}

	for _, c := range []struct{ l, b float64 }{
		{170, 10}, {-170, -10}, {0, 0}, {45, 30},
	} {
		base := mw.Sample(c.l, c.b)
		if plus := mw.Sample(c.l+360, c.b); plus != base {
			t.Errorf("Sample(%v+360, %v) = %+v, want %+v", c.l, c.b, plus, base)
		}
		if minus := mw.Sample(c.l-360, c.b); minus != base {
			t.Errorf("Sample(%v-360, %v) = %+v, want %+v", c.l, c.b, minus, base)
		}
	}
}

func TestMilkyWaySampleOrientation(t *testing.T) {
	// Top half white (b > 0), bottom half black: the north galactic pole
	// must sample bright, the south pole dark.
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		c := color.RGBA{0, 0, 0, 255}
		if y < 2 {
			c = color.RGBA{255, 255, 255, 255}
		}
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	mw := &MilkyWay{img: img, width: 8, height: 4}

	if got := mw.Sample(0, 80); got.R < 200 {
		t.Errorf("Sample near north pole = %+v, want bright (b=+90 at top)", got)
	}
	if got := mw.Sample(0, -80); got.R > 55 {
		t.Errorf("Sample near south pole = %+v, want dark (b=-90 at bottom)", got)
	}
}
