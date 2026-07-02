package main

import (
	"math"
	"testing"
	"time"

	"github.com/soniakeys/meeus/v3/julian"
)

var tokyo = Location{Lat: 35.6895, Lon: 139.6917, TZ: "Asia/Tokyo"}

func TestJulianDateAnchor(t *testing.T) {
	// J2000.0 epoch: 2000-01-01 12:00 UT is exactly JD 2451545.0.
	jd := julian.TimeToJD(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC))
	if math.Abs(jd-2451545.0) > 1e-9 {
		t.Errorf("TimeToJD(J2000.0) = %f, want 2451545.0", jd)
	}
}

// TestRADecToHorizMeeus13b anchors the full transform against worked example
// 13.b from Meeus, "Astronomical Algorithms" (2nd ed., p. 95): apparent place
// of Venus seen from the US Naval Observatory, Washington DC, on
// 1987 April 10 at 19:21:00 UT. Expected: h = +15.1249°, azimuth 68.0337°
// measured westward from South = 248.0337° in navigation convention.
// This is the regression test for the longitude sign: meeus wants longitude
// positive WEST, so east-positive geographic input must be negated.
func TestRADecToHorizMeeus13b(t *testing.T) {
	washington := Location{Lat: 38.92139, Lon: -77.06556, TZ: "America/New_York"}
	ot := NewObserverTime(washington, time.Date(1987, 4, 10, 19, 21, 0, 0, time.UTC))
	hc := ot.RADecToHoriz(347.3193, -6.71989)

	if math.Abs(hc.Alt-15.1249) > 0.1 {
		t.Errorf("Alt = %.4f, want 15.1249 ±0.1", hc.Alt)
	}
	if math.Abs(hc.Az-248.0337) > 0.1 {
		t.Errorf("Az = %.4f, want 248.0337 ±0.1 (longitude sign flipped?)", hc.Az)
	}
}

// TestRADecToHorizZenith: at J2000.0 the Greenwich sidereal time is
// 280.4606°, so for Tokyo (139.6917°E) the local sidereal time is 60.1523°.
// A star at RA = LST with Dec = latitude stands exactly at the zenith.
func TestRADecToHorizZenith(t *testing.T) {
	ot := NewObserverTime(tokyo, time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC))
	hc := ot.RADecToHoriz(280.4606+tokyo.Lon-360, tokyo.Lat)
	if hc.Alt < 89.9 {
		t.Errorf("Alt = %.4f, want ~90 (star at RA=LST, Dec=latitude must be at zenith)", hc.Alt)
	}
}

// TestRADecToHorizPolaris: Polaris sits within 0.75° of the celestial pole,
// so from any northern site its altitude ≈ latitude and azimuth ≈ North,
// at any time. Also pins the navigation azimuth convention (0=N, 90=E).
func TestRADecToHorizPolaris(t *testing.T) {
	const polarisRA, polarisDec = 37.954, 89.264
	for _, tm := range []time.Time{
		time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	} {
		ot := NewObserverTime(tokyo, tm)
		hc := ot.RADecToHoriz(polarisRA, polarisDec)
		if math.Abs(hc.Alt-tokyo.Lat) > 1.0 {
			t.Errorf("%s: Polaris Alt = %.3f, want latitude %.3f ±1.0", tm, hc.Alt, tokyo.Lat)
		}
		azFromNorth := math.Min(hc.Az, 360-hc.Az)
		if azFromNorth > 1.5 {
			t.Errorf("%s: Polaris Az = %.3f, want within 1.5° of North", tm, hc.Az)
		}
	}
}

// TestHorizToGalacticPole: converting the horizontal position of the north
// galactic pole (J2000 RA 192.8595°, Dec 27.1283°) back through
// HorizToGalactic must give b ≈ +90°.
func TestHorizToGalacticPole(t *testing.T) {
	ot := NewObserverTime(tokyo, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	hc := ot.RADecToHoriz(192.8595, 27.1283)
	_, b := ot.HorizToGalactic(hc.Alt, hc.Az)
	if b < 89.5 {
		t.Errorf("galactic b = %.4f, want > 89.5 at the north galactic pole", b)
	}
}

// TestHorizGalacticRoundTrip: RADecToHoriz and HorizToGalactic share gst/ψ,
// so galactic coordinates of a fixed J2000 direction must not depend on
// observation time or observer location.
func TestHorizGalacticRoundTrip(t *testing.T) {
	const ra, dec = 88.793, 7.407 // Betelgeuse
	var refL, refB float64
	cases := []struct {
		loc Location
		tm  time.Time
	}{
		{tokyo, time.Date(2026, 1, 15, 3, 0, 0, 0, time.UTC)},
		{tokyo, time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)},
		{Location{Lat: -33.87, Lon: 151.21}, time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC)},
	}
	for i, c := range cases {
		ot := NewObserverTime(c.loc, c.tm)
		hc := ot.RADecToHoriz(ra, dec)
		l, b := ot.HorizToGalactic(hc.Alt, hc.Az)
		if i == 0 {
			refL, refB = l, b
			continue
		}
		if math.Abs(l-refL) > 0.01 || math.Abs(b-refB) > 0.01 {
			t.Errorf("case %d: galactic (%.4f, %.4f) != reference (%.4f, %.4f)",
				i, l, b, refL, refB)
		}
	}
}

