package main

import "testing"

func TestLoadStarsEmbedded(t *testing.T) {
	stars, err := LoadStars(6.5)
	if err != nil {
		t.Fatalf("LoadStars: %v", err)
	}
	if len(stars) == 0 {
		t.Fatal("embedded catalogue yielded no stars")
	}
	for _, s := range stars {
		if s.RA < 0 || s.RA >= 360 {
			t.Fatalf("star RA = %v, want [0, 360)", s.RA)
		}
		if s.Dec < -90 || s.Dec > 90 {
			t.Fatalf("star Dec = %v, want [-90, 90]", s.Dec)
		}
		if s.Mag > 6.5 {
			t.Fatalf("star Mag = %v exceeds the 6.5 limit", s.Mag)
		}
	}

	bright, err := LoadStars(2.0)
	if err != nil {
		t.Fatalf("LoadStars(2.0): %v", err)
	}
	if len(bright) == 0 || len(bright) >= len(stars) {
		t.Errorf("magLimit filter: %d bright stars vs %d total", len(bright), len(stars))
	}
}

func TestLoadStarsParsing(t *testing.T) {
	orig := bscCSV
	t.Cleanup(func() { bscCSV = orig })
	bscCSV = []byte(
		"10.5,-20.25,1.5\n" +
			"\n" + // blank line skipped
			"30,40\n" + // too few fields skipped
			"50.0,60.0,7.0\n" + // above mag limit skipped
			"20,30,abc\n" + // parse error skipped
			"  350.25,89.9,6.5  \n") // surrounding whitespace trimmed

	stars, err := LoadStars(6.5)
	if err != nil {
		t.Fatalf("LoadStars: %v", err)
	}
	want := []Star{
		{RA: 10.5, Dec: -20.25, Mag: 1.5},
		{RA: 350.25, Dec: 89.9, Mag: 6.5},
	}
	if len(stars) != len(want) {
		t.Fatalf("got %d stars %v, want %d", len(stars), stars, len(want))
	}
	for i := range want {
		if stars[i] != want[i] {
			t.Errorf("stars[%d] = %+v, want %+v", i, stars[i], want[i])
		}
	}
}
