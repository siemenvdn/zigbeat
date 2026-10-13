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
	l := newHALink(runtest.Config("separate"), "test")
	for _, c := range []struct {
		key, payload string
		brightness   int
		toShow       bool
	}{
		{"brightness", "40", 40, true},
		{"brightness", "40", 40, false},
		{"brightness", "150", 40, false},
		{"speed", "3", 40, false},
		{"brightness", "", 60, true},
		{"brightness", "", 60, false},
	} {
		l.choose(ha.StyleTopic(c.key), []byte(c.payload), time.Time{})
		if got := l.config().Effects.Brightness; got != c.brightness {
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

// TestOffsetCommands.
func TestOffsetCommands(t *testing.T) {
	l := newHALink(runtest.Config("separate"), "test")
	l.choose(ha.StyleTopic("audio_offset_ms"), []byte("180"), time.Time{})
	select {
	case d := <-l.offsets:
		if d != 180*time.Millisecond {
			t.Errorf("the show got offset %v", d)
		}
	default:
		t.Error("the show got no offset")
	}
	select {
	case e := <-l.styles:
		t.Errorf("the show got a style: %+v", e)
	default:
	}
	if got := l.config().Budget.AudioOffsetMS; got != 180 {
		t.Errorf("the next show gets offset %d", got)
	}
	if s := string(l.current()); !strings.Contains(s, `"audio_offset_ms":180`) {
		t.Errorf("state %s", s)
	}
}

func TestShowGetsTheStyles(t *testing.T) {
	h := serveFake(t, 0)
	newest(h.cmds, true)
	h.expect(t, "starting", "running")
	got := <-h.show.hooks
	if got.Effects != (<-chan config.Effects)(h.styles) || got.Offsets != (<-chan time.Duration)(h.offsets) {
		t.Error("the show did not get the styles and offsets from Home Assistant")
	}
}
