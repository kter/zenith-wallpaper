package main

import "encoding/json"

// parseSwayOutputs extracts active outputs from the JSON emitted by
// `swaymsg -t get_outputs`. It lives outside the linux build tag so the
// parsing of captured payloads is testable from any development platform.
func parseSwayOutputs(raw []byte) ([]Output, error) {
	var nodes []struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
		Rect   struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"rect"`
		CurrentMode struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"current_mode"`
		Transform string `json:"transform"`
	}
	if err := json.Unmarshal(raw, &nodes); err != nil {
		return nil, err
	}
	var outputs []Output
	for _, r := range nodes {
		if !r.Active {
			continue
		}
		o := Output{
			Name:  r.Name,
			Index: len(outputs),
		}
		// Use physical resolution (current_mode) so the image matches
		// what swaybg displays at 1:1 pixels on HiDPI screens.
		// rect gives logical pixels which are smaller on scaled displays.
		o.Width = r.CurrentMode.Width
		o.Height = r.CurrentMode.Height
		// For 90/270-degree rotated outputs, swap physical dimensions.
		switch r.Transform {
		case "90", "270", "flipped-90", "flipped-270":
			o.Width, o.Height = o.Height, o.Width
		}
		if o.Width > 0 && o.Height > 0 {
			outputs = append(outputs, o)
		}
	}
	return outputs, nil
}
