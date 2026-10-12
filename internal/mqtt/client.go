// Package mqtt adapts paho.mqtt.golang to zigbeat's small client API: QoS 0,
// retained publish and will, live-only subscribe, keepalive, no automatic
// reconnect (the commands dial again themselves).
package mqtt

import (
	"errors"
	"fmt"
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
		SetConnectionLostHandler(func(_ paho.Client, err error) { cl.shutdown(fmt.Errorf("connection lost: %w", err)) })
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

// Subscribe registers fn for messages matching filter (QoS 0) and waits for
// the SUBACK. One handler per filter: a second one replaces the first.
func (c *Client) Subscribe(filter string, fn Handler) error {
	return c.subscribe(filter, func(_ paho.Client, m paho.Message) { fn(m.Topic(), m.Payload(), time.Now()) })
}

// SubscribeLive is Subscribe without the retained messages the broker sends
// for a new subscription: fn only gets what is published from now on.
func (c *Client) SubscribeLive(filter string, fn Handler) error {
	return c.subscribe(filter, func(_ paho.Client, m paho.Message) {
		if !m.Retained() {
			fn(m.Topic(), m.Payload(), time.Now())
		}
	})
}

// subscribe waits for the SUBACK. paho routes filter to h before it asks the
// broker, so a refusal or a timeout takes the route away again.
func (c *Client) subscribe(filter string, h paho.MessageHandler) error {
	t := c.c.Subscribe(filter, qos0, h)
	err := wait(t)
	if err == nil && t.(*paho.SubscribeToken).Result()[filter] == subackFailure {
		err = fmt.Errorf("subscription to %q refused", filter)
	}
	if err != nil {
		c.c.Unsubscribe(filter)
	}
	return err
}

// Unsubscribe drops the handler for filter at once and waits for the broker.
func (c *Client) Unsubscribe(filter string) error { return wait(c.c.Unsubscribe(filter)) }

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

func wait(t paho.Token) error {
	if !t.WaitTimeout(ioWait) {
		return errors.New("no answer from the broker")
	}
	return t.Error()
}
