package main

import "testing"

func TestParseSwayOutputs(t *testing.T) {
	raw := []byte(`[
	  {
	    "name": "DP-1",
	    "active": true,
	    "rect": {"width": 1920, "height": 1080},
	    "current_mode": {"width": 3840, "height": 2160},
	    "transform": "normal"
	  },
	  {
	    "name": "DP-2",
	    "active": false,
	    "rect": {"width": 1920, "height": 1080},
	    "current_mode": {"width": 1920, "height": 1080},
	    "transform": "normal"
	  },
	  {
	    "name": "HDMI-A-1",
	    "active": true,
	    "rect": {"width": 1080, "height": 1920},
	    "current_mode": {"width": 1920, "height": 1080},
	    "transform": "90"
	  }
	]`)
	outputs, err := parseSwayOutputs(raw)
	if err != nil {
		t.Fatalf("parseSwayOutputs: %v", err)
	}
	if len(outputs) != 2 {
		t.Fatalf("got %d outputs, want 2 (inactive filtered)", len(outputs))
	}

	// Physical pixels from current_mode, not logical pixels from rect.
	if outputs[0].Name != "DP-1" || outputs[0].Width != 3840 || outputs[0].Height != 2160 {
		t.Errorf("outputs[0] = %+v, want DP-1 3840x2160 (rect instead of current_mode?)", outputs[0])
	}

	// Sway reports current_mode unrotated, so 90° transforms must swap.
	if outputs[1].Name != "HDMI-A-1" || outputs[1].Width != 1080 || outputs[1].Height != 1920 {
		t.Errorf("outputs[1] = %+v, want HDMI-A-1 1080x1920 (transform swap missing?)", outputs[1])
	}

	// Index counts active outputs only.
	if outputs[0].Index != 0 || outputs[1].Index != 1 {
		t.Errorf("indices = %d, %d, want 0, 1", outputs[0].Index, outputs[1].Index)
	}
}

func TestParseSwayOutputsTransforms(t *testing.T) {
	tests := []struct {
		transform string
		w, h      int
	}{
		{"normal", 1920, 1080},
		{"180", 1920, 1080},
		{"flipped", 1920, 1080},
		{"90", 1080, 1920},
		{"270", 1080, 1920},
		{"flipped-90", 1080, 1920},
		{"flipped-270", 1080, 1920},
	}
	for _, tt := range tests {
		raw := []byte(`[{"name": "X", "active": true,
			"current_mode": {"width": 1920, "height": 1080},
			"transform": "` + tt.transform + `"}]`)
		outputs, err := parseSwayOutputs(raw)
		if err != nil {
			t.Fatalf("transform %q: %v", tt.transform, err)
		}
		if len(outputs) != 1 || outputs[0].Width != tt.w || outputs[0].Height != tt.h {
			t.Errorf("transform %q = %dx%d, want %dx%d",
				tt.transform, outputs[0].Width, outputs[0].Height, tt.w, tt.h)
		}
	}
}

func TestParseSwayOutputsZeroDims(t *testing.T) {
	raw := []byte(`[{"name": "BAD", "active": true,
		"current_mode": {"width": 0, "height": 0}, "transform": "normal"}]`)
	outputs, err := parseSwayOutputs(raw)
	if err != nil {
		t.Fatalf("parseSwayOutputs: %v", err)
	}
	if len(outputs) != 0 {
		t.Errorf("got %d outputs, want 0 (zero-dim output must be skipped)", len(outputs))
	}
}

func TestParseSwayOutputsInvalidJSON(t *testing.T) {
	if _, err := parseSwayOutputs([]byte("nope")); err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}
