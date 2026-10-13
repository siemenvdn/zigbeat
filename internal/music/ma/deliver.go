package ma

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

// minLeadPct of the lead must be left when a beat is sent; a later one came
// too late to plan like the rest, and is numbered but not sent.
const minLeadPct = 90

// bar is the length of a bar at the track's tempo, in µs on MA's clock.
func (s *session) bar() int64 {
	return music.PerBar * music.Track{BPM: s.g.bpm()}.Period().Microseconds()
}

// arm sets the timer for the next beat, one lead before it sounds. Nothing is
// delivered before the burst is complete: a beat numbered earlier could miss
// its downbeat.
func (s *session) arm() {
	f, ok := s.g.peek()
	if !s.synced || !s.announced || !ok {
		s.emit.Stop()
		return
	}
	at, _ := s.clk.at(f.ts)
	s.emit.Reset(time.Until(at) - s.src.lead())
}

// deliver sends every beat that is due. A beat that became known too late is
// numbered but not sent. It returns false when ctx ended the wait.
func (s *session) deliver(ctx context.Context) bool {
	for {
		f, ok := s.g.peek()
		if !ok {
			break
		}
		at, _ := s.clk.at(f.ts)
		lead := time.Until(at)
		if lead > s.src.lead() {
			break
		}
		b := s.g.deliver(at)
		b.Energy = s.m.energy(f.ts, s.bar())
		if lead < s.src.lead()*minLeadPct/100 {
			continue
		}
		if !s.send(ctx, b) {
			return false
		}
	}
	s.arm()
	return true
}

// send hands ev to the show. It returns false when ctx ended the wait.
func (s *session) send(ctx context.Context, ev music.Event) bool {
	select {
	case s.out <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}
