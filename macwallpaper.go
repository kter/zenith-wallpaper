package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Pure helpers for the macOS SetWallpaper paths (desktoppr read-back parsing
// and AppleScript generation). Kept outside the darwin build tag so they stay
// testable from any development platform; output_darwin.go only runs them.

// desktopprListsTarget reports whether desktoppr's read-back output (one
// picture path per screen, in screen order) lists target for the screen at
// index. verifiable is false when that screen is missing from the output.
func desktopprListsTarget(raw []byte, index int, target string) (ok, verifiable bool) {
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if index < 0 || index >= len(lines) {
		return false, true
	}
	// Match on the unique timestamped basename: macOS may report a
	// normalised variant of the full path.
	return strings.Contains(lines[index], filepath.Base(target)), true
}

// osascriptSetByNameScript sets target on every System Events desktop whose
// display name matches displayName (or on the only desktop when there is just
// one) and returns the number of desktops it changed.
func osascriptSetByNameScript(target, displayName string) string {
	return fmt.Sprintf(`tell application "System Events"
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
end tell`, appleScriptQuote(target), appleScriptQuote(displayName), appleScriptQuote(target))
}

// osascriptSetByOrdinalScript sets target on the System Events desktop at the
// given 1-based ordinal.
func osascriptSetByOrdinalScript(ordinal int, target string) string {
	return fmt.Sprintf(
		`tell application "System Events" to set picture of desktop %d to POSIX file %s`,
		ordinal, appleScriptQuote(target))
}

func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
