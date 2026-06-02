package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	_ "image/jpeg"
	"math"
)

//go:embed data/milkyway.jpg
var milkywayJPG []byte

// MilkyWay holds the decoded galactic equirectangular panorama.
// The ESO eso0932a image is in galactic coordinates:
//   - x=0 → l=180°, x=W → l=0° (galactic center at center)
//   - y=0 → b=+90°, y=H → b=-90°
//
// Equivalently: l increases left-to-right from -180 to +180 starting at x=0,
// but the image has galactic center (l=0) at the horizontal center.
type MilkyWay struct {
	img    image.Image
	width  int
	height int
}

// LoadMilkyWay decodes the embedded panorama.
func LoadMilkyWay() (*MilkyWay, error) {
	img, _, err := image.Decode(bytes.NewReader(milkywayJPG))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	return &MilkyWay{img: img, width: b.Dx(), height: b.Dy()}, nil
}

// Sample returns the panorama color at galactic longitude l, latitude b (degrees).
// l ∈ [-180, 180), b ∈ [-90, 90].
// The ESO image has l=0 (galactic center) at the horizontal center (x = W/2).
// l increases to the right for positive l (galactic east).
// b=+90 at top, b=-90 at bottom.
func (mw *MilkyWay) Sample(l, b float64) color.RGBA {
	// Normalise l to [-180, 180)
	for l >= 180 {
		l -= 360
	}
	for l < -180 {
		l += 360
	}

	// x: l=0 → x=W/2; l=180 → x=W or 0 (wrap); l=-180 → x=0
	// The image maps left→right as l goes from -180 to +180.
	fx := (l + 180.0) / 360.0 * float64(mw.width)
	// y: b=+90 → y=0 (top); b=-90 → y=H (bottom)
	fy := (90.0 - b) / 180.0 * float64(mw.height)

	// Bilinear interpolation
	x0 := int(math.Floor(fx)) % mw.width
	y0 := int(math.Floor(fy))
	x1 := (x0 + 1) % mw.width
	y1 := y0 + 1

	if x0 < 0 {
		x0 += mw.width
	}
	if y0 < 0 {
		y0 = 0
	}
	if y1 >= mw.height {
		y1 = mw.height - 1
	}

	tx := fx - math.Floor(fx)
	ty := fy - math.Floor(fy)

	c00 := colorToFloats(mw.img.At(x0+mw.img.Bounds().Min.X, y0+mw.img.Bounds().Min.Y))
	c10 := colorToFloats(mw.img.At(x1+mw.img.Bounds().Min.X, y0+mw.img.Bounds().Min.Y))
	c01 := colorToFloats(mw.img.At(x0+mw.img.Bounds().Min.X, y1+mw.img.Bounds().Min.Y))
	c11 := colorToFloats(mw.img.At(x1+mw.img.Bounds().Min.X, y1+mw.img.Bounds().Min.Y))

	r := bilerp(c00[0], c10[0], c01[0], c11[0], tx, ty)
	g := bilerp(c00[1], c10[1], c01[1], c11[1], tx, ty)
	b2 := bilerp(c00[2], c10[2], c01[2], c11[2], tx, ty)

	return color.RGBA{
		R: uint8(clamp01(r) * 255),
		G: uint8(clamp01(g) * 255),
		B: uint8(clamp01(b2) * 255),
		A: 255,
	}
}

func colorToFloats(c color.Color) [3]float64 {
	r, g, b, _ := c.RGBA()
	return [3]float64{float64(r) / 65535, float64(g) / 65535, float64(b) / 65535}
}

func bilerp(c00, c10, c01, c11, tx, ty float64) float64 {
	return c00*(1-tx)*(1-ty) + c10*tx*(1-ty) + c01*(1-tx)*ty + c11*tx*ty
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
