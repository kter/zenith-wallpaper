package main

import (
	"bytes"
	"image"
	"image/color"
	"math"
	"testing"
	"time"
)

func TestProjectionRoundTrip(t *testing.T) {
	const cx, cy, radius = 500.0, 400.0, 700.0
	for alt := 1.0; alt <= 90; alt += 8.9 {
		for az := 0.0; az < 360; az += 30 {
			px, py, ok := horizToPixel(alt, az, cx, cy, radius)
			if !ok {
				t.Fatalf("horizToPixel(%v, %v) unexpectedly failed", alt, az)
			}
			alt2, az2, ok := pixelToHoriz(px, py, cx, cy, radius)
			if !ok {
				t.Fatalf("pixelToHoriz of projected (%v, %v) fell outside dome", alt, az)
			}
			if math.Abs(alt2-alt) > 1e-6 {
				t.Errorf("alt round trip: %v → %v", alt, alt2)
			}
			// Azimuth is undefined at the zenith.
			azDiff := math.Abs(mod360(az2-az+180) - 180)
			if alt < 89.9 && azDiff > 1e-6 {
				t.Errorf("az round trip at alt=%v: %v → %v", alt, az, az2)
			}
		}
	}
}

func TestHorizToPixelConventions(t *testing.T) {
	const cx, cy, radius = 500.0, 500.0, 700.0

	// Below the horizon is not projected.
	if _, _, ok := horizToPixel(-0.1, 0, cx, cy, radius); ok {
		t.Error("horizToPixel(alt<0) must return ok=false")
	}

	// The zenith maps to the center.
	px, py, _ := horizToPixel(90, 123, cx, cy, radius)
	if math.Abs(px-cx) > 1e-9 || math.Abs(py-cy) > 1e-9 {
		t.Errorf("zenith at (%v, %v), want center (%v, %v)", px, py, cx, cy)
	}

	// Looking-up view: North=up (smaller y), East=left (smaller x).
	px, py, _ = horizToPixel(45, 0, cx, cy, radius)
	if !(py < cy) || math.Abs(px-cx) > 1e-9 {
		t.Errorf("north at (%v, %v), want directly above center", px, py)
	}
	px, py, _ = horizToPixel(45, 90, cx, cy, radius)
	if !(px < cx) || math.Abs(py-cy) > 1e-9 {
		t.Errorf("east at (%v, %v), want directly left of center", px, py)
	}
}

func TestPixelToHorizOutsideDome(t *testing.T) {
	if _, _, ok := pixelToHoriz(0, 0, 500, 500, 100); ok {
		t.Error("pixel far outside the dome must return ok=false")
	}
	// The horizon ring itself is inside.
	if _, _, ok := pixelToHoriz(500, 500-100, 500, 500, 100); !ok {
		t.Error("pixel on the dome radius must return ok=true")
	}
}

func TestSetPixelBlend(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))

	// Out-of-bounds writes must be silently clipped, not panic.
	setPixelBlend(img, -1, 0, color.RGBA{255, 255, 255, 255})
	setPixelBlend(img, 4, 0, color.RGBA{255, 255, 255, 255})
	setPixelBlend(img, 0, 4, color.RGBA{255, 255, 255, 255})

	// Blending keeps the per-channel maximum.
	img.SetRGBA(1, 1, color.RGBA{R: 100, G: 50, B: 200, A: 255})
	setPixelBlend(img, 1, 1, color.RGBA{R: 50, G: 80, B: 100, A: 10})
	if got, want := img.RGBAAt(1, 1), (color.RGBA{R: 100, G: 80, B: 200, A: 255}); got != want {
		t.Errorf("blend = %+v, want %+v", got, want)
	}
}

func TestFillCircle(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	white := color.RGBA{255, 255, 255, 255}
	fillCircle(img, 10, 10, 3, white)

	if img.RGBAAt(10, 10) != white || img.RGBAAt(13, 10) != white {
		t.Error("center and radius edge must be filled")
	}
	if got := img.RGBAAt(13, 13); got == white {
		t.Error("bounding-box corner outside the circle must stay unfilled")
	}
	// Circles crossing the image edge must clip, not panic.
	fillCircle(img, 0, 0, 5, white)
	fillCircle(img, 19, 19, 5, white)
}

// testMilkyWay builds a small uniform panorama so renderSky tests do not
// depend on the embedded NASA JPEG.
func testMilkyWay(c color.RGBA) *MilkyWay {
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return &MilkyWay{img: img, width: 8, height: 4}
}

func TestRenderSky(t *testing.T) {
	ot := NewObserverTime(tokyo, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	mw := testMilkyWay(color.RGBA{40, 40, 60, 255})
	stars := []Star{
		{RA: 37.954, Dec: 89.264, Mag: 1.97},  // Polaris: always up from Tokyo
		{RA: 101.287, Dec: -16.716, Mag: -1.46}, // Sirius: sometimes below horizon
	}
	planets := []Planet{{Name: "Moon", HorizCoord: HorizCoord{Alt: 45, Az: 180}}}

	const w, h = 64, 40
	img := renderSky(ot, mw, stars, planets, w, h)
	if got := img.Bounds(); got.Dx() != w || got.Dy() != h {
		t.Fatalf("bounds = %v, want %dx%d", got, w, h)
	}

	// Every pixel is covered by sky (the dome circumscribes the rectangle),
	// so nothing may remain fully black at the milky way base color.
	if img.RGBAAt(0, 0).A != 255 {
		t.Error("corner pixel must be opaque")
	}

	// Rendering is deterministic.
	img2 := renderSky(ot, mw, stars, planets, w, h)
	if !bytes.Equal(img.Pix, img2.Pix) {
		t.Error("two renders of the same input differ")
	}
}

func TestRenderSkyTiny(t *testing.T) {
	ot := NewObserverTime(tokyo, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	mw := testMilkyWay(color.RGBA{0, 0, 0, 255})
	// 1x1 output must not panic or divide by zero.
	img := renderSky(ot, mw, nil, nil, 1, 1)
	if got := img.Bounds(); got.Dx() != 1 || got.Dy() != 1 {
		t.Fatalf("bounds = %v, want 1x1", got)
	}
}
