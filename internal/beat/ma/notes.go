package ma

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

const tokenRefused = 4001 // MA's close code for a bad token

// notes reports what the source does, once per change.
type notes struct {
	w    io.Writer
	last string
}

func newNotes(w io.Writer) *notes {
	if w == nil {
		w = os.Stdout
	}
	return &notes{w: w}
}

func (n *notes) say(format string, args ...any) {
	s := fmt.Sprintf(format, args...)
	if s == n.last {
		return
	}
	n.last = s
	fmt.Fprintf(n.w, "%s  music assistant: %s\n", cli.Stamp(), s)
}

// failed reports a connection that did not work and returns how long to wait.
func (n *notes) failed(err error) time.Duration {
	var ce *ws.CloseError
	switch {
	case errors.As(err, &ce) && ce.Code == tokenRefused:
		n.say("token refused (%s); check the token in music_assistant.token_env, trying again every 60 s", ce.Reason)
		return refusedEvery
	case strings.Contains(err.Error(), "404"):
		n.say("no visualizer relay (%v); is the MilkDrop Visualizer plugin enabled? Trying again every 5 s", err)
	default:
		n.say("connection lost (%v), reconnecting every 5 s", err)
	}
	return retryEvery
}

func (n *notes) noPlayer(player string) {
	if player == "" {
		n.say("nothing is playing, waiting")
		return
	}
	n.say("no player %s in Music Assistant; check music_assistant.player", player)
}
