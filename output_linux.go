//go:build linux

package main

import (
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
	return parseSwayOutputs(out)
}

// SetWallpaper applies an image file as the background for the given output.
func SetWallpaper(out Output, imagePath string) error {
	findSwaySock()
	cmd := exec.Command("swaymsg", "output", out.Name, "bg", imagePath, "fill")
	return cmd.Run()
}
