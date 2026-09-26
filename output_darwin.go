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
// SetWallpaper once per display, but the store must be inspected/cleaned only
// once per invocation, before the first set.
var wallpaperStoreOnce sync.Once

// clearSpacesOverridesIfNeeded works around a Spaces limitation: since Sonoma
// the wallpaper API only changes the currently visible Space of each display,
// so any Space with its own entry in the wallpaper store keeps a frozen image
// forever. When such per-Space overrides exist, surgically empty only the
// "Spaces" dictionary (never delete the whole store: that would drop the
// display-level configuration too and flash the stock default wallpaper) and
// restart WallpaperAgent; the orphaned Spaces then fall back to the
// display-level wallpaper — the previous sky — until the subsequent set
// requests land. When no overrides exist this is a no-op, so the desktop
// does not flash on normal timer runs.
func clearSpacesOverridesIfNeeded() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	index := filepath.Join(home,
		"Library", "Application Support", "com.apple.wallpaper", "Store", "Index.plist")
	plistXML, err := exec.Command("plutil", "-convert", "xml1", "-o", "-", index).Output()
	if err != nil {
		// No store (pre-Sonoma) or unreadable: nothing to clean.
		return
	}
	n, err := spacesOverrideCount(plistXML)
	if err != nil {
		log.Printf("wallpaper store: %v (skipping Spaces cleanup)", err)
		return
	}
	if n == 0 {
		return
	}
	log.Printf("wallpaper store: clearing %d per-Space override(s) so every Space follows the display wallpaper", n)
	if _, err := runCombined("plutil", "-replace", "Spaces", "-xml", "<dict/>", index); err != nil {
		log.Printf("wallpaper store: plutil -replace: %v", err)
		return
	}
	_ = exec.Command("killall", "WallpaperAgent").Run()
	waitForWallpaperAgent()
}

// waitForWallpaperAgent blocks until WallpaperAgent is running again after a
// killall, so the first set request is not swallowed by a mid-restart agent.
func waitForWallpaperAgent() {
	for i := 0; i < 10; i++ {
		time.Sleep(500 * time.Millisecond)
		if exec.Command("pgrep", "-x", "WallpaperAgent").Run() == nil {
			// The process exists but may still be initialising; give it a
			// moment before firing set requests at it.
			time.Sleep(time.Second)
			return
		}
	}
	log.Print("wallpaper store: WallpaperAgent did not come back within 5s; applying anyway")
}

// SetWallpaper applies an image file as the background for the given display.
// It prefers desktoppr (no Apple Events, so no TCC automation prompt) when
// installed, and falls back to scripting System Events via osascript, which
// requires a one-time automation approval on first run.
func SetWallpaper(out Output, imagePath string) error {
	wallpaperStoreOnce.Do(clearSpacesOverridesIfNeeded)
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
		return setViaDesktoppr(desktoppr, out, target)
	}
	return setViaOsascript(out, target)
}

// setViaDesktoppr sets the wallpaper and verifies it landed by reading the
// per-screen state back, retrying a couple of times: a set request issued
// while WallpaperAgent is (re)starting can be silently dropped.
func setViaDesktoppr(desktoppr string, out Output, target string) error {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(1500 * time.Millisecond)
		}
		if _, err := runCombined(desktoppr, strconv.Itoa(out.Index), target); err != nil {
			lastErr = fmt.Errorf("desktoppr: %w", err)
			continue
		}
		ok, verifiable := desktopprShows(desktoppr, out.Index, target)
		if ok || !verifiable {
			return nil
		}
		lastErr = fmt.Errorf("desktoppr: screen %d did not pick up %s", out.Index, target)
	}
	return lastErr
}

// desktopprShows reports whether desktoppr's read-back output lists the
// target image for the given screen. verifiable is false when the state
// cannot be read, in which case the set is assumed to have worked.
func desktopprShows(desktoppr string, index int, target string) (ok, verifiable bool) {
	raw, err := exec.Command(desktoppr).Output()
	if err != nil {
		return false, false
	}
	return desktopprListsTarget(raw, index, target)
}

// setViaOsascript scripts System Events. Desktops are matched by display
// name first; when nothing matches (System Events localises names such as
// "Built-in Liquid Retina Display" on non-English systems, while
// system_profiler always reports English) it falls back to the desktop
// ordinal.
func setViaOsascript(out Output, target string) error {
	matched, err := runCombined("osascript", "-e", osascriptSetByNameScript(target, out.displayName))
	if err != nil {
		return fmt.Errorf("osascript: %w", err)
	}
	if matched != "0" {
		return nil
	}
	// Expected on non-English systems: System Events localises display
	// names (e.g. "Color LCD" → "カラーLCD") while system_profiler always
	// reports English, so the positional path is the normal one there.
	log.Printf("applying to desktop %d by position (%s: System Events uses a localised display name)",
		out.Index+1, out.displayName)
	if _, err := runCombined("osascript", "-e", osascriptSetByOrdinalScript(out.Index+1, target)); err != nil {
		return fmt.Errorf("osascript desktop %d: %w", out.Index+1, err)
	}
	return nil
}

// runCombined runs a command and returns its trimmed combined output. On
// failure the output is folded into the error, since these tools report the
// reason on stdout/stderr rather than through the exit status.
func runCombined(name string, args ...string) (string, error) {
	msg, err := exec.Command(name, args...).CombinedOutput()
	out := strings.TrimSpace(string(msg))
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, out)
	}
	return out, nil
}
