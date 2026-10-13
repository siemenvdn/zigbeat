// Package ma is the Music Assistant source.
package ma

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	refusedEvery = 60 * time.Second

	msgBuffer = 64
)

// Source delivers the tracks and beats that Music Assistant's analysis found in what it plays.
type Source struct {
	URL       string
	Token     string
	Player    string
	Lookahead time.Duration
	Notes     io.Writer
}

func (s Source) String() string {
	who := "any playing player"
	if s.Player != "" {
		who = "player " + s.Player
	}
	return fmt.Sprintf("Music Assistant at %s (%s)", s.URL, who)
}

func (s Source) lead() time.Duration {
	if s.Lookahead > 0 {
		return s.Lookahead
	}
	return music.Lookahead
}

// Events connects to MA and keeps connecting until ctx is done.
func (s Source) Events(ctx context.Context) <-chan music.Event {
	out := make(chan music.Event)
	go func() {
		defer close(out)
		n := newNotes(s.Notes)
		for {
			wait := s.session(ctx, out, n)
			if ctx.Err() != nil {
				return
			}
			if !cli.Sleep(ctx, wait) {
				return
			}
		}
	}()
	return out
}

func (s Source) session(ctx context.Context, out chan<- music.Event, n *notes) time.Duration {
	clk := newClock()
	conn, err := s.dial()
	if err != nil {
		return n.failed(err)
	}
	defer conn.Close()
	done := make(chan struct{})
	defer close(done)
	msgs := make(chan message, msgBuffer)
	go read(conn, clk, msgs, done)
	ss := &session{src: s, conn: conn, out: out, n: n, clk: clk,
		m: newMeter(), emit: stopped(), ping: stopped(), report: stopped(), fade: stopped()}
	return ss.loop(ctx, msgs)
}

func stopped() *time.Timer {
	t := time.NewTimer(time.Hour)
	t.Stop()
	return t
}
