package serve

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/run/runtest"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

func TestStyleCommands(t *testing.T) {
	l := newLink(runtest.Config("separate"), "test")
	for _, c := range []struct {
		key, payload string
		brightness   int  // in force afterwards
		toShow       bool // handed to the show
	}{
		{"brightness", "40", 40, true},
		{"brightness", "40", 40, false},  // a kept choice again, after a reconnect
		{"brightness", "150", 40, false}, // refused
		{"speed", "3", 40, false},        // no such setting
		{"brightness", "", 60, true},     // wiped: the config's again
		{"brightness", "", 60, false},
	} {
		l.choose(ha.StyleTopic(c.key), []byte(c.payload), time.Time{})
		if got := l.effects().Brightness; got != c.brightness {
			t.Errorf("%s %q: brightness %d, want %d", c.key, c.payload, got, c.brightness)
		}
		select {
		case e := <-l.styles:
			if !c.toShow || e.Brightness != c.brightness {
				t.Errorf("%s %q: the show got brightness %d", c.key, c.payload, e.Brightness)
			}
		default:
			if c.toShow {
				t.Errorf("%s %q: the show got nothing", c.key, c.payload)
			}
		}
		select {
		case <-l.wake:
		default:
			t.Errorf("%s %q: the state is not published again", c.key, c.payload)
		}
	}
	if s := string(l.current()); !strings.Contains(s, `"brightness":60`) || !strings.Contains(s, `"status":"off"`) {
		t.Errorf("state %s", s)
	}
}

func TestShowGetsTheStyles(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	if got := <-h.show.effects; got != (<-chan config.Effects)(h.styles) {
		t.Error("the show did not get the styles from Home Assistant")
	}
}
