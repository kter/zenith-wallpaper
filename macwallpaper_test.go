package main

import (
	"strings"
	"testing"
)

func TestDesktopprListsTarget(t *testing.T) {
	raw := []byte("/Users/me/cache/Color_LCD.111.png\n/Users/me/cache/DELL.222.png\n")
	tests := []struct {
		name           string
		index          int
		target         string
		ok, verifiable bool
	}{
		{"match on first screen", 0, "/Users/me/cache/Color_LCD.111.png", true, true},
		{"match by basename despite different dir", 1, "/private/var/x/DELL.222.png", true, true},
		{"stale picture", 1, "/Users/me/cache/DELL.333.png", false, true},
		{"screen missing from output", 2, "/Users/me/cache/X.1.png", false, true},
		{"negative index", -1, "/Users/me/cache/X.1.png", false, true},
	}
	for _, tt := range tests {
		ok, verifiable := desktopprListsTarget(raw, tt.index, tt.target)
		if ok != tt.ok || verifiable != tt.verifiable {
			t.Errorf("%s: got (%v, %v), want (%v, %v)", tt.name, ok, verifiable, tt.ok, tt.verifiable)
		}
	}
}

func TestOsascriptSetByNameScript(t *testing.T) {
	s := osascriptSetByNameScript(`/tmp/a "b".png`, "Color LCD")
	for _, want := range []string{
		`set picture of every desktop to POSIX file "/tmp/a \"b\".png"`,
		`if display name of d is "Color LCD" then`,
		`return matched as text`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("script missing %q:\n%s", want, s)
		}
	}
}

func TestOsascriptSetByOrdinalScript(t *testing.T) {
	got := osascriptSetByOrdinalScript(2, "/tmp/a.png")
	want := `tell application "System Events" to set picture of desktop 2 to POSIX file "/tmp/a.png"`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
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
