package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	authWait     = 10 * time.Second
	tokenRefused = 4001
	pingEvery    = 25 * time.Second

	viewerBuffer = 4096
)

type viewer struct {
	c   *ws.Conn
	out chan outMsg
}

type outMsg struct {
	text bool
	data []byte
}

func (tl *timeline) fan(data []byte, text bool) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	for v := range tl.viewers {
		select {
		case v.out <- outMsg{text, data}:
		default:
		}
	}
}

func (tl *timeline) serve(w http.ResponseWriter, r *http.Request, token, player string) {
	c, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Now().Add(authWait))
	m, err := c.Read()
	var auth struct{ Type, Token string }
	if err != nil || json.Unmarshal(m.Data, &auth) != nil || auth.Type != "auth" || auth.Token != token {
		c.CloseWith(tokenRefused, "Invalid or expired token")
		return
	}
	_ = c.WriteText([]byte(`{"type": "auth_ok"}`))
	tl.mu.Lock()
	playing := tl.playing
	tl.mu.Unlock()
	asked := r.URL.Query().Get("player")
	if asked != "" && player != "" && asked != player || asked == "" && !playing {
		_ = c.WriteText([]byte(`{"type":"error","message":"no such player"}`))
		c.Close()
		return
	}
	types := `"waveform","beat"`
	if tl.colors {
		types += `,"color"`
	}
	_ = c.WriteText([]byte(`{"type":"stream/start","payload":{"visualizer":{"types":[` + types + `]}}}`))
	v := &viewer{c: c, out: make(chan outMsg, viewerBuffer)}
	go v.write()
	tl.join(v)
	defer func() {
		tl.mu.Lock()
		delete(tl.viewers, v)
		tl.mu.Unlock()
		close(v.out)
	}()
	tl.answer(c)
}

func (tl *timeline) answer(c *ws.Conn) {
	for {
		_ = c.SetReadDeadline(time.Time{})
		m, err := c.Read()
		if err != nil {
			return
		}
		var req struct {
			Type    string `json:"type"`
			Payload struct {
				ClientTransmitted int64 `json:"client_transmitted"`
			} `json:"payload"`
		}
		if json.Unmarshal(m.Data, &req) != nil || req.Type != "client/time" {
			continue
		}
		now := tl.now()
		b, _ := json.Marshal(map[string]any{"type": "server/time", "payload": map[string]int64{
			"client_transmitted": req.Payload.ClientTransmitted, "server_received": now, "server_transmitted": now}})
		_ = c.WriteText(b)
	}
}

func (v *viewer) write() {
	ping := time.NewTicker(pingEvery)
	defer ping.Stop()
	for {
		select {
		case m, ok := <-v.out:
			if !ok {
				return
			}
			if m.text {
				_ = v.c.WriteText(m.data)
			} else {
				_ = v.c.WriteBinary(m.data)
			}
		case <-ping.C:
			_ = v.c.Ping()
		}
	}
}
