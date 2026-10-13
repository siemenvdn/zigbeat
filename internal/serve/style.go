package serve

import (
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

func (l *haLink) choose(topic string, p []byte, _ time.Time) {
	key := ha.StyleKey(topic)
	l.mu.Lock()
	prev := l.style
	next, err := ha.Choose(prev, l.base, key, string(p))
	changed := err == nil && ha.StyleOf(next) != ha.StyleOf(prev)
	if changed {
		l.style = next
	}
	l.mu.Unlock()
	switch {
	case err != nil:
		l.say("refused from Home Assistant: %s %q: %v", key, p, err)
	case changed:
		how := "set in Home Assistant"
		if len(p) == 0 {
			how = "from the config again"
		}
		l.say("%s %v, %s", key, ha.Value(next, key), how)
		if next.Budget.AudioOffsetMS != prev.Budget.AudioOffsetMS {
			newest(l.offsets, next.Budget.AudioOffset())
		} else {
			newest(l.styles, next.Effects)
		}
	}
	l.signal()
}

func (l *haLink) config() config.Config {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.style
}
