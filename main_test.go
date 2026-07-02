package main

import "testing"

func TestSanitize(t *testing.T) {
	tests := []struct{ in, want string }{
		{"DP-1", "DP-1"},
		{"DELL U2720Q 2", "DELL_U2720Q_2"},
		{"a/b\\c:d", "a_b_c_d"},
		{"", ""},
		{"Built-in Liquid Retina Display", "Built-in_Liquid_Retina_Display"},
	}
	for _, tt := range tests {
		if got := sanitize(tt.in); got != tt.want {
			t.Errorf("sanitize(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
