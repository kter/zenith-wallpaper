//go:build darwin

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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
	return parseSystemProfilerOutputs(raw)
}

// wallpaperStoreOnce guards the per-run Spaces-override check: main calls
// SetWallpaper once per display, but the store must be inspected/reset only
// once per invocation, before the first set.
var wallpaperStoreOnce sync.Once

// resetWallpaperStoreIfNeeded works around a Spaces limitation: since Sonoma
// the wallpaper API only changes the currently visible Space of each display,
// so any Space with its own entry in the wallpaper store keeps a frozen image
// forever. When such per-Space overrides exist, delete the store and restart
// WallpaperAgent; afterwards every Space follows the display-level
// configuration that the subsequent SetWallpaper calls write. When no
// overrides exist this is a no-op, so the desktop does not flash on normal
// timer runs.
func resetWallpaperStoreIfNeeded() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	index := filepath.Join(home,
		"Library", "Application Support", "com.apple.wallpaper", "Store", "Index.plist")
	plistXML, err := exec.Command("plutil", "-convert", "xml1", "-o", "-", index).Output()
	if err != nil {
		// No store (pre-Sonoma) or unreadable: nothing to reset.
		return
	}
	n, err := spacesOverrideCount(plistXML)
	if err != nil {
		log.Printf("wallpaper store: %v (skipping Spaces reset)", err)
		return
	}
	if n == 0 {
		return
	}
	log.Printf("wallpaper store: clearing %d per-Space override(s) so all Spaces update", n)
	if err := os.Remove(index); err != nil {
		log.Printf("wallpaper store: remove: %v", err)
		return
	}
	_ = exec.Command("killall", "WallpaperAgent").Run()
	// Give the agent a moment to come back before the first set request.
	time.Sleep(2 * time.Second)
}

// SetWallpaper applies an image file as the background for the given display.
// It prefers desktoppr (no Apple Events, so no TCC automation prompt) when
// installed, and falls back to scripting System Events via osascript, which
// requires a one-time automation approval on first run.
func SetWallpaper(out Output, imagePath string) error {
	wallpaperStoreOnce.Do(resetWallpaperStoreIfNeeded)
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
