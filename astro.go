package main

import (
	"math"
	"time"

	"github.com/soniakeys/meeus/v3/coord"
	"github.com/soniakeys/meeus/v3/julian"
	"github.com/soniakeys/meeus/v3/precess"
	"github.com/soniakeys/meeus/v3/sidereal"
	"github.com/soniakeys/unit"
)

// HorizCoord is altitude and azimuth in degrees.
// Azimuth: 0=North, 90=East (navigation convention).
type HorizCoord struct {
	Alt, Az float64 // degrees
}

// ObserverTime bundles location and JDE/GST for coordinate transforms.
type ObserverTime struct {
	Loc  Location
	T    time.Time
	jde  float64
	gst  unit.Time  // Greenwich Apparent Sidereal Time
	phi  unit.Angle // observer latitude (radians)
	psi  unit.Angle // observer longitude (radians)
	prec *precess.Precessor // J2000 → B1950
}

func NewObserverTime(loc Location, t time.Time) ObserverTime {
	jde := julian.TimeToJD(t)
	gst := sidereal.Apparent(jde)
	p := precess.NewPrecessor(2000.0, 1950.0)
	return ObserverTime{
		Loc:  loc,
		T:    t,
		jde:  jde,
		gst:  gst,
		phi:  unit.AngleFromDeg(loc.Lat),
		// meeus follows the book's convention of longitude positive WEST of
		// Greenwich (H = θ0 - ψ - α), so the geographic east-positive
		// longitude must be negated here.
		psi:  unit.AngleFromDeg(-loc.Lon),
		prec: p,
	}
}

// RADecToHoriz converts J2000 equatorial (degrees) to horizontal (alt/az).
func (ot ObserverTime) RADecToHoriz(raDeg, decDeg float64) HorizCoord {
	ra := unit.RAFromDeg(raDeg)
	dec := unit.AngleFromDeg(decDeg)
	// meeus EqToHz: A is measured from South (0=S, 90=W, 180=N, 270=E)
	A, h := coord.EqToHz(ra, dec, ot.phi, ot.psi, ot.gst)
	// Convert to navigation azimuth: N=0, E=90
	az := mod360(A.Deg() + 180.0)
	return HorizCoord{Alt: h.Deg(), Az: az}
}

// HorizToGalactic converts horizontal (alt, az degrees) to galactic (l, b degrees).
// Core of the Milky Way warp: each screen pixel → galactic coords.
func (ot ObserverTime) HorizToGalactic(altDeg, azDeg float64) (l, b float64) {
	// Convert navigation azimuth back to meeus azimuth (measured from South)
	A := unit.AngleFromDeg(azDeg - 180.0)
	h := unit.AngleFromDeg(altDeg)
	ra, dec := coord.HzToEq(A, h, ot.phi, ot.psi, ot.gst)

	// Precess J2000 → B1950 (required by meeus EqToGal)
	eq2000 := &coord.Equatorial{RA: ra, Dec: dec}
	eq1950 := &coord.Equatorial{}
	ot.prec.Precess(eq2000, eq1950)

	lAngle, bAngle := coord.EqToGal(eq1950.RA, eq1950.Dec)
	return lAngle.Deg(), bAngle.Deg()
}

// ---- Planets (low-precision Schlyter method) ----

type Planet struct {
	Name string
	HorizCoord
}

// PlanetPositions returns planets + Moon above (or near) the horizon.
func PlanetPositions(ot ObserverTime) []Planet {
	d := ot.jde - 2451545.0 // days from J2000.0
	var results []Planet

	bodies := []struct{ name string; ra, dec float64 }{moonRADec(d)}
	for _, p := range planetRADecs(d) {
		bodies = append(bodies, p)
	}

	for _, b := range bodies {
		hc := ot.RADecToHoriz(b.ra, b.dec)
		if hc.Alt > -2 {
			results = append(results, Planet{Name: b.name, HorizCoord: hc})
		}
	}
	return results
}

type namedRADec struct{ name string; ra, dec float64 }

// moonRADec returns Moon RA/Dec (degrees) via Schlyter low-precision method.
func moonRADec(d float64) namedRADec {
	N := mod360(125.1228 - 0.0529538083*d)
	iDeg := 5.1454
	w := mod360(318.0634 + 0.1643573223*d)
	a := 60.2666
	e := 0.054900
	M := mod360(115.3654 + 13.0649929509*d)

	E := solveKepler(M, e)
	xv := a * (math.Cos(deg2rad(E)) - e)
	yv := a * math.Sqrt(1-e*e) * math.Sin(deg2rad(E))
	v := math.Atan2(yv, xv)
	r := math.Sqrt(xv*xv + yv*yv)

	lonEcl := v + deg2rad(w)
	Nrad, irad := deg2rad(N), deg2rad(iDeg)

	xecl := r * (math.Cos(Nrad)*math.Cos(lonEcl) - math.Sin(Nrad)*math.Sin(lonEcl)*math.Cos(irad))
	yecl := r * (math.Sin(Nrad)*math.Cos(lonEcl) + math.Cos(Nrad)*math.Sin(lonEcl)*math.Cos(irad))
	zecl := r * math.Sin(lonEcl) * math.Sin(irad)

	return eclToEqRADec("Moon", d, xecl, yecl, zecl)
}

