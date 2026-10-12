package engine

import (
	"testing"

	"github.com/siemenvdn/zigbeat/internal/music"
)

func TestTrackSetsTheTempo(t *testing.T) {
	e := started(t, "separate", 100)
	e.Music(t0, music.Track{BPM: 60})
	e.Music(t0, music.Beat{At: at(4000), Index: 4})
	q := e.queue["lamp-1"]
	if len(q) != 2 || !q[1].Land.Equal(at(5000)) {
		t.Errorf("at 60 BPM the kick of lamp-1 returns a second later: %+v", q)
	}
}
