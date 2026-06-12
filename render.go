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

	// Dome: radius = half-diagonal so the projection disk circumscribes the
	// rectangle and every pixel (including corners) is covered by sky.
	cx := float64(rw) / 2.0
	cy := float64(rh) / 2.0
	radius := math.Hypot(cx, cy)

	renderMilkyWay(img, ot, mw, cx, cy, radius)
	renderStars(img, ot, stars, cx, cy, radius)
	renderPlanets(img, planets, cx, cy, radius)

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
