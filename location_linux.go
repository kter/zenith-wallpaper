//go:build linux

package main

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

// tryPlatformLocation queries GeoClue2 over the system D-Bus.
func tryPlatformLocation() (Location, bool) {
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
