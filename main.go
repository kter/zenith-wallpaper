package main

import (
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"
)

// version is injected at build time via -ldflags "-X main.version=X.Y".
var version = "dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Println("zenith-wallpaper", version)
		return
	}

	loc := GetLocation()
	tz, err := time.LoadLocation(loc.TZ)
	if err != nil {
		tz = time.UTC
	}
	now := time.Now().In(tz)
	ot := NewObserverTime(loc, now)

	log.Printf("location: lat=%.4f lon=%.4f tz=%s", loc.Lat, loc.Lon, loc.TZ)
	log.Printf("time: %s", now.Format(time.RFC3339))

	mw, err := LoadMilkyWay()
	if err != nil {
		log.Fatalf("milkyway: %v", err)
	}

	stars, err := LoadStars(6.5)
	if err != nil {
		log.Fatalf("stars: %v", err)
	}
	log.Printf("loaded %d stars", len(stars))

	planets := PlanetPositions(ot)
	log.Printf("planets/moon visible: %d", len(planets))

	outputs, err := GetOutputs()
	if err != nil {
		log.Fatalf("outputs: %v", err)
	}
	if len(outputs) == 0 {
		log.Fatal("no active outputs found")
	}

	cacheDir := cacheDir()
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		log.Fatalf("cache dir: %v", err)
	}

	// Render every output before applying any wallpaper: rendering takes
	// seconds per display, and applying mid-render would leave the desktop
	// in a transitional state for the whole run.
	type rendered struct {
		out  Output
		path string
	}
	var ready []rendered
	for _, out := range outputs {
		log.Printf("rendering %s (%dx%d)...", out.Name, out.Width, out.Height)
		img := renderSky(ot, mw, stars, planets, out.Width, out.Height)

		path := filepath.Join(cacheDir, sanitize(out.Name)+".png")
		f, err := os.Create(path)
		if err != nil {
			log.Printf("create %s: %v", path, err)
			continue
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			log.Printf("encode %s: %v", path, err)
			continue
		}
		f.Close()
		log.Printf("wrote %s", path)
		ready = append(ready, rendered{out, path})
	}

	for _, r := range ready {
		if err := SetWallpaper(r.out, r.path); err != nil {
			log.Printf("set wallpaper %s: %v (image saved, will apply on next login)", r.out.Name, err)
		}
	}
}

func cacheDir() string {
	base, _ := os.UserCacheDir()
	return filepath.Join(base, "zenith-wallpaper")
}

// sanitize replaces characters unsafe for filenames.
func sanitize(s string) string {
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '/' || c == '\\' || c == ':' || c == ' ' {
			out[i] = '_'
		} else {
			out[i] = c
		}
	}
	return string(out)
}
