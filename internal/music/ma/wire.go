package ma

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	silence = 60 * time.Second

	dialWait = 10 * time.Second
	authWait = 10 * time.Second
)

func (s Source) dial() (*ws.Conn, error) {
	u := strings.TrimSuffix(s.URL, "/") + "/milkdrop_visualizer"
	if s.Player != "" {
		u += "?player=" + url.QueryEscape(s.Player)
	}
	c, err := ws.Dial(u, dialWait)
	if err != nil {
		return nil, err
	}
	auth, _ := json.Marshal(map[string]string{"type": "auth", "token": s.Token})
	err = c.WriteText(auth)
	var m ws.Message
	if err == nil {
		_ = c.SetReadDeadline(time.Now().Add(authWait))
		m, err = c.Read()
	}
	if err == nil && parseType(m) != "auth_ok" {
		err = fmt.Errorf("unexpected answer to auth: %.80s", m.Data)
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func read(c *ws.Conn, clk *clock, msgs chan<- message, done <-chan struct{}) {
	c.SetIdleTimeout(silence)
	for {
		m, err := c.Read()
		msg := message{kind: kindFailed, err: err}
		if err == nil {
			msg = parse(m, clk.now())
		}
		select {
		case msgs <- msg:
		case <-done:
			return
		}
		if err != nil {
			return
		}
	}
}
