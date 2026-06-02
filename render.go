package main

import (
	"image"
	"image/color"
	imgdraw "image/draw"
	"math"

	xdraw "golang.org/x/image/draw"
)

// renderSky produces the final RGBA image for the given output dimensions.
// Renders at 2× internally then downsamples for anti-aliasing.
func renderSky(
	ot ObserverTime,
	mw *MilkyWay,
	stars []Star,
	planets []Planet,
	outW, outH int,
) *image.RGBA {
	scale := 2
	rw, rh := outW*scale, outH*scale
	img := image.NewRGBA(image.Rect(0, 0, rw, rh))

	imgdraw.Draw(img, img.Bounds(), image.NewUniform(color.Black), image.Point{}, imgdraw.Src)

	// Dome: diameter = longer side
	longer := rw
	if rh > rw {
		longer = rh
	}
	radius := float64(longer) / 2.0
	cx := float64(rw) / 2.0
	cy := float64(rh) / 2.0

	renderMilkyWay(img, ot, mw, cx, cy, radius)
	renderStars(img, ot, stars, cx, cy, radius)
	renderPlanets(img, planets, cx, cy, radius)
	renderHorizon(img, cx, cy, radius)

	out := image.NewRGBA(image.Rect(0, 0, outW, outH))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Over, nil)
	return out
}

// horizToPixel: Lambert azimuthal equal-area, zenith-centered.
// North=up, East=left (looking-up view).
func horizToPixel(altDeg, azDeg, cx, cy, radius float64) (float64, float64, bool) {
	if altDeg < 0 {
		return 0, 0, false
	}
	zenith := (90.0 - altDeg) * math.Pi / 180.0
	rProj := radius * math.Sqrt(2) * math.Sin(zenith/2)
	azRad := azDeg * math.Pi / 180.0
	px := cx - rProj*math.Sin(azRad) // East → left
	py := cy - rProj*math.Cos(azRad) // North → up
	return px, py, true
}

// pixelToHoriz: inverse Lambert. Returns false if outside dome.
func pixelToHoriz(px, py, cx, cy, radius float64) (alt, az float64, ok bool) {
	dx := cx - px
	dy := cy - py
	rProj := math.Sqrt(dx*dx + dy*dy)
	if rProj > radius {
		return 0, 0, false
	}
	sinHalf := rProj / (radius * math.Sqrt2)
	if sinHalf > 1 {
		sinHalf = 1
	}
	zenithRad := 2 * math.Asin(sinHalf)
	alt = 90.0 - zenithRad*180/math.Pi
	az = math.Atan2(dx, dy) * 180 / math.Pi
	az = mod360(az)
	return alt, az, true
}

func renderMilkyWay(img *image.RGBA, ot ObserverTime, mw *MilkyWay, cx, cy, radius float64) {
	b := img.Bounds()
	for py := b.Min.Y; py < b.Max.Y; py++ {
		for px := b.Min.X; px < b.Max.X; px++ {
			alt, az, ok := pixelToHoriz(float64(px), float64(py), cx, cy, radius)
			if !ok {
				continue
			}
			l, bGal := ot.HorizToGalactic(alt, az)
			img.SetRGBA(px, py, mw.Sample(l, bGal))
		}
	}
}

func renderStars(img *image.RGBA, ot ObserverTime, stars []Star, cx, cy, radius float64) {
	for _, s := range stars {
		hc := ot.RADecToHoriz(s.RA, s.Dec)
		if hc.Alt < 0 {
			continue
		}
		px, py, ok := horizToPixel(hc.Alt, hc.Az, cx, cy, radius)
		if !ok {
			continue
		}
		r := math.Max(1.0, 5.0-s.Mag*0.7)
		intensity := math.Min(1.0, (6.5-s.Mag)/6.5)
		alpha := uint8(intensity * 255)
		blue := uint8(200 + intensity*55)
		col := color.RGBA{R: alpha, G: alpha, B: blue, A: alpha}
		fillCircle(img, int(px+0.5), int(py+0.5), int(r+0.5), col)
	}
}

func renderPlanets(img *image.RGBA, planets []Planet, cx, cy, radius float64) {
	for _, p := range planets {
		px, py, ok := horizToPixel(p.Alt, p.Az, cx, cy, radius)
		if !ok {
			continue
		}
		var col color.RGBA
		r := 6
		if p.Name == "Moon" {
			col = color.RGBA{R: 240, G: 240, B: 200, A: 230}
			r = 10
		} else {
			col = color.RGBA{R: 255, G: 200, B: 80, A: 210}
		}
		fillCircle(img, int(px+0.5), int(py+0.5), r, col)
		drawLabel(img, int(px+0.5)+r+4, int(py+0.5)-4, p.Name, col)
	}
}

func renderHorizon(img *image.RGBA, cx, cy, radius float64) {
	ringColor := color.RGBA{R: 60, G: 120, B: 180, A: 160}
	for angle := 0.0; angle < 360.0; angle += 0.05 {
		a := angle * math.Pi / 180.0
		for dr := -2.0; dr <= 2.0; dr += 0.5 {
			r := radius + dr
			px := int(cx + r*math.Sin(a))
			py := int(cy - r*math.Cos(a))
			setPixelBlend(img, px, py, ringColor)
		}
	}

	labelColor := color.RGBA{R: 140, G: 200, B: 255, A: 220}
	cardinals := []struct {
		label string
		az    float64
	}{
		{"N", 0}, {"E", 90}, {"S", 180}, {"W", 270},
	}
	for _, c := range cardinals {
		azRad := c.az * math.Pi / 180.0
		r := radius + 24
		px := int(cx - r*math.Sin(azRad))
		py := int(cy - r*math.Cos(azRad))
		drawLabel(img, px-4, py-6, c.label, labelColor)
	}
}

