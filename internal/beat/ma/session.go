package ma

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/beat"
	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	reportAfter = 200 * time.Millisecond // a burst of beats is reported once it is quiet this long
	// noBeatsHint is well past MA's analysis of a first play: beats are not coming.
	noBeatsHint = 30 * time.Second
)

// session is one connection to the relay. Only its loop goroutine touches it.
type session struct {
	src  Source
	conn *ws.Conn
	out  chan<- beat.Beat
	n    *notes
	clk  *clock
	g    grid

	synced, announced  bool // announced: the track's burst of beats is complete and reported
	pings              int
	waveSince          time.Time // first audio since the last clear without a beat
	beatSeen, hinted   bool
	emit, ping, report *time.Timer
}

func (s *session) loop(ctx context.Context, msgs <-chan message) time.Duration {
	burst := time.NewTicker(burstEvery)
	defer burst.Stop()
	s.startBurst()
	for {
		select {
		case <-ctx.Done():
			return 0
		case <-burst.C:
			s.startBurst()
		case <-s.ping.C:
			s.sendPing()
		case <-s.report.C:
			beats, downs := s.g.ahead()
			s.n.say("%d beats ahead, %.1f BPM, %d downbeats", beats, s.g.bpm(), downs)
			s.announced = true
			s.arm() // the burst is complete, so the first beat can be numbered from its downbeat
		case <-s.emit.C:
			if !s.deliver(ctx) {
				return 0
			}
		case m := <-msgs:
			if wait, done := s.handle(m); done {
				return wait
			}
		}
	}
}

// handle reacts to one message; done means the session is over.
func (s *session) handle(m message) (wait time.Duration, done bool) {
	switch m.kind {
	case kindFailed:
		return s.n.failed(m.err), true
	case kindError:
		s.n.noPlayer(s.src.Player)
		return cli.RedialEvery, true
	case kindTime:
		s.clk.add(m.t1, m.t2, m.t3, m.t4)
		if !s.synced {
			e, _ := s.clk.best()
			s.synced = true
			s.n.say("connected, clock round trip %.1f ms", float64(e.rtt)/1000)
			s.arm()
		}
	case kindBeat:
		s.beatSeen = true
		if s.g.add(m.f) {
			if !s.announced {
				s.report.Reset(reportAfter)
			}
			s.arm()
		}
	case kindWave:
		if s.waveSince.IsZero() {
			s.waveSince = time.Now()
		}
		if !s.beatSeen && !s.hinted && time.Since(s.waveSince) > noBeatsHint {
			s.hinted = true
			s.n.say("audio but no beats after %s; is the Smart Fades provider enabled?", cli.Seconds(noBeatsHint))
		}
	case kindClear:
		s.forget("new track or seek, waiting for beats")
	case kindEnd:
		s.forget("music stopped")
		if s.src.Player == "" {
			return cli.RedialEvery, true // MA picks whichever player plays when the source dials again
		}
	}
	return 0, false
}

// forget drops the track: its beats no longer apply.
func (s *session) forget(note string) {
	s.g.clear()
	s.emit.Stop()
	s.report.Stop()
	s.announced, s.beatSeen, s.hinted, s.waveSince = false, false, false, time.Time{}
	s.n.say("%s", note)
}
