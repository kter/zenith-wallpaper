package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Location holds observer coordinates and timezone.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	TZ  string  `json:"tz"`
}

// defaultLocation is used only when all location sources fail.
// The Tokyo Imperial Palace is used as the fallback; in practice geoclue or
// ipinfo.io will supply the actual coordinates before this is reached.
var defaultLocation = Location{Lat: 35.6852, Lon: 139.7528, TZ: "Asia/Tokyo"}

func cacheFile() string {
	base, _ := os.UserCacheDir()
	return filepath.Join(base, "zenith-wallpaper", "location.json")
}

func saveLocation(loc Location) {
	path := cacheFile()
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	b, _ := json.Marshal(loc)
	_ = os.WriteFile(path, b, 0o644)
}

func loadCachedLocation() (Location, bool) {
	b, err := os.ReadFile(cacheFile())
	if err != nil {
		return Location{}, false
	}
	var loc Location
	if err := json.Unmarshal(b, &loc); err != nil {
		return Location{}, false
	}
	return loc, true
}

// GetLocation tries the platform source (GeoClue2 on Linux) → ipinfo → cache → default.
func GetLocation() Location {
	if loc, ok := tryPlatformLocation(); ok {
		saveLocation(loc)
		return loc
	}
	if loc, ok := tryIPInfo(); ok {
		saveLocation(loc)
		return loc
	}
	if loc, ok := loadCachedLocation(); ok {
		return loc
	}
	return defaultLocation
}

type ipInfoResp struct {
	Loc      string `json:"loc"`
	Timezone string `json:"timezone"`
}

func tryIPInfo() (Location, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/json", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Location{}, false
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return parseIPInfo(b)
}

// parseIPInfo extracts a Location from an ipinfo.io/json response body.
func parseIPInfo(b []byte) (Location, bool) {
	var r ipInfoResp
	if err := json.Unmarshal(b, &r); err != nil {
		return Location{}, false
	}
	var lat, lon float64
	if _, err := fmt.Sscanf(r.Loc, "%f,%f", &lat, &lon); err != nil {
		return Location{}, false
	}
	tz := r.Timezone
	if tz == "" {
		tz = inferTZ(lat, lon)
	}
	return Location{Lat: lat, Lon: lon, TZ: tz}, true
}

// inferTZ is a coarse fallback when no timezone is returned.
func inferTZ(lat, lon float64) string {
	_ = lat
	// math.Round, not int(x+0.5): truncation would round western (negative)
	// longitudes toward zero and shift them one hour east.
	offset := int(math.Round(lon / 15.0))
	if offset == 0 {
		return "UTC"
	}
	if offset > 0 {
		return fmt.Sprintf("Etc/GMT-%d", offset)
	}
	return fmt.Sprintf("Etc/GMT+%d", -offset)
}
