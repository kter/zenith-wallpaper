package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/godbus/dbus/v5"
)

// Location holds observer coordinates and timezone.
type Location struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	TZ  string  `json:"tz"`
}

var defaultLocation = Location{Lat: 35.6895, Lon: 139.6917, TZ: "Asia/Tokyo"}

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

// GetLocation tries geoclue → ipinfo → cache → default.
func GetLocation() Location {
	if loc, ok := tryGeoclue(); ok {
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

func tryGeoclue() (Location, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	conn, err := dbus.ConnectSystemBus(dbus.WithContext(ctx))
	if err != nil {
		return Location{}, false
	}
	defer conn.Close()

	manager := conn.Object("org.freedesktop.GeoClue2", "/org/freedesktop/GeoClue2/Manager")
	var clientPath dbus.ObjectPath
	call := manager.CallWithContext(ctx, "org.freedesktop.GeoClue2.Manager.GetClient", 0)
	if call.Err != nil {
		return Location{}, false
	}
	if err := call.Store(&clientPath); err != nil {
		return Location{}, false
	}

	client := conn.Object("org.freedesktop.GeoClue2", clientPath)
	_ = client.SetProperty("org.freedesktop.GeoClue2.Client.DesktopId", dbus.MakeVariant("zenith-wallpaper"))
	_ = client.SetProperty("org.freedesktop.GeoClue2.Client.RequestedAccuracyLevel", dbus.MakeVariant(uint32(4)))

	done := make(chan Location, 1)
	_ = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.GeoClue2.Client"),
		dbus.WithMatchMember("LocationUpdated"),
		dbus.WithMatchObjectPath(clientPath),
	)
	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)

	startCall := client.CallWithContext(ctx, "org.freedesktop.GeoClue2.Client.Start", 0)
	if startCall.Err != nil {
		return Location{}, false
	}

	go func() {
		for sig := range signals {
			if sig.Name != "org.freedesktop.GeoClue2.Client.LocationUpdated" {
				continue
			}
			if len(sig.Body) < 2 {
				continue
			}
			locPath, ok := sig.Body[1].(dbus.ObjectPath)
			if !ok {
				continue
			}
			locObj := conn.Object("org.freedesktop.GeoClue2", locPath)
			latV, err1 := locObj.GetProperty("org.freedesktop.GeoClue2.Location.Latitude")
			lonV, err2 := locObj.GetProperty("org.freedesktop.GeoClue2.Location.Longitude")
			if err1 != nil || err2 != nil {
				continue
			}
			lat, ok1 := latV.Value().(float64)
			lon, ok2 := lonV.Value().(float64)
			if !ok1 || !ok2 {
				continue
			}
			done <- Location{Lat: lat, Lon: lon, TZ: inferTZ(lat, lon)}
			return
		}
	}()

	select {
	case loc := <-done:
		_ = client.Call("org.freedesktop.GeoClue2.Client.Stop", 0)
		return loc, true
	case <-ctx.Done():
		return Location{}, false
	}
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
	offset := int(lon/15.0 + 0.5)
	if offset == 0 {
		return "UTC"
	}
	if offset > 0 {
		return fmt.Sprintf("Etc/GMT-%d", offset)
	}
	return fmt.Sprintf("Etc/GMT+%d", -offset)
}
