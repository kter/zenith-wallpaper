//go:build darwin

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GetOutputs queries system_profiler for connected displays. The reported
// _spdisplays_pixels value is the framebuffer resolution in physical pixels,
// matching the physical-pixel convention of the Sway implementation.
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
			// Unlike sway's current_mode, _spdisplays_pixels already reflects
			// the rotated framebuffer (a portrait display reports h > w), so
			// no width/height swap on spdisplays_rotation — swapping here
			// would double-rotate and yield a landscape image for a portrait
			// screen.
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
	// macOS copies the picture into its own wallpaper store and ignores a
	// set request whose path matches the currently configured one, so
	// overwriting the file in place never refreshes the desktop. Apply each
	// render through a uniquely named copy and prune the previous copies.
	target, err := uniqueWallpaperCopy(abs)
	if err != nil {
		return err
	}
	if desktoppr, err := exec.LookPath("desktoppr"); err == nil {
		if msg, err := exec.Command(desktoppr, strconv.Itoa(out.Index), target).CombinedOutput(); err != nil {
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
end tell`, appleScriptQuote(target), appleScriptQuote(out.displayName), appleScriptQuote(target))
	if msg, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(msg)))
	}
	return nil
}

// uniqueWallpaperCopy duplicates abs next to itself under a timestamped name
// (e.g. "Built-in_Display.1718064000123456789.png") and removes stale copies
// from earlier runs of the same output.
func uniqueWallpaperCopy(abs string) (string, error) {
	dir := filepath.Dir(abs)
	ext := filepath.Ext(abs)
	stem := strings.TrimSuffix(filepath.Base(abs), ext)
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("read wallpaper: %w", err)
	}
	target := filepath.Join(dir, fmt.Sprintf("%s.%d%s", stem, time.Now().UnixNano(), ext))
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", fmt.Errorf("copy wallpaper: %w", err)
	}
	// The desktop keeps showing the wallpaper store's internal copy, so the
	// source files of previous runs can be removed even before the new image
	// is applied. The pattern cannot match abs itself ("stem.png" has nothing
	// between the two dots the pattern requires).
	if old, err := filepath.Glob(filepath.Join(dir, stem+".*"+ext)); err == nil {
		for _, p := range old {
			if p != target {
				os.Remove(p)
			}
		}
	}
	return target, nil
}

func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
