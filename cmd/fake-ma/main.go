// fake-ma simulates the visualizer relay of Music Assistant (the MilkDrop
// Visualizer plugin, providers/milkdrop_visualizer in MA 2.10), so zigbeat's
// beat source can be tested without Music Assistant.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type track struct {
	bpm    float64
	length time.Duration
}

func main() {
	listen := flag.String("listen", "127.0.0.1:8095", "address of the webserver")
	token := flag.String("token", "fake-token", "the token it accepts")
	player := flag.String("player", "", "when set, the only player id")
	tracks := flag.String("tracks", "124:60s", "tempo:length of each track, played back to back")
	loop := flag.Bool("loop", false, "play the list again; then the analysis is cached")
	analysis := flag.Duration("analysis", 6*time.Second, "on a first play the beats arrive this long into the track")
	pickup := flag.Int("pickup", 1, "beats before the first downbeat")
	seek := flag.Duration("seek", 0, "when set, this far into each first play the track jumps 10 s ahead")
	gap := flag.Duration("gap", 0, "silence between tracks")
	offset := flag.Duration("clock-offset", 37*time.Second, "added to the fake's monotonic clock")
	logBeats := flag.Bool("log", false, "print every beat with the wall time it sounds")
	flag.Parse()

	list, err := parseTracks(*tracks)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	tl := newTimeline(*offset, *logBeats)
	go tl.play(list, *loop, *analysis, *pickup, *seek, *gap)
	http.HandleFunc("/milkdrop_visualizer", func(w http.ResponseWriter, r *http.Request) { tl.serve(w, r, *token, *player) })
	fmt.Printf("fake-ma ready on ws://%s, tracks %s\n", *listen, *tracks)
	if err := http.ListenAndServe(*listen, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func parseTracks(s string) ([]track, error) {
	var out []track
	for _, part := range strings.Split(s, ",") {
		bpm, length, ok := strings.Cut(part, ":")
		b, err1 := strconv.ParseFloat(bpm, 64)
		l, err2 := time.ParseDuration(length)
		if !ok || err1 != nil || err2 != nil || b <= 0 || l <= 0 {
			return nil, fmt.Errorf("-tracks: %q is not tempo:length, such as 124:60s", part)
		}
		out = append(out, track{b, l})
	}
	return out, nil
}
