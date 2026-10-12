package main

import (
	"math"
	"math/rand/v2"
	"testing"
)

// TestSynthSections measures the level of each section, in dB of full scale.
func TestSynthSections(t *testing.T) {
	tr := track{bpm: 124}
	noise := rand.New(rand.NewPCG(1, 2))
	sum, n := map[section]float64{}, map[section]int{}
	for first := int64(0); first < 30*sampleRate; first += waveSamples {
		for i, s := range synth(tr, 1, first, noise) {
			v := (float64(s) - silentSample) / fullScale
			sec := sectionAt(tr, 1, float64(first+int64(i))/sampleRate)
			sum[sec] += v * v
			n[sec]++
		}
	}
	db := func(s section) float64 { return 10 * math.Log10(sum[s]/float64(n[s])+1e-12) }
	t.Logf("intro %.1f dB, drop %.1f dB, breakdown %.1f dB", db(intro), db(drop), db(breakdown))
	if db(drop)-db(intro) < 6 || sum[breakdown] != 0 {
		t.Errorf("intro %.1f, drop %.1f, breakdown %.1f dB", db(intro), db(drop), db(breakdown))
	}
}
