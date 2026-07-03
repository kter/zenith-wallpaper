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
	if msg, err := exec.Command("plutil", "-replace", "Spaces", "-xml", "<dict/>", index).CombinedOutput(); err != nil {
		log.Printf("wallpaper store: plutil -replace: %v: %s", err, strings.TrimSpace(string(msg)))
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
		if msg, err := exec.Command(desktoppr, strconv.Itoa(out.Index), target).CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("desktoppr: %w: %s", err, strings.TrimSpace(string(msg)))
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
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if index >= len(lines) {
		return false, true
	}
	// Match on the unique timestamped basename: macOS may report a
	// normalised variant of the full path.
	return strings.Contains(lines[index], filepath.Base(target)), true
}

// setViaOsascript scripts System Events. Desktops are matched by display
// name first; when nothing matches (System Events localises names such as
// "Built-in Liquid Retina Display" on non-English systems, while
// system_profiler always reports English) it falls back to the desktop
// ordinal.
func setViaOsascript(out Output, target string) error {
	script := fmt.Sprintf(`tell application "System Events"
	if (count of desktops) is 1 then
		set picture of every desktop to POSIX file %s
		return "1"
	end if
	set matched to 0
	repeat with d in desktops
		if display name of d is %s then
			set picture of d to POSIX file %s
			set matched to matched + 1
		end if
	end repeat
	return matched as text
end tell`, appleScriptQuote(target), appleScriptQuote(out.displayName), appleScriptQuote(target))
	msg, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("osascript: %w: %s", err, strings.TrimSpace(string(msg)))
	}
	if strings.TrimSpace(string(msg)) != "0" {
		return nil
	}
	log.Printf("osascript: no desktop named %q (localised name?); falling back to desktop %d",
		out.displayName, out.Index+1)
	fallback := fmt.Sprintf(
		`tell application "System Events" to set picture of desktop %d to POSIX file %s`,
		out.Index+1, appleScriptQuote(target))
	if msg, err := exec.Command("osascript", "-e", fallback).CombinedOutput(); err != nil {
		return fmt.Errorf("osascript desktop %d: %w: %s", out.Index+1, err, strings.TrimSpace(string(msg)))
	}
	return nil
}
