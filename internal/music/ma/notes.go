package ma

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/ws"
)

const tokenRefused = 4001

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

func (n *notes) failed(err error) time.Duration {
	var ce *ws.CloseError
	var se *ws.StatusError
	switch {
	case errors.As(err, &ce) && ce.Code == tokenRefused:
		n.say("token refused (%s); check the token in music_assistant.token_env, trying again every %s", ce.Reason, cli.Seconds(refusedEvery))
		return refusedEvery
	case errors.As(err, &se) && se.Code == http.StatusNotFound:
		n.say("no visualizer relay (%v); is the MilkDrop Visualizer plugin enabled? Trying again every %s", err, cli.Seconds(cli.RedialEvery))
	default:
		n.say("connection lost (%v), reconnecting every %s", err, cli.Seconds(cli.RedialEvery))
	}
	return cli.RedialEvery
}

func (n *notes) noPlayer(player string) {
	if player == "" {
		n.say("nothing is playing, waiting")
		return
	}
	n.say("no player %s in Music Assistant; check music_assistant.player", player)
}
