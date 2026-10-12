package ma

import (
	"encoding/binary"
	"encoding/json"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

const (
	beatTag   = 17            // [17][ts:8][flags], bit 0 of flags is a downbeat
	waveTag   = 22            // [22][ts:8][1024 samples]
	headerLen = 1 + 8         // tag and timestamp
	beatLen   = headerLen + 1 // and the flags
	downBit   = 1
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
	err            error
}

type text struct {
	Type    string `json:"type"`
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
		return message{kind: kindError}
	}
	return message{}
}
