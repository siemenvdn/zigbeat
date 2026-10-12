package serve

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

// choose takes a style command from Home Assistant, on the MQTT reader: it
// checks the value like the config, keeps it for the next show and hands it to
// a running one. A kept choice that comes again after a reconnect changes
// nothing and prints nothing. Every command gets the state back.
func (l *link) choose(topic string, p []byte, _ time.Time) {
	key := ha.StyleKey(topic)
	l.mu.Lock()
	next, err := ha.Choose(l.style, l.base, key, string(p))
	changed := err == nil && ha.StyleOf(next) != ha.StyleOf(l.style)
	if changed {
		l.style = next
	}
	l.mu.Unlock()
	switch {
	case err != nil:
		fmt.Printf("%s  refused from Home Assistant: %s %q: %v\n", cli.Stamp(), key, p, err)
	case changed:
		how := "set in Home Assistant"
		if len(p) == 0 {
			how = "from the config again"
		}
		fmt.Printf("%s  %s %v, %s\n", cli.Stamp(), key, ha.Value(next, key), how)
		newest(l.styles, next)
	}
	l.signal()
}

// effects is the style in force: the config's, with the choices over it.
func (l *link) effects() config.Effects {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.style
}
