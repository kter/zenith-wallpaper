//go:build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// findSwaySock looks for a sway IPC socket in the user's runtime dir,
// falling back to the SWAYSOCK env var. It sets the env var for child processes.
func findSwaySock() {
	if os.Getenv("SWAYSOCK") != "" {
		return
	}
	uid := fmt.Sprintf("%d", os.Getuid())
	pattern := filepath.Join("/run/user", uid, "sway-ipc.*.sock")
	matches, err := filepath.Glob(pattern)
	if err == nil && len(matches) > 0 {
		os.Setenv("SWAYSOCK", matches[0])
	}
}

// GetOutputs queries sway for connected, active outputs.
func GetOutputs() ([]Output, error) {
	findSwaySock()
	out, err := exec.Command("swaymsg", "-t", "get_outputs").Output()
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
		Rect   struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"rect"`
		CurrentMode struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"current_mode"`
		Transform string `json:"transform"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var outputs []Output
	for _, r := range raw {
		if !r.Active {
			continue
		}
		o := Output{
			Name:  r.Name,
			Index: len(outputs),
		}
		// Use physical resolution (current_mode) so the image matches
		// what swaybg displays at 1:1 pixels on HiDPI screens.
		// rect gives logical pixels which are smaller on scaled displays.
		o.Width = r.CurrentMode.Width
		o.Height = r.CurrentMode.Height
		// For 90/270-degree rotated outputs, swap physical dimensions.
		switch r.Transform {
		case "90", "270", "flipped-90", "flipped-270":
			o.Width, o.Height = o.Height, o.Width
		}
		if o.Width > 0 && o.Height > 0 {
			outputs = append(outputs, o)
		}
	}
	return outputs, nil
}

// SetWallpaper applies an image file as the background for the given output.
func SetWallpaper(out Output, imagePath string) error {
	findSwaySock()
	cmd := exec.Command("swaymsg", "output", out.Name, "bg", imagePath, "fill")
	return cmd.Run()
}
