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

	"github.com/siemenvdn/zigbeat/internal/cli"
)

// Flag defaults.
const (
	defaultListen = "127.0.0.1:8095" // MA's webserver port
	defaultTracks = "124:60s"        // a minute at a common dance tempo
	analysisDelay = 6 * time.Second  // MA analyses a first play for seconds before beats come
	// clockOffset is far from zero, so a client that skips the clock sync is visibly off.
	clockOffset = 37 * time.Second
)

type track struct {
	bpm    float64
	length time.Duration
}

func main() {
	listen := flag.String("listen", defaultListen, "address of the webserver")
	token := flag.String("token", "fake-token", "the token it accepts")
	player := flag.String("player", "", "when set, the only player id")
	tracks := flag.String("tracks", defaultTracks, "tempo:length of each track, played back to back")
	loop := flag.Bool("loop", false, "play the list again; then the analysis is cached")
	analysis := flag.Duration("analysis", analysisDelay, "on a first play the beats arrive this long into the track")
	pickup := flag.Int("pickup", 1, "beats before the first downbeat")
	seek := flag.Duration("seek", 0, "when set, this far into each first play the track jumps "+cli.Seconds(seekJump)+" ahead")
	gap := flag.Duration("gap", 0, "silence between tracks")
	offset := flag.Duration("clock-offset", clockOffset, "added to the fake's monotonic clock")
	logBeats := flag.Bool("log", false, "print every beat with the wall time it sounds")
	flag.Parse()

	list, err := parseTracks(*tracks)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(cli.ExitUsage)
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
			return nil, fmt.Errorf("-tracks: %q is not tempo:length, such as %s", part, defaultTracks)
		}
		out = append(out, track{b, l})
	}
	return out, nil
}
