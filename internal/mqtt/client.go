// Package mqtt is a minimal MQTT 3.1.1 client: QoS 0 publish, QoS 0 subscribe, keepalive, retained will.
package mqtt

import (
	"bufio"
	"net"
	"sync"
	"time"
)

const (
	typeConnect     = 1
	typeConnack     = 2
	typePublish     = 3
	typePuback      = 4
	typeSubscribe   = 8
	typeSuback      = 9
	typePingreq     = 12
	typePingresp    = 13
	typeDisconnect  = 14
	protocolLevel   = 4
	maxRemainingLen = 268435455
)

// Handler is called from the read loop and must not block.
type Handler func(topic string, payload []byte, received time.Time)

type Options struct {
	ClientID  string
	Username  string
	Password  string
	KeepAlive time.Duration
	// WillTopic, when set, gets WillPayload retained if the connection drops without a DISCONNECT.
	WillTopic   string
	WillPayload []byte
}

type subscription struct {
	filter string
	fn     Handler
}

type Client struct {
	conn net.Conn
	r    *bufio.Reader
	ka   time.Duration

	wmu sync.Mutex

	smu    sync.RWMutex
	subs   []subscription
	nextID uint16
	acks   map[uint16]chan []byte

	closeOnce sync.Once
	done      chan struct{}
	err       error
}

func (c *Client) Close() error {
	_ = c.write(typeDisconnect<<4, nil)
	c.shutdown(nil)
	return nil
}

func (c *Client) Done() <-chan struct{} { return c.done }

// Closed reports whether the connection is gone.
func (c *Client) Closed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Client) Err() error {
	c.smu.RLock()
	defer c.smu.RUnlock()
	return c.err
}

func (c *Client) shutdown(err error) {
	c.closeOnce.Do(func() {
		c.smu.Lock()
		c.err = err
		c.smu.Unlock()
		c.conn.Close()
		close(c.done)
	})
}