func fillCircle(img *image.RGBA, cx, cy, r int, c color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				setPixelBlend(img, cx+dx, cy+dy, c)
			}
		}
	}
}

func setPixelBlend(img *image.RGBA, x, y int, c color.RGBA) {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return
	}
	e := img.RGBAAt(x, y)
	img.SetRGBA(x, y, color.RGBA{
		R: maxU8(e.R, c.R),
		G: maxU8(e.G, c.G),
		B: maxU8(e.B, c.B),
		A: maxU8(e.A, c.A),
	})
}

func maxU8(a, b uint8) uint8 {
	if a > b {
		return a
	}
	return b
}

// drawLabel renders text using a 5×7 bitmap font.
func drawLabel(img *image.RGBA, x, y int, text string, c color.RGBA) {
	for i, ch := range text {
		bm, ok := font5x7[ch]
		if !ok {
			continue
		}
		for row := 0; row < 7; row++ {
			for col := 0; col < 5; col++ {
				if bm[row]&(1<<(4-col)) != 0 {
					setPixelBlend(img, x+i*6+col, y+row, c)
				}
			}
		}
	}
}

// font5x7: 5-wide × 7-tall bitmap glyphs for planet names + cardinals.
// Each [7]byte row: bit4=leftmost pixel.
var font5x7 = map[rune][7]byte{
	' ': {0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	'N': {0x11, 0x19, 0x15, 0x13, 0x11, 0x11, 0x11},
	'E': {0x1F, 0x01, 0x01, 0x0F, 0x01, 0x01, 0x1F},
	'S': {0x1F, 0x01, 0x01, 0x1F, 0x10, 0x10, 0x1F},
	'W': {0x11, 0x11, 0x11, 0x15, 0x0A, 0x0A, 0x11},
	'M': {0x11, 0x1B, 0x15, 0x11, 0x11, 0x11, 0x11},
	'o': {0x00, 0x00, 0x0E, 0x11, 0x11, 0x11, 0x0E},
	'n': {0x00, 0x00, 0x0F, 0x11, 0x11, 0x11, 0x11},
	'J': {0x1E, 0x04, 0x04, 0x04, 0x04, 0x14, 0x08},
	'u': {0x00, 0x00, 0x11, 0x11, 0x11, 0x13, 0x1D},
	'p': {0x00, 0x00, 0x0F, 0x11, 0x0F, 0x01, 0x01},
	'i': {0x00, 0x04, 0x00, 0x0C, 0x04, 0x04, 0x0E},
	't': {0x04, 0x04, 0x1F, 0x04, 0x04, 0x04, 0x1E},
	'e': {0x00, 0x00, 0x0E, 0x11, 0x1F, 0x01, 0x0E},
	'r': {0x00, 0x00, 0x16, 0x19, 0x01, 0x01, 0x01},
	'V': {0x11, 0x11, 0x11, 0x11, 0x0A, 0x0A, 0x04},
	's': {0x00, 0x00, 0x0F, 0x01, 0x0E, 0x10, 0x0F},
	'a': {0x00, 0x00, 0x0E, 0x10, 0x1E, 0x11, 0x1E},
	'c': {0x00, 0x00, 0x0E, 0x01, 0x01, 0x01, 0x0E},
	'y': {0x00, 0x00, 0x11, 0x11, 0x1E, 0x10, 0x0E},
	'h': {0x01, 0x01, 0x0F, 0x11, 0x11, 0x11, 0x11},
	'g': {0x00, 0x00, 0x1E, 0x11, 0x1E, 0x10, 0x0E},
	'U': {0x11, 0x11, 0x11, 0x11, 0x11, 0x11, 0x0E},
	'R': {0x0F, 0x11, 0x11, 0x0F, 0x05, 0x09, 0x11},
	'P': {0x0F, 0x11, 0x11, 0x0F, 0x01, 0x01, 0x01},
	'A': {0x04, 0x0A, 0x11, 0x11, 0x1F, 0x11, 0x11},
	'T': {0x1F, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04},
	'k': {0x01, 0x01, 0x09, 0x05, 0x03, 0x05, 0x09},
	'G': {0x0E, 0x11, 0x01, 0x1D, 0x11, 0x11, 0x0E},
	'l': {0x06, 0x02, 0x02, 0x02, 0x02, 0x02, 0x07},
	'b': {0x01, 0x01, 0x0F, 0x11, 0x11, 0x11, 0x0F},
	'd': {0x10, 0x10, 0x1E, 0x11, 0x11, 0x11, 0x1E},
	'C': {0x0E, 0x11, 0x01, 0x01, 0x01, 0x11, 0x0E},
	'x': {0x00, 0x00, 0x11, 0x0A, 0x04, 0x0A, 0x11},
	'q': {0x00, 0x00, 0x1E, 0x11, 0x1E, 0x10, 0x10},
	'H': {0x11, 0x11, 0x11, 0x1F, 0x11, 0x11, 0x11},
	'f': {0x18, 0x04, 0x04, 0x1E, 0x04, 0x04, 0x04},
	'v': {0x00, 0x00, 0x11, 0x11, 0x0A, 0x0A, 0x04},
}
