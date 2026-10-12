// Package mqtt is a minimal MQTT 3.1.1 client: QoS 0 publish, QoS 0 subscribe and unsubscribe, keepalive, retained will.
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
	typeUnsubscribe = 10
	typePingreq     = 12
	typePingresp    = 13
	typeDisconnect  = 14
	protocolLevel   = 4
	maxRemainingLen = 268435455
	maxLenBytes     = 4 // bytes of the remaining length, at most
	maxHeader       = 1 + maxLenBytes

	// CONNECT flags.
	flagCleanSession = 0x02
	flagWill         = 0x04
	flagWillRetain   = 0x20
	flagPassword     = 0x40
	flagUsername     = 0x80

	flagRetain    = 0x01 // PUBLISH
	subscribeBits = 0x02 // reserved bits of SUBSCRIBE and UNSUBSCRIBE
	subackFailure = 0x80
)

// DefaultBroker is a broker on this host, on MQTT's port.
const DefaultBroker = "127.0.0.1:1883"

const (
	// defaultKeepAlive notices a dead broker within half a minute (keepAliveGrace).
	defaultKeepAlive = 20 * time.Second
	// keepAliveGrace is the read deadline in keepalives, as MQTT 3.1.1 3.1.2.10 allows.
	keepAliveGrace = 1.5
	// pingsPerKeepAlive leaves room for one late ping within the keepalive.
	pingsPerKeepAlive = 2
	// ioWait bounds a dial, a write and a broker's answer: a LAN broker answers in
	// milliseconds, so this long means it is gone.
	ioWait = 5 * time.Second
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
	subs   []*subscription
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
