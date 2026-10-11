package probe

import (
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/report"
)

func (s *session) interrupted() bool {
	if s.ctx.Err() == nil {
		return false
	}
	s.intOnce.Do(func() {
		s.stopSignals()
		fmt.Println("\ninterrupted (Ctrl-C again to quit without restoring)")
	})
	return true
}

func (s *session) connLost() bool {
	if !s.client.Closed() {
		return false
	}
	s.lostOnce.Do(func() {
		fmt.Fprintf(os.Stderr, "\nmqtt connection lost: %v\nrestore by hand from: %s\n", s.client.Err(), s.snapJSON)
	})
	return true
}

// finish stops the background work, prints the summary and writes the raw samples.
func (s *session) finish(results []measure.Step, base measure.Stats) {
	s.stopBg()
	if len(results) > 0 {
		report.Summary(os.Stdout, results, base, s.cfg.canary)
	}
	samples, logs := s.rec.Snapshot()
	if err := report.WriteCSV(s.cfg.out, samples, logs); err != nil {
		fmt.Fprintf(os.Stderr, "csv: %v\n", err)
	} else {
		fmt.Printf("raw samples: %s\n", s.cfg.out)
	}
}