func TestSolveKepler(t *testing.T) {
	// solveKepler works in degrees: E - deg(e)·sin(E) = M must hold.
	for _, e := range []float64{0, 0.0549, 0.2056} {
		for _, M := range []float64{0, 45, 123.4, 200, 359} {
			E := solveKepler(M, e)
			residual := E - rad2deg(e)*math.Sin(deg2rad(E)) - M
			if math.Abs(residual) > 1e-6 {
				t.Errorf("solveKepler(M=%v, e=%v) = %v, residual %v", M, e, E, residual)
			}
		}
	}
}

func TestMod360(t *testing.T) {
	tests := []struct{ in, want float64 }{
		{0, 0}, {360, 0}, {361, 1}, {-1, 359}, {-361, 359}, {725.5, 5.5},
	}
	for _, tt := range tests {
		if got := mod360(tt.in); math.Abs(got-tt.want) > 1e-12 {
			t.Errorf("mod360(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// angSep returns the angular separation in degrees between two RA/Dec pairs.
func angSep(a, b namedRADec) float64 {
	sa, ca := math.Sincos(deg2rad(a.dec))
	sb, cb := math.Sincos(deg2rad(b.dec))
	cosSep := sa*sb + ca*cb*math.Cos(deg2rad(a.ra-b.ra))
	return rad2deg(math.Acos(math.Max(-1, math.Min(1, cosSep))))
}

// TestMoonMotion: the Moon moves 10–17° per day against the stars.
// Gross errors in the Schlyter elements would break this immediately.
func TestMoonMotion(t *testing.T) {
	for _, d := range []float64{-3543, 0, 9678} {
		m0, m1 := moonRADec(d), moonRADec(d+1)
		sep := angSep(m0, m1)
		if sep < 10 || sep > 17 {
			t.Errorf("d=%v: Moon daily motion = %.2f°, want 10–17°", d, sep)
		}
		if math.Abs(m0.dec) > 30 {
			t.Errorf("d=%v: Moon dec = %.2f, want |dec| ≤ 30", d, m0.dec)
		}
	}
}

// TestPlanetMotion: geocentric planets stay near the ecliptic and move
// slowly; daily motion beyond a few degrees means broken elements.
func TestPlanetMotion(t *testing.T) {
	for _, d := range []float64{-3543, 0, 9678} {
		p0, p1 := planetRADecs(d), planetRADecs(d+1)
		if len(p0) != 5 {
			t.Fatalf("d=%v: got %d planets, want 5", d, len(p0))
		}
		for i := range p0 {
			if p0[i].ra < 0 || p0[i].ra >= 360 {
				t.Errorf("d=%v: %s RA = %v, want [0, 360)", d, p0[i].name, p0[i].ra)
			}
			if math.Abs(p0[i].dec) > 35 {
				t.Errorf("d=%v: %s dec = %v, want |dec| ≤ 35", d, p0[i].name, p0[i].dec)
			}
			if sep := angSep(p0[i], p1[i]); sep > 4 {
				t.Errorf("d=%v: %s daily motion = %.2f°, want < 4°", d, p0[i].name, sep)
			}
		}
	}
}

func TestPlanetPositions(t *testing.T) {
	ot := NewObserverTime(tokyo, time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC))
	planets := PlanetPositions(ot)
	if len(planets) > 6 {
		t.Fatalf("got %d bodies, want at most 6 (Moon + 5 planets)", len(planets))
	}
	valid := map[string]bool{
		"Moon": true, "Mercury": true, "Venus": true,
		"Mars": true, "Jupiter": true, "Saturn": true,
	}
	seen := map[string]bool{}
	for _, p := range planets {
		if !valid[p.Name] {
			t.Errorf("unexpected body %q", p.Name)
		}
		if seen[p.Name] {
			t.Errorf("duplicate body %q", p.Name)
		}
		seen[p.Name] = true
		if p.Alt <= -2 {
			t.Errorf("%s Alt = %v, want > -2 (below-horizon filter)", p.Name, p.Alt)
		}
		if p.Az < 0 || p.Az >= 360 {
			t.Errorf("%s Az = %v, want [0, 360)", p.Name, p.Az)
		}
	}
}
