//go:build darwin

package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// GetOutputs queries system_profiler for connected displays. The reported
// _spdisplays_pixels value is the framebuffer resolution in physical pixels,
// matching the physical-pixel convention of the Sway implementation.
func GetOutputs() ([]Output, error) {
	raw, err := exec.Command("system_profiler", "SPDisplaysDataType", "-json").Output()
	if err != nil {
		return nil, fmt.Errorf("system_profiler: %w", err)
	}
	return parseSystemProfilerOutputs(raw)
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
