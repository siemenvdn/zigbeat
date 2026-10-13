package probe

import (
	"strings"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/probe/palette"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

var t0 = time.Date(2026, 10, 13, 21, 0, 0, 0, time.UTC)

// cmds is n answered commands a second apart, with latencies from lat in turn.
func cmds(n int, lat ...time.Duration) []Sample {
	var out []Sample
	for i := range n {
		out = append(out, Sample{Sent: t0.Add(time.Duration(i) * time.Second), Kind: "cmd", Latency: lat[i%len(lat)], OK: true})
	}
	return out
}

func TestEvaluate(t *testing.T) {
	ms := time.Millisecond
	lim := Limits{CanaryP50: 40 * ms, CanaryP95: 50 * ms} // canary limit 350 ms
	steady := cmds(20, 60*ms)
	cases := []struct {
		name   string
		step   Step
		cmds   []Sample
		want   Verdict
		reason string
	}{
		{"clean", Step{Sent: 20}, steady, OK, ""},
		{"one lost", Step{Sent: 20, Cmd: Stats{N: 19, Lost: 1}}, steady, Warn, "1 commands unanswered"},
		{"many lost", Step{Sent: 20, Cmd: Stats{N: 18, Lost: 2}}, steady, Stop, "10% of commands unanswered"},
		{"busy targets", Step{Sent: 8, Skipped: 2}, steady, Warn, "targets still busy"},
		{"slow p95", Step{Sent: 20, Cmd: Stats{P95: 1600 * ms}}, steady, Stop, "command p95 above 1.5 s"},
		{"drift, fast", Step{Sent: 30}, append(cmds(10, 50*ms), cmds(20, 400*ms)...), Warn, "latency trending up"},
		{"drift, slow", Step{Sent: 30, Cmd: Stats{P95: 900 * ms}}, append(cmds(10, 50*ms), cmds(20, 400*ms)...), Stop, "queue builds up"},
		{"canary slow", Step{Sent: 20, Canary: Stats{P95: 400 * ms}}, steady, Stop, "rest of the network slowed down"},
		{"canary slower", Step{Sent: 20, Canary: Stats{P50: 150 * ms, P95: 200 * ms}}, steady, Warn, "noticeably slower"},
		{"canary lost once", Step{Sent: 20, Canary: Stats{Lost: 1}}, steady, Warn, "one canary probe lost"},
		{"canary gone", Step{Sent: 20, Canary: Stats{Lost: 2}}, steady, Stop, "canary stopped answering"},
		{"one error", Step{Sent: 20, Errs: 1}, steady, Warn, "Zigbee2MQTT logged an error"},
		{"errors", Step{Sent: 20, Errs: 3}, steady, Stop, "Zigbee2MQTT logged errors"},
	}
	for _, c := range cases {
		// drift compares the first and last third in send order, so the slow ones come last
		for i := range c.cmds {
			c.cmds[i].Sent = t0.Add(time.Duration(i) * time.Second)
		}
		Evaluate(&c.step, c.cmds, lim)
		if c.step.Verdict != c.want || !strings.Contains(strings.Join(c.step.Reasons, "; "), c.reason) {
			t.Errorf("%s: %s %v, want %s with %q", c.name, c.step.Verdict, c.step.Reasons, c.want, c.reason)
		}
	}
}

func TestDriftingNeedsEnoughAnswers(t *testing.T) {
	ms := time.Millisecond
	if Drifting(append(cmds(4, 50*ms), cmds(4, 900*ms)...)) {
		t.Error("eight answers are too few to judge drift")
	}
	if Drifting(append(cmds(10, 50*ms), cmds(20, 200*ms)...)) {
		t.Error("150 ms more is below the drift margin")
	}
}

func TestTrackerMatchesWithinSlackAndExpires(t *testing.T) {
	rec := &Recorder{}
	tr := NewTracker(rec, 5*time.Second, palette.KeyFunc(palette.Brightness))
	level := func(b int) z2m.State { return z2m.State{Brightness: &b} }
	tr.Add("lamp-1", 100, Pending{Sent: t0, Phase: "step", Rate: 4})
	tr.Add("lamp-2", 200, Pending{Sent: t0, Phase: "step", Rate: 4})
	if !tr.Busy("lamp-1") || tr.InFlight() != 2 {
		t.Fatalf("busy %v, in flight %d", tr.Busy("lamp-1"), tr.InFlight())
	}
	tr.OnState("lamp-1", level(100+palette.KeySlack+1), t0.Add(time.Second)) // too far off: another command
	tr.OnState("lamp-1", level(100+palette.KeySlack), t0.Add(80*time.Millisecond))
	if tr.Busy("lamp-1") {
		t.Error("an echo within the slack confirms the command")
	}
	tr.Expire(t0.Add(6 * time.Second))
	samples, _ := rec.Snapshot()
	if len(samples) != 2 || !samples[0].OK || samples[0].Latency != 80*time.Millisecond || samples[1].OK || samples[1].Device != "lamp-2" {
		t.Errorf("samples %+v: want lamp-1 answered after 80 ms, lamp-2 lost", samples)
	}
}
