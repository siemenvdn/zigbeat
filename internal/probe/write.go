package probe

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/probe/measure"
	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/probe/report"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// saveBudget writes the budget of results into the config of -config, when
// there is one and a step ran clean. It returns false when writing failed.
func (s *session) saveBudget(results []measure.Step) bool {
	b, ok := report.Suggest(results)
	if !ok || s.cfg.config == "" {
		return true
	}
	rate, latency, err := writeBudget(s.cfg.config, b, s.cfg.mode, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not write the budget into %s (%v); set budget.max_rate: %g and budget.latency_ms: %d by hand\n",
			s.cfg.config, err, rate, latency.Milliseconds())
		return false
	}
	fmt.Printf("budget written to %s: max_rate %g -> %g frames/s, latency_ms %d -> %d ms\n",
		s.cfg.config, s.cfg.budget.MaxRate, rate, s.cfg.budget.LatencyMS, latency.Milliseconds())
	return true
}

// writeBudget puts b into the config at path in Zigbee frames, as the show
// counts them: a command of mode is that many frames. The comments say when
// and from what. It returns the values, also when writing failed.
func writeBudget(path string, b report.Budget, mode string, now time.Time) (rate float64, latency time.Duration, err error) {
	frames := z2m.Frames(palette.New(mode).Next("").Payload)
	rate = b.Rate * float64(frames)
	latency = b.Ref.Cmd.P50 / time.Duration(frames) // whole ms below, as the summary prints them
	f, err := config.OpenFile(path)
	if err != nil {
		return rate, latency, err
	}
	day := now.Format(time.DateOnly)
	err = f.Set("budget.max_rate", strconv.FormatFloat(rate, 'g', -1, 64),
		fmt.Sprintf("zigbeat probe %s: the clean rate of %g cmd/s in %s mode / %d", day, b.Clean.Rate, mode, report.Headroom))
	if err == nil {
		err = f.Set("budget.latency_ms", strconv.FormatInt(latency.Milliseconds(), 10),
			fmt.Sprintf("zigbeat probe %s: p50 per frame at %g cmd/s", day, b.Ref.Rate))
	}
	if err == nil {
		err = f.Save(func(data []byte) error {
			_, err := config.ParseDraft(data, os.Getenv)
			return err
		})
	}
	return rate, latency, err
}
