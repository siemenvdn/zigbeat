// Package mqtt adapts paho.mqtt.golang to zigbeat's small client API: QoS 0,
// retained publish and will, live-only subscribe, keepalive, no automatic
// reconnect (the commands dial again themselves).
package mqtt

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// DefaultBroker is a broker on this host, on MQTT's port.
const DefaultBroker = "127.0.0.1:1883"

const (
	// defaultKeepAlive notices a dead broker within half a minute, with pingWait.
	defaultKeepAlive = 20 * time.Second
	pingWait         = defaultKeepAlive / 2
	// ioWait bounds a dial, a write and a broker's answer: a LAN broker answers in
	// milliseconds, so this long means it is gone.
	ioWait        = 5 * time.Second
	quiesceMS     = 250 // paho's time to send the DISCONNECT
	qos0          = 0
	subackFailure = 0x80
	protocolLevel = 4 // MQTT 3.1.1
)

// Handler is called on paho's router goroutine, one message at a time in the
// order they arrive, and must not block.
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

type Client struct {
	c    paho.Client
	once sync.Once
	done chan struct{}
	mu   sync.Mutex
	err  error
}

func Dial(addr string, o Options) (*Client, error) {
	if o.KeepAlive == 0 {
		o.KeepAlive = defaultKeepAlive
	}
	cl := &Client{done: make(chan struct{})}
	po := paho.NewClientOptions().AddBroker("tcp://" + addr).SetClientID(o.ClientID).
		SetUsername(o.Username).SetPassword(o.Password).SetProtocolVersion(protocolLevel).
		SetKeepAlive(o.KeepAlive).SetPingTimeout(pingWait).SetConnectTimeout(ioWait).SetWriteTimeout(ioWait).
		SetAutoReconnect(false).SetOrderMatters(true). // one handler at a time, in order
		SetConnectionLostHandler(func(_ paho.Client, err error) { cl.shutdown(fmt.Errorf("connection lost: %w", err)) }).
		SetCustomOpenConnectionFn(dialDirect)
	if o.WillTopic != "" {
		po.SetBinaryWill(o.WillTopic, o.WillPayload, qos0, true)
	}
	cl.c = paho.NewClient(po)
	if err := wait(cl.c.Connect()); err != nil {
		return nil, err
	}
	return cl, nil
}

func (c *Client) Publish(topic string, payload []byte) error {
	return wait(c.c.Publish(topic, qos0, false, payload))
}

// PublishRetained publishes a message the broker keeps for later subscribers.
func (c *Client) PublishRetained(topic string, payload []byte) error {
	return wait(c.c.Publish(topic, qos0, true, payload))
}

func (c *Client) Close() error {
	c.c.Disconnect(quiesceMS)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

// shutdown records why the connection ended; paho calls nothing on a local close.
func (c *Client) shutdown(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		c.mu.Unlock()
		close(c.done)
	})
}

// dialDirect opens the TCP connection itself: paho's own would take a proxy
// from ALL_PROXY, also for a broker on the LAN.
func dialDirect(u *url.URL, _ paho.ClientOptions) (net.Conn, error) {
	return net.DialTimeout("tcp", u.Host, ioWait)
}

func wait(t paho.Token) error {
	if !t.WaitTimeout(ioWait) {
		return errors.New("no answer from the broker")
	}
	return t.Error()
}
