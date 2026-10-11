package engine

import (
	"fmt"
	"time"

	"github.com/siemenvdn/zigbeat/internal/run/flow"
)

func (e *Engine) raise(now time.Time, max int, why string) {
	if !e.bo.Signal(now, max) {
		return
	}
	if e.bo.Level() == flow.PauseLevel {
		e.paused(now, why)
		return
	}
	e.note(now, fmt.Sprintf("backoff %d: %s", e.bo.Level(), why))
}

func (e *Engine) paused(now time.Time, why string) {
	clear(e.queue)
	e.fl.Forget()
	e.note(now, "paused: "+why+"; the show resumes after a quiet minute")
}