// planetRADecs returns approximate RA/Dec for 5 planets (degrees).
func planetRADecs(d float64) []namedRADec {
	type elem struct {
		name                          string
		N0, Nd, i0, w0, wd, M0, Md   float64
		a, e0, ed                     float64
	}
	planets := []elem{
		{"Mercury", 48.3313, 3.24587e-5, 7.0047, 29.1241, 1.01444e-5, 0.387098, 0.205635, 5.59e-10, 168.6562, 4.0923344368},
		{"Venus", 76.6799, 2.46590e-5, 3.3946, 54.8910, 1.38374e-5, 0.723330, 0.006773, -1.302e-9, 48.0052, 1.6021302244},
		{"Mars", 49.5574, 2.11081e-5, 1.8497, 286.5016, 2.92961e-5, 1.523688, 0.093405, 2.516e-9, 18.6021, 0.5240207766},
		{"Jupiter", 100.4542, 2.76854e-5, 1.3030, 273.8777, 1.64505e-5, 5.20256, 0.048498, 4.469e-9, 19.8950, 0.0830853001},
		{"Saturn", 113.6634, 2.38980e-5, 2.4886, 339.3939, 2.97661e-5, 9.55475, 0.055546, -9.499e-9, 316.9670, 0.0334442282},
	}

	// Earth's position (heliocentric ecliptic)
	Msun := mod360(356.0470 + 0.9856002585*d)
	wsun := mod360(282.9404 + 4.70935e-5*d)
	lonSun := deg2rad(mod360(Msun + wsun + 180))
	xe, ye := math.Cos(lonSun), math.Sin(lonSun)

	var results []namedRADec
	for _, p := range planets {
		N := mod360(p.N0 + p.Nd*d)
		w := mod360(p.w0 + p.wd*d)
		e := p.e0 + p.ed*d
		M := mod360(p.M0 + p.Md*d)

		E := solveKepler(M, e)
		xv := p.a * (math.Cos(deg2rad(E)) - e)
		yv := p.a * math.Sqrt(1-e*e) * math.Sin(deg2rad(E))
		v := math.Atan2(yv, xv)
		r := math.Sqrt(xv*xv + yv*yv)

		lonEcl := v + deg2rad(w)
		Nrad, irad := deg2rad(N), deg2rad(p.i0)

		xh := r * (math.Cos(Nrad)*math.Cos(lonEcl) - math.Sin(Nrad)*math.Sin(lonEcl)*math.Cos(irad))
		yh := r * (math.Sin(Nrad)*math.Cos(lonEcl) + math.Cos(Nrad)*math.Sin(lonEcl)*math.Cos(irad))
		zh := r * math.Sin(lonEcl) * math.Sin(irad)

		// Geocentric
		xg, yg, zg := xh-xe, yh-ye, zh-0.0

		results = append(results, eclToEqRADec(p.name, d, xg, yg, zg))
	}
	return results
}

// eclToEqRADec converts geocentric ecliptic XYZ to RA/Dec degrees.
func eclToEqRADec(name string, d, x, y, z float64) namedRADec {
	oblecl := deg2rad(23.4393 - 3.563e-7*d)
	xeq := x
	yeq := y*math.Cos(oblecl) - z*math.Sin(oblecl)
	zeq := y*math.Sin(oblecl) + z*math.Cos(oblecl)
	ra := mod360(rad2deg(math.Atan2(yeq, xeq)))
	dec := rad2deg(math.Atan2(zeq, math.Sqrt(xeq*xeq+yeq*yeq)))
	return namedRADec{name, ra, dec}
}

// solveKepler iteratively solves Kepler's equation E - e*sin(E) = M.
func solveKepler(M, e float64) float64 {
	Mrad := deg2rad(M)
	E := M + rad2deg(e)*math.Sin(Mrad)
	for i := 0; i < 5; i++ {
		Erad := deg2rad(E)
		E -= (E - rad2deg(e)*math.Sin(Erad) - M) / (1 - e*math.Cos(Erad))
	}
	return E
}

func deg2rad(d float64) float64 { return d * math.Pi / 180 }
func rad2deg(r float64) float64 { return r * 180 / math.Pi }
func mod360(x float64) float64  { return x - 360*math.Floor(x/360) }
