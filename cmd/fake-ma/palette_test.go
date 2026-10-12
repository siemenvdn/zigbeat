package main

import "testing"

func TestColorMessage(t *testing.T) {
	want := `{"type":"color","payload":{"background_dark":[20,28,56],"background_light":[200,205,220],` +
		`"primary":[32,45,89],"accent":[219,128,52],"on_dark":[240,220,200],"on_light":[60,30,20]}}`
	if got := string(colorMessage(palettes[0])); got != want {
		t.Errorf("navy cover:\n%s\nwant\n%s", got, want)
	}
	none := `{"type":"color","payload":{"background_dark":null,"background_light":null,"primary":null,"accent":null,"on_dark":null,"on_light":null}}`
	if got := string(colorMessage(noArt)); got != none {
		t.Errorf("no artwork: %s", got)
	}
	if palettes[1] != noArt || len(palettes) != 3 {
		t.Errorf("the second track has no artwork, and three palettes go round: %v", palettes)
	}
}
