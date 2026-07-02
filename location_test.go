package main

import (
	"runtime"
	"testing"
)

func TestInferTZ(t *testing.T) {
	tests := []struct {
		lon  float64
		want string
	}{
		{0, "UTC"},
		{7.2, "UTC"},          // rounds to offset 0
		{-7.2, "UTC"},         // rounds to offset 0 from the west as well
		{139.6917, "Etc/GMT-9"}, // Tokyo; Etc zones use POSIX inverted signs
		{-74.0060, "Etc/GMT+5"}, // New York; truncation would give +4
		{151.2093, "Etc/GMT-10"},
		{-122.4194, "Etc/GMT+8"},
		{172.6, "Etc/GMT-12"},
	}
	for _, tt := range tests {
		if got := inferTZ(0, tt.lon); got != tt.want {
			t.Errorf("inferTZ(0, %v) = %q, want %q", tt.lon, got, tt.want)
		}
	}
}

func TestParseIPInfo(t *testing.T) {
	tests := []struct {
		name string
		body string
		want Location
		ok   bool
	}{
		{
			"full response",
			`{"loc": "35.6895,139.6917", "timezone": "Asia/Tokyo"}`,
			Location{Lat: 35.6895, Lon: 139.6917, TZ: "Asia/Tokyo"},
			true,
		},
		{
			"missing timezone falls back to inferTZ",
			`{"loc": "40.7128,-74.0060"}`,
			Location{Lat: 40.7128, Lon: -74.0060, TZ: "Etc/GMT+5"},
			true,
		},
		{"malformed loc", `{"loc": "somewhere"}`, Location{}, false},
		{"empty body", ``, Location{}, false},
		{"not json", `<html>rate limited</html>`, Location{}, false},
	}
	for _, tt := range tests {
		got, ok := parseIPInfo([]byte(tt.body))
		if ok != tt.ok || got != tt.want {
			t.Errorf("%s: parseIPInfo = (%+v, %v), want (%+v, %v)",
				tt.name, got, ok, tt.want, tt.ok)
		}
	}
}

func TestLocationCacheRoundTrip(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("os.UserCacheDir ignores XDG_CACHE_HOME on darwin")
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if _, ok := loadCachedLocation(); ok {
		t.Fatal("loadCachedLocation reported a hit in an empty cache dir")
	}

	want := Location{Lat: 35.6895, Lon: 139.6917, TZ: "Asia/Tokyo"}
	saveLocation(want)
	got, ok := loadCachedLocation()
	if !ok || got != want {
		t.Errorf("round trip = (%+v, %v), want (%+v, true)", got, ok, want)
	}
}
