package main

// Output describes a connected display to render a wallpaper for.
type Output struct {
	Name   string // unique label, also used for the cache filename
	Index  int    // 0-based display ordinal (desktoppr index on macOS)
	Width  int    // physical pixels
	Height int

	// displayName is the System Events "display name" used for AppleScript
	// matching on macOS. Unlike Name it is not uniquified. Empty on Linux.
	displayName string
}
