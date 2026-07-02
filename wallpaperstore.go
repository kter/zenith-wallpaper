package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

// spacesOverrideCount counts the per-Space wallpaper overrides in the macOS
// wallpaper store (`~/Library/Application Support/com.apple.wallpaper/Store/
// Index.plist`, converted to xml1 via plutil). Since Sonoma the wallpaper API
// only affects the currently visible Space of each display; a Space listed
// under the root "Spaces" dictionary keeps its own frozen wallpaper until the
// store is reset. The caller resets the store (delete + killall
// WallpaperAgent) exactly when this count is non-zero, so every Space falls
// back to the display-level configuration that zenith keeps updating.
//
// Kept outside the darwin build tag so the parsing is testable from any
// development platform.
func spacesOverrideCount(plistXML []byte) (int, error) {
	dec := xml.NewDecoder(bytes.NewReader(plistXML))
	depth := 0        // element nesting depth
	inRootDict := false
	rootDictDepth := 0
	var pendingKey string // last <key> text read at root-dict level
	keyText := false      // currently inside a root-level <key> element

	for {
		tok, err := dec.Token()
		if err != nil {
			return 0, fmt.Errorf("wallpaper store plist: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if !inRootDict && t.Name.Local == "dict" && depth == 2 {
				// The first <dict> directly under <plist> is the root dict.
				inRootDict = true
				rootDictDepth = depth
				continue
			}
			if inRootDict && depth == rootDictDepth+1 {
				if t.Name.Local == "key" {
					keyText = true
					pendingKey = ""
					continue
				}
				if pendingKey == "Spaces" {
					if t.Name.Local != "dict" {
						return 0, nil // unexpected value type; treat as none
					}
					return countImmediateKeys(dec), nil
				}
				pendingKey = ""
			}
		case xml.EndElement:
			if keyText && t.Name.Local == "key" {
				keyText = false
				depth--
				continue
			}
			depth--
			if inRootDict && depth < rootDictDepth {
				return 0, nil // root dict closed without a Spaces entry
			}
		case xml.CharData:
			if keyText {
				pendingKey += string(t)
			}
		}
	}
}

// countImmediateKeys consumes the decoder up to the end of the dict element
// just opened and returns how many <key> children sit directly inside it.
func countImmediateKeys(dec *xml.Decoder) int {
	depth := 1
	count := 0
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return count
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 1 && t.Name.Local == "key" {
				count++
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return count
}
