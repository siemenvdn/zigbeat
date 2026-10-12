// Package ma is the Music Assistant source: it reads what MA plays, its beats
// and their tracks, from the visualizer relay on MA's webserver.
package ma

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/siemenvdn/zigbeat/internal/music"
)

const (
	// refusedEvery waits for a fixed token far longer than cli.RedialEvery, which
	// follows a lost connection or no player.
	refusedEvery = 60 * time.Second
	// msgBuffer holds the burst of beats MA sends right after connecting.
	msgBuffer = 64
)

// Source delivers the tracks and beats that Music Assistant's analysis found in what it plays.
type Source struct {
	URL       string        // ws://host:port of MA's webserver
	Token     string        // a long-lived MA token
	Player    string        // MA player id; empty follows whichever player is playing
	Lookahead time.Duration // how long before At each beat is delivered; 0 means music.Lookahead
	Notes     io.Writer     // where changes are reported; nil means stdout
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

// Events connects to MA and keeps connecting until ctx is done; only then is the channel closed.
func (s Source) Events(ctx context.Context) <-chan music.Event {
	out := make(chan music.Event)
	go func() {
		defer close(out)
		n := newNotes(s.Notes)
		for {
			wait := s.session(ctx, out, n)
			if ctx.Err() != nil {
				return // a wait of 0 would race ctx and dial once more
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
	return out
}

// session runs one connection until it fails or ctx is done, and returns how
// long to wait before dialling again.
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

// stopped is a timer that waits for its first Reset; any duration works here.
func stopped() *time.Timer {
	t := time.NewTimer(time.Hour)
	t.Stop()
	return t
}
