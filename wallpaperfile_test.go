package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUniqueWallpaperCopy(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "DP-1.png")
	content := []byte("fake png payload")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	// Stale copies from earlier runs, plus an unrelated output's file that
	// must survive pruning.
	stale := filepath.Join(dir, "DP-1.123456789.png")
	other := filepath.Join(dir, "DP-2.987654321.png")
	for _, p := range []string{stale, other} {
		if err := os.WriteFile(p, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	target, err := uniqueWallpaperCopy(src)
	if err != nil {
		t.Fatalf("uniqueWallpaperCopy: %v", err)
	}

	if filepath.Dir(target) != dir || !strings.HasPrefix(filepath.Base(target), "DP-1.") ||
		!strings.HasSuffix(target, ".png") || target == src {
		t.Errorf("target = %q, want timestamped DP-1.*.png next to source", target)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(content) {
		t.Errorf("target content = %q (%v), want source copy", got, err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source was removed: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale copy %q was not pruned", stale)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("unrelated output's copy was pruned: %v", err)
	}
}

func TestUniqueWallpaperCopyMissingSource(t *testing.T) {
	if _, err := uniqueWallpaperCopy(filepath.Join(t.TempDir(), "absent.png")); err == nil {
		t.Error("expected error for missing source file")
	}
}

func TestAppleScriptQuote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", `"plain"`},
		{`with "quotes"`, `"with \"quotes\""`},
		{`back\slash`, `"back\\slash"`},
		{`both "\"`, `"both \"\\\""`},
	}
	for _, tt := range tests {
		if got := appleScriptQuote(tt.in); got != tt.want {
			t.Errorf("appleScriptQuote(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
