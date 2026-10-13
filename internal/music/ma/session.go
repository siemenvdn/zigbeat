package ma

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	reportAfter = 200 * time.Millisecond

	noBeatsHint = 30 * time.Second
)

type session struct {
	src  Source
	conn *ws.Conn
	out  chan<- music.Event
	n    *notes
	clk  *clock
	g    grid
	m    *meter

	synced, announced  bool
	pings              int
	waveSince          time.Time
	beatSeen, hinted   bool
	emit, ping, report *time.Timer
	fade               *time.Timer
	palette            []music.RGB
	sent               *music.Track
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
			if !s.track(ctx) {
				return 0
			}
			s.arm()
		case <-s.fade.C:
			s.palette = nil
			if !s.track(ctx) {
				return 0
			}
		case <-s.emit.C:
			if !s.deliver(ctx) {
				return 0
			}
		case m := <-msgs:
			if wait, done := s.handle(ctx, m); done {
				return wait
			}
		}
	}
}

func (s *session) handle(ctx context.Context, m message) (wait time.Duration, done bool) {
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
			} else if !s.track(ctx) {
				return 0, true
			}
			s.arm()
		}
	case kindWave:
		s.m.add(m.f.ts, m.samples)
		if s.waveSince.IsZero() {
			s.waveSince = time.Now()
		}
		if !s.beatSeen && !s.hinted && time.Since(s.waveSince) > noBeatsHint {
			s.hinted = true
			s.n.say("audio but no beats after %s; is the Smart Fades provider enabled?", cli.Seconds(noBeatsHint))
		}
	case kindColor:
		if !s.album(ctx, m.palette) {
			return 0, true
		}
	case kindClear:
		if !s.forget(ctx, "new track or seek, waiting for beats") {
			return 0, true
		}
	case kindEnd:
		if !s.forget(ctx, "music stopped") {
			return 0, true
		}
		if s.src.Player == "" {
			return cli.RedialEvery, true
		}
	}
	return 0, false
}

func (s *session) forget(ctx context.Context, note string) bool {
	delivered := s.g.started
	s.g.clear()
	s.m.reset()
	s.emit.Stop()
	s.report.Stop()
	s.announced, s.beatSeen, s.hinted, s.waveSince, s.sent = false, false, false, time.Time{}, nil
	s.n.say("%s", note)
	return !delivered || s.send(ctx, music.Clear{})
}
