package report

import (
	"encoding/csv"
	"os"
	"strconv"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/measure"
)

// WriteCSV writes every sample and log line to path.
func WriteCSV(path string, samples []measure.Sample, logs []measure.LogEvent) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"sent", "phase", "rate", "kind", "device", "latency_ms", "result"})
	for _, s := range samples {
		res := "ok"
		lat := strconv.FormatFloat(float64(s.Latency.Microseconds())/1000, 'f', 1, 64)
		if !s.OK {
			res, lat = "timeout", ""
		}
		_ = w.Write([]string{s.Sent.Format(time.RFC3339Nano), s.Phase, strconv.FormatFloat(s.Rate, 'g', -1, 64), s.Kind, s.Device, lat, res})
	}
	for _, l := range logs {
		_ = w.Write([]string{l.At.Format(time.RFC3339Nano), "z2m-log", "", l.Level, "", "", l.Msg})
	}
	w.Flush()
	return w.Error()
}
