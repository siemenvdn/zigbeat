package ma

import (
	"context"
	"time"
)

// minLeadPct of the lead must be left when a beat is sent; a later one came
// too late to plan like the rest, and is numbered but not sent.
const minLeadPct = 90

// arm sets the timer for the next beat, one lead before it sounds.
func (s *session) arm() {
	f, ok := s.g.peek()
	if !s.synced || !ok {
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
		if lead < s.src.lead()*minLeadPct/100 {
			continue
		}
		select {
		case s.out <- b:
		case <-ctx.Done():
			return false
		}
	}
	s.arm()
	return true
}
