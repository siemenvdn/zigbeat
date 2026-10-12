package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

// viewer is one connected client; a writer goroutine drains out.
type viewer struct {
	c   *ws.Conn
	out chan outMsg
}

type outMsg struct {
	text bool
	data []byte
}

// fan sends a frame to every viewer; a viewer that falls behind loses it, as in MA.
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

// serve is the relay route: auth, the target check, stream/start, the pending beats, then live frames.
func (tl *timeline) serve(w http.ResponseWriter, r *http.Request, token, player string) {
	c, err := ws.Upgrade(w, r)
	if err != nil {
		return
	}
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	m, err := c.Read()
	var auth struct{ Type, Token string }
	if err != nil || json.Unmarshal(m.Data, &auth) != nil || auth.Type != "auth" || auth.Token != token {
		c.CloseWith(4001, "Invalid or expired token")
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
	_ = c.WriteText([]byte(`{"type":"stream/start","payload":{"visualizer":{"types":["waveform","beat"]}}}`))
	v := &viewer{c: c, out: make(chan outMsg, 4096)}
	tl.join(v)
	go v.write()
	defer func() {
		tl.mu.Lock()
		delete(tl.viewers, v)
		tl.mu.Unlock()
		close(v.out)
	}()
	tl.answer(c)
}

// answer replies to client/time until the connection ends.
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

// write sends the viewer's frames and a ping every 25 s, like aiohttp's heartbeat.
func (v *viewer) write() {
	ping := time.NewTicker(25 * time.Second)
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
