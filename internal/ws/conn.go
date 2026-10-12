// Package ws adapts github.com/gorilla/websocket to zigbeat's small API: ws://
// only, text and binary messages, close codes, a read deadline and an idle
// timeout. The server side is for tests and fake-ma.
package ws

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// maxMessage caps what a bad peer can make us hold, far above MA's largest frame of about 1 KiB.
	maxMessage = 4 << 20
	// writeTimeout gives up on a peer that stopped reading.
	writeTimeout = 5 * time.Second
)

// Conn is one WebSocket connection. Read, SetReadDeadline and SetIdleTimeout
// belong to one goroutine; writes may come from any.
type Conn struct {
	c    *websocket.Conn
	idle time.Duration // zero, or how long Read waits for each message
	wmu  sync.Mutex    // gorilla takes one writer at a time
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

// StatusError is a handshake the server answered with another HTTP status.
type StatusError struct {
	Code   int
	Status string
}

func (e *StatusError) Error() string { return "ws: " + e.Status }

// Dial opens a client connection to a ws:// URL, giving up after timeout. A
// Dialer without Proxy keeps HTTP_PROXY away from a connection on the LAN.
func Dial(rawURL string, timeout time.Duration) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("ws: unsupported scheme %q", u.Scheme)
	}
	d := websocket.Dialer{HandshakeTimeout: timeout}
	c, resp, err := d.Dial(rawURL, nil)
	if err != nil {
		if resp != nil && resp.StatusCode != http.StatusSwitchingProtocols {
			return nil, &StatusError{Code: resp.StatusCode, Status: resp.Status}
		}
		return nil, err
	}
	return newConn(c), nil
}

// Upgrade answers a client's handshake and returns the server side.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	return newConn(c), nil
}

func newConn(c *websocket.Conn) *Conn {
	c.SetReadLimit(maxMessage)
	return &Conn{c: c}
}

// Read returns the next text or binary message. It answers pings and returns
// a *CloseError when the peer closes. After a timeout the connection is done.
func (c *Conn) Read() (Message, error) {
	if c.idle > 0 {
		_ = c.c.SetReadDeadline(time.Now().Add(c.idle))
	}
	typ, p, err := c.c.ReadMessage()
	var ce *websocket.CloseError
	if errors.As(err, &ce) {
		return Message{}, &CloseError{Code: ce.Code, Reason: ce.Text}
	}
	if err != nil {
		return Message{}, err
	}
	return Message{Binary: typ == websocket.BinaryMessage, Data: p}, nil
}

// WriteText sends one text message.
func (c *Conn) WriteText(p []byte) error { return c.write(websocket.TextMessage, p) }

// WriteBinary sends one binary message.
func (c *Conn) WriteBinary(p []byte) error { return c.write(websocket.BinaryMessage, p) }

func (c *Conn) write(typ int, p []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.c.WriteMessage(typ, p)
}

// Ping sends a ping; the peer's Read answers it. Like CloseWith, it may run
// next to a write.
func (c *Conn) Ping() error {
	return c.c.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout))
}

// SetReadDeadline makes Read fail after t.
func (c *Conn) SetReadDeadline(t time.Time) error { return c.c.SetReadDeadline(t) }

// SetIdleTimeout makes Read fail when no message arrives for d; it wins over
// SetReadDeadline, and pings do not count.
func (c *Conn) SetIdleTimeout(d time.Duration) { c.idle = d }

// CloseWith sends a close frame with code and reason and closes the connection.
func (c *Conn) CloseWith(code int, reason string) error {
	_ = c.c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, reason), time.Now().Add(writeTimeout))
	return c.c.Close()
}

// Close closes the connection normally.
func (c *Conn) Close() error { return c.CloseWith(websocket.CloseNormalClosure, "") }
