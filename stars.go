package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"strconv"
	"strings"
)

//go:embed data/bsc.csv
var bscCSV []byte

// Star holds a catalogue entry.
type Star struct {
	RA, Dec float64 // J2000 degrees
	Mag     float64
}

// LoadStars parses the embedded BSC CSV (ra_deg,dec_deg,vmag).
func LoadStars(magLimit float64) ([]Star, error) {
	var stars []Star
	scanner := bufio.NewScanner(bytes.NewReader(bscCSV))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 3 {
			continue
		}
		ra, err1 := strconv.ParseFloat(parts[0], 64)
		dec, err2 := strconv.ParseFloat(parts[1], 64)
		mag, err3 := strconv.ParseFloat(parts[2], 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue
		}
		if mag > magLimit {
			continue
		}
		stars = append(stars, Star{RA: ra, Dec: dec, Mag: mag})
	}
	return stars, scanner.Err()
}
