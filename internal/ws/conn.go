// Package ws is a minimal WebSocket (RFC 6455) client, plus the server side
// for tests and fakes. No extensions, compression or TLS.
package ws

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	opCont   = 0x0
	opText   = 0x1
	opBinary = 0x2
	opClose  = 0x8
	opPing   = 0x9
	opPong   = 0xA

	// maxMessage caps what a bad peer can make us hold, far above MA's largest frame of about 1 KiB.
	maxMessage = 4 << 20
	// writeTimeout gives up on a peer that stopped reading.
	writeTimeout = 5 * time.Second

	// RFC 6455 close codes.
	closeNormal   = 1000
	closeNoStatus = 1005
)

// Conn is one WebSocket connection. Read must be called from one goroutine;
// writes may come from any.
type Conn struct {
	nc     net.Conn
	r      *bufio.Reader
	client bool // a client masks its frames
	wmu    sync.Mutex
}

// Message is one text or binary message.
type Message struct {
	Binary bool
	Data   []byte
}

// CloseError is the close frame the peer sent.
type CloseError struct {
	Code   int
	Reason string
}

func (e *CloseError) Error() string { return fmt.Sprintf("closed by peer: %d %s", e.Code, e.Reason) }

// Read returns the next text or binary message. It answers pings and returns
// a *CloseError when the peer closes.
func (c *Conn) Read() (Message, error) {
	var msg []byte
	var op byte
	for {
		fin, o, payload, err := c.readFrame()
		if err != nil {
			return Message{}, err
		}
		switch o {
		case opPing:
			if err := c.write(opPong, payload); err != nil {
				return Message{}, err
			}
			continue
		case opPong:
			continue
		case opClose:
			ce := &CloseError{Code: closeNoStatus}
			if len(payload) >= 2 {
				ce.Code, ce.Reason = int(binary.BigEndian.Uint16(payload)), string(payload[2:])
			}
			_ = c.write(opClose, payload[:min(2, len(payload))])
			return Message{}, ce
		case opText, opBinary:
			if op != 0 {
				return Message{}, errors.New("ws: new message inside a fragmented one")
			}
			op = o
		case opCont:
			if op == 0 {
				return Message{}, errors.New("ws: continuation without a message")
			}
		default:
			return Message{}, fmt.Errorf("ws: unknown opcode %d", o)
		}
		if msg = append(msg, payload...); len(msg) > maxMessage {
			return Message{}, errors.New("ws: message too large")
		}
		if fin {
			return Message{Binary: op == opBinary, Data: msg}, nil
		}
	}
}

// WriteText sends one text message.
func (c *Conn) WriteText(p []byte) error { return c.write(opText, p) }

// WriteBinary sends one binary message.
func (c *Conn) WriteBinary(p []byte) error { return c.write(opBinary, p) }

// Ping sends a ping; the peer's Read answers it.
func (c *Conn) Ping() error { return c.write(opPing, nil) }

// SetReadDeadline makes Read fail after t.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.nc.SetReadDeadline(t) }

// CloseWith sends a close frame with code and reason and closes the connection.
func (c *Conn) CloseWith(code int, reason string) error {
	_ = c.write(opClose, append(binary.BigEndian.AppendUint16(nil, uint16(code)), reason...))
	return c.nc.Close()
}

// Close closes the connection normally.
func (c *Conn) Close() error { return c.CloseWith(closeNormal, "") }
