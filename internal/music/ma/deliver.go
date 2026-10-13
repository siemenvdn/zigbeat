package ma

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const minLeadPct = 90

func (s *session) bar() int64 {
	return (music.PerBar * music.Track{BPM: s.g.bpm()}.Period()).Microseconds()
}

func (s *session) arm() {
	f, ok := s.g.peek()
	if !s.synced || !s.announced || !ok {
		s.emit.Stop()
		return
	}
	at, _ := s.clk.at(f.ts)
	s.emit.Reset(time.Until(at) - s.src.lead())
}

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

func (s *session) send(ctx context.Context, ev music.Event) bool {
	select {
	case s.out <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}
