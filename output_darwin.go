//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GetOutputs queries system_profiler for connected displays. The reported
// _spdisplays_pixels value is the native panel resolution, matching the
// physical-pixel convention of the Sway implementation.
func GetOutputs() ([]Output, error) {
	raw, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler: %w", err)
	}
	var data struct {
		Gpus []struct {
			Displays []struct {
				Name       string `json:"_name"`
				Pixels     string `json:"_spdisplays_pixels"`
				Resolution string `json:"_spdisplays_resolution"`
				Rotation   string `json:"spdisplays_rotation"`
			} `json:"spdisplays_ndrvs"`
		} `json:"SPDisplaysDataType"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("system_profiler json: %w", err)
	}

	var outputs []Output
	seen := map[string]int{}
	for _, gpu := range data.Gpus {
		for _, d := range gpu.Displays {
			w, h, ok := parseDimensions(d.Pixels)
			if !ok {
				w, h, ok = parseDimensions(d.Resolution)
			}
			if !ok || w <= 0 || h <= 0 {
				continue
			}
			// Rotation reads "spdisplays_supported" when upright and the
			// angle ("90", "270", ...) when rotated.
			if deg, err := strconv.Atoi(strings.TrimSpace(d.Rotation)); err == nil {
				if deg == 90 || deg == 270 {
					w, h = h, w
				}
			}
			name := d.Name
			seen[name]++
			if n := seen[name]; n > 1 {
				name = fmt.Sprintf("%s %d", name, n)
			}
			outputs = append(outputs, Output{
				Name:        name,
				Index:       len(outputs),
				Width:       w,
				Height:      h,
				displayName: d.Name,
			})
		}
	}
	return outputs, nil
}

func parseDimensions(s string) (w, h int, ok bool) {
	if _, err := fmt.Sscanf(s, "%d x %d", &w, &h); err != nil {
		return 0, 0, false
	}
	return w, h, true
}

// SetWallpaper applies an image file as the background for the given display.
// It prefers desktoppr (no Apple Events, so no TCC automation prompt) when
// installed, and falls back to scripting System Events via osascript, which
// requires a one-time automation approval on first run.
func SetWallpaper(out Output, imagePath string) error {
	abs, err := filepath.Abs(imagePath)
	if err != nil {
		abs = imagePath
	}
	if desktoppr, err := exec.LookPath("desktoppr"); err == nil {
		if msg, err := exec.Command(desktoppr, strconv.Itoa(out.Index), abs).CombinedOutput(); err != nil {
			return fmt.Errorf("desktoppr: %w: %s", err, strings.TrimSpace(string(msg)))
		}
		return nil
	}
	script := fmt.Sprintf(`tell application "System Events"
	if (count of desktops) is 1 then
		set picture of every desktop to POSIX file %s
	else
		repeat with d in desktops
			if display name of d is %s then set picture of d to POSIX file %s
		end repeat
	end if
end tell`, appleScriptQuote(abs), appleScriptQuote(out.displayName), appleScriptQuote(abs))
	if msg, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(msg)))
	}
	return nil
}

func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
