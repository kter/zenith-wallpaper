//go:build darwin

package main

// tryPlatformLocation has no native source on macOS; CoreLocation would need
// cgo and a TCC permission prompt for city-level accuracy that ipinfo.io
// already provides. GetLocation falls through to the HTTP and cache sources.
func tryPlatformLocation() (Location, bool) {
	return Location{}, false
}
