package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// uniqueWallpaperCopy duplicates abs next to itself under a timestamped name
// (e.g. "Built-in_Display.1718064000123456789.png") and removes stale copies
// from earlier runs of the same output. It supports the macOS SetWallpaper
// workaround but is kept outside the darwin build tag so it stays testable
// from any development platform.
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
