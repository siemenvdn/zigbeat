package ma

import (
	"encoding/json"
	"time"
)

const (
	burstSize  = 8
	burstGap   = 25 * time.Millisecond
	burstEvery = 10 * time.Second
)

// startBurst sends burstSize time requests, burstGap apart; the answers feed the clock.
func (s *session) startBurst() {
	s.clk.burst()
	s.pings = burstSize
	s.sendPing()
}

func (s *session) sendPing() {
	if s.pings == 0 {
		return
	}
	s.pings--
	b, _ := json.Marshal(map[string]any{"type": "client/time", "payload": map[string]int64{"client_transmitted": s.clk.now()}})
	_ = s.conn.WriteText(b)
	if s.pings > 0 {
		s.ping.Reset(burstGap)
	}
}
