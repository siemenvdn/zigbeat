package ma

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	beatTag   = 17            // [17][ts:8][flags], bit 0 of flags is a downbeat
	waveTag   = 22            // [22][ts:8][1024 samples]
	headerLen = 1 + 8         // tag and timestamp
	beatLen   = headerLen + 1 // and the flags
	downBit   = 1

	silence = 60 * time.Second // MA pings every 25 s
	// dialWait and authWait: MA answers on a LAN in milliseconds, so this long
	// means it is not there.
	dialWait = 10 * time.Second
	authWait = 10 * time.Second
)

type kind int

const (
	kindOther kind = iota
	kindBeat
	kindWave
	kindTime
	kindClear
	kindEnd
	kindError
	kindFailed
)

// message is one parsed message from the relay.
type message struct {
	kind           kind
	f              frame
	t1, t2, t3, t4 int64 // a time answer, in µs
	text           string
	err            error
}

// dial connects to the relay and authenticates.
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

// read passes every message to msgs, stamped with zigbeat's clock, until the
// connection fails or done closes.
func read(c *ws.Conn, clk *clock, msgs chan<- message, done <-chan struct{}) {
	for {
		_ = c.SetReadDeadline(time.Now().Add(silence))
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

type text struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Payload struct {
		ClientTransmitted int64 `json:"client_transmitted"`
		ServerReceived    int64 `json:"server_received"`
		ServerTransmitted int64 `json:"server_transmitted"`
	} `json:"payload"`
}

func parseType(m ws.Message) string {
	var t text
	_ = json.Unmarshal(m.Data, &t)
	return t.Type
}

func parse(m ws.Message, now int64) message {
	if m.Binary {
		if len(m.Data) < headerLen {
			return message{}
		}
		ts := int64(binary.BigEndian.Uint64(m.Data[1:headerLen]))
		switch {
		case m.Data[0] == beatTag && len(m.Data) >= beatLen:
			return message{kind: kindBeat, f: frame{ts: ts, down: m.Data[headerLen]&downBit != 0}}
		case m.Data[0] == waveTag:
			return message{kind: kindWave}
		}
		return message{}
	}
	var t text
	if json.Unmarshal(m.Data, &t) != nil {
		return message{}
	}
	switch t.Type {
	case "server/time":
		p := t.Payload
		return message{kind: kindTime, t1: p.ClientTransmitted, t2: p.ServerReceived, t3: p.ServerTransmitted, t4: now}
	case "stream/clear":
		return message{kind: kindClear}
	case "stream/end":
		return message{kind: kindEnd}
	case "error":
		return message{kind: kindError, text: t.Message}
	}
	return message{}
}
