package main

import (
	"encoding/json"
	"fmt"
)

// parseSystemProfilerOutputs extracts displays from the JSON emitted by
// `system_profiler SPDisplaysDataType -json`. It lives outside the darwin
// build tag so the parsing of captured payloads is testable from any
// development platform.
//
// _spdisplays_pixels already reflects the rotated framebuffer (a portrait
// display reports h > w), so no width/height swap on spdisplays_rotation —
// swapping would double-rotate and yield a landscape image for a portrait
// screen. spdisplays_rotation is also type-unstable ("spdisplays_supported"
// when upright, a bare number when rotated), so it is deliberately not
// unmarshalled at all.
func parseSystemProfilerOutputs(raw []byte) ([]Output, error) {
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
