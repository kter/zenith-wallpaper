package main

import "testing"

func TestParseDimensions(t *testing.T) {
	tests := []struct {
		in     string
		w, h   int
		ok     bool
	}{
		{"3840 x 2160", 3840, 2160, true},
		{"2160 x 3840", 2160, 3840, true},
		{"1920 x 1080 @ 60.00Hz", 1920, 1080, true},
		{"3024 x 1964 Retina", 3024, 1964, true},
		{"", 0, 0, false},
		{"spdisplays_supported", 0, 0, false},
		{"1920", 0, 0, false},
	}
	for _, tt := range tests {
		w, h, ok := parseDimensions(tt.in)
		if w != tt.w || h != tt.h || ok != tt.ok {
			t.Errorf("parseDimensions(%q) = (%d, %d, %v), want (%d, %d, %v)",
				tt.in, w, h, ok, tt.w, tt.h, tt.ok)
		}
	}
}

// TestParseSystemProfilerPortrait is the regression test for the double
// rotation bug: _spdisplays_pixels already reports the rotated framebuffer,
// so a portrait display must come through with h > w, not swapped back to
// landscape. The numeric spdisplays_rotation must also not break unmarshal.
func TestParseSystemProfilerPortrait(t *testing.T) {
	raw := []byte(`{
	  "SPDisplaysDataType": [
	    {
	      "_name": "Apple M2",
	      "spdisplays_ndrvs": [
	        {
	          "_name": "DELL U2720Q",
	          "_spdisplays_pixels": "2160 x 3840",
	          "_spdisplays_resolution": "2160 x 3840 @ 60.00Hz",
	          "spdisplays_rotation": 90
	        }
	      ]
	    }
	  ]
	}`)
	outputs, err := parseSystemProfilerOutputs(raw)
	if err != nil {
		t.Fatalf("parseSystemProfilerOutputs: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("got %d outputs, want 1", len(outputs))
	}
	o := outputs[0]
	if o.Width != 2160 || o.Height != 3840 {
		t.Errorf("portrait display = %dx%d, want 2160x3840 (double-rotated back to landscape?)",
			o.Width, o.Height)
	}
}

func TestParseSystemProfilerMultiDisplay(t *testing.T) {
	raw := []byte(`{
	  "SPDisplaysDataType": [
	    {
	      "_name": "Apple M2",
	      "spdisplays_ndrvs": [
	        {
	          "_name": "Built-in Liquid Retina Display",
	          "_spdisplays_pixels": "3024 x 1964",
	          "_spdisplays_resolution": "1512 x 982 @ 60.00Hz",
	          "spdisplays_rotation": "spdisplays_supported"
	        },
	        {
	          "_name": "DELL U2720Q",
	          "_spdisplays_pixels": "3840 x 2160",
	          "_spdisplays_resolution": "1920 x 1080 @ 60.00Hz",
	          "spdisplays_rotation": "spdisplays_supported"
	        },
	        {
	          "_name": "DELL U2720Q",
	          "_spdisplays_pixels": "2160 x 3840",
	          "_spdisplays_resolution": "1080 x 1920 @ 60.00Hz",
	          "spdisplays_rotation": 270
	        }
	      ]
	    }
	  ]
	}`)
	outputs, err := parseSystemProfilerOutputs(raw)
	if err != nil {
		t.Fatalf("parseSystemProfilerOutputs: %v", err)
	}
	if len(outputs) != 3 {
		t.Fatalf("got %d outputs, want 3", len(outputs))
	}

	// Retina display must use physical pixels, not the "looks like" points.
	if outputs[0].Width != 3024 || outputs[0].Height != 1964 {
		t.Errorf("built-in = %dx%d, want 3024x1964 (points instead of pixels?)",
			outputs[0].Width, outputs[0].Height)
	}

	// Duplicate names are uniquified for cache filenames, but displayName
	// keeps the System Events name for AppleScript matching.
	if outputs[1].Name != "DELL U2720Q" || outputs[2].Name != "DELL U2720Q 2" {
		t.Errorf("names = %q, %q, want \"DELL U2720Q\", \"DELL U2720Q 2\"",
			outputs[1].Name, outputs[2].Name)
	}
	if outputs[2].displayName != "DELL U2720Q" {
		t.Errorf("displayName = %q, want un-uniquified \"DELL U2720Q\"", outputs[2].displayName)
	}

	// Index must be the 0-based ordinal used by desktoppr.
	for i, o := range outputs {
		if o.Index != i {
			t.Errorf("outputs[%d].Index = %d, want %d", i, o.Index, i)
		}
	}
}

func TestParseSystemProfilerFallbackAndSkips(t *testing.T) {
	raw := []byte(`{
	  "SPDisplaysDataType": [
	    {
	      "_name": "GPU",
	      "spdisplays_ndrvs": [
	        {
	          "_name": "NoPixelsDisplay",
	          "_spdisplays_resolution": "1920 x 1080 @ 60.00Hz"
	        },
	        {
	          "_name": "BrokenDisplay",
	          "_spdisplays_pixels": "garbage",
	          "_spdisplays_resolution": "also garbage"
	        },
	        {
	          "_name": "ZeroDisplay",
	          "_spdisplays_pixels": "0 x 0"
	        }
	      ]
	    }
	  ]
	}`)
	outputs, err := parseSystemProfilerOutputs(raw)
	if err != nil {
		t.Fatalf("parseSystemProfilerOutputs: %v", err)
	}
	if len(outputs) != 1 {
		t.Fatalf("got %d outputs, want 1 (fallback display only)", len(outputs))
	}
	if outputs[0].Name != "NoPixelsDisplay" || outputs[0].Width != 1920 || outputs[0].Height != 1080 {
		t.Errorf("fallback output = %+v, want NoPixelsDisplay 1920x1080", outputs[0])
	}
}

func TestParseSystemProfilerInvalidJSON(t *testing.T) {
	if _, err := parseSystemProfilerOutputs([]byte("not json")); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestParseSystemProfilerNoDisplays(t *testing.T) {
	outputs, err := parseSystemProfilerOutputs([]byte(`{"SPDisplaysDataType": []}`))
	if err != nil {
		t.Fatalf("parseSystemProfilerOutputs: %v", err)
	}
	if len(outputs) != 0 {
		t.Errorf("got %d outputs, want 0", len(outputs))
	}
}
