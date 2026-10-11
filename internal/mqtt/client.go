// Package mqtt is a minimal MQTT 3.1.1 client: QoS 0 publish, QoS 0 subscribe, keepalive.
package mqtt

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
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

var connackErrors = map[byte]string{
	1: "unacceptable protocol version",
	2: "client identifier rejected",
	3: "server unavailable",
	4: "bad username or password",
	5: "not authorized",
}

func Dial(addr string, opts Options) (*Client, error) {
	if opts.KeepAlive == 0 {
		opts.KeepAlive = 30 * time.Second
	}
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return nil, err
	}
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
	}
	c := &Client{
		conn: conn,
		r:    bufio.NewReader(conn),
		ka:   opts.KeepAlive,
		acks: map[uint16]chan []byte{},
		done: make(chan struct{}),
	}
	if err := c.handshake(opts); err != nil {
		conn.Close()
		return nil, err
	}
	go c.readLoop()
	go c.pingLoop()
	return c, nil
}

func (c *Client) handshake(o Options) error {
	var vh []byte
	vh = appendString(vh, "MQTT")
	flags := byte(0x02) // clean session
	if o.Username != "" {
		flags |= 0x80
	}
	if o.Password != "" {
		flags |= 0x40
	}
	vh = append(vh, protocolLevel, flags)
	vh = binary.BigEndian.AppendUint16(vh, uint16(o.KeepAlive/time.Second))
	vh = appendString(vh, o.ClientID)
	if o.Username != "" {
		vh = appendString(vh, o.Username)
	}
	if o.Password != "" {
		vh = appendString(vh, o.Password)
	}
	if err := c.write(typeConnect<<4, vh); err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	h, body, err := c.readPacket()
	if err != nil {
		return fmt.Errorf("waiting for CONNACK: %w", err)
	}
	if h>>4 != typeConnack || len(body) < 2 {
		return errors.New("unexpected reply to CONNECT")
	}
	if body[1] != 0 {
		if msg, ok := connackErrors[body[1]]; ok {
			return fmt.Errorf("broker refused connection: %s", msg)
		}
		return fmt.Errorf("broker refused connection: code %d", body[1])
	}
	return nil
}

// Subscribe registers fn for messages matching filter (QoS 0) and waits for SUBACK.
func (c *Client) Subscribe(filter string, fn Handler) error {
	c.smu.Lock()
	c.nextID++
	if c.nextID == 0 {
		c.nextID = 1
	}
	id := c.nextID
	ack := make(chan []byte, 1)
	c.acks[id] = ack
	c.subs = append(c.subs, subscription{filter: filter, fn: fn})
	c.smu.Unlock()

	var vh []byte
	vh = binary.BigEndian.AppendUint16(vh, id)
	vh = appendString(vh, filter)
	vh = append(vh, 0)
	if err := c.write(typeSubscribe<<4|0x02, vh); err != nil {
		return err
	}
	select {
	case codes := <-ack:
		if len(codes) == 0 || codes[0] == 0x80 {
			return fmt.Errorf("subscription to %q refused", filter)
		}
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("no SUBACK for %q", filter)
	case <-c.done:
		return c.Err()
	}
}

func (c *Client) Publish(topic string, payload []byte) error { return c.publish(topic, payload, 0) }

// PublishRetained publishes a message the broker keeps for later subscribers.
func (c *Client) PublishRetained(topic string, payload []byte) error {
	return c.publish(topic, payload, 1)
}

func (c *Client) publish(topic string, payload []byte, flags byte) error {
	var vh []byte
	vh = appendString(vh, topic)
	vh = append(vh, payload...)
	return c.write(typePublish<<4|flags, vh)
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

func (c *Client) write(header byte, body []byte) error {
	if len(body) > maxRemainingLen {
		return errors.New("packet too large")
	}
	pkt := make([]byte, 0, len(body)+5)
	pkt = append(pkt, header)
	pkt = appendVarint(pkt, len(body))
	pkt = append(pkt, body...)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write(pkt)
	if err != nil {
		c.shutdown(err)
	}
	return err
}

func (c *Client) readPacket() (byte, []byte, error) {
	h, err := c.r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	n, err := readVarint(c.r)
	if err != nil {
		return 0, nil, err
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(c.r, body); err != nil {
		return 0, nil, err
	}
	return h, body, nil
}

func (c *Client) readLoop() {
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(c.ka * 3 / 2))
		h, body, err := c.readPacket()
		if err != nil {
			c.shutdown(fmt.Errorf("connection lost: %w", err))
			return
		}
		now := time.Now()
		switch h >> 4 {
		case typePublish:
			c.handlePublish(h, body, now)
		case typeSuback:
			if len(body) >= 2 {
				id := binary.BigEndian.Uint16(body)
				c.smu.Lock()
				if ch, ok := c.acks[id]; ok {
					ch <- body[2:]
					delete(c.acks, id)
				}
				c.smu.Unlock()
			}
		case typePingresp:
		}
	}
}

func (c *Client) handlePublish(h byte, body []byte, now time.Time) {
	if len(body) < 2 {
		return
	}
	tl := int(binary.BigEndian.Uint16(body))
	if len(body) < 2+tl {
		return
	}
	topic := string(body[2 : 2+tl])
	rest := body[2+tl:]
	if qos := (h >> 1) & 0x03; qos > 0 {
		if len(rest) < 2 {
			return
		}
		id := rest[:2]
		rest = rest[2:]
		if qos == 1 {
			_ = c.write(typePuback<<4, id)
		}
	}
	c.smu.RLock()
	subs := c.subs
	c.smu.RUnlock()
	for _, s := range subs {
		if Match(s.filter, topic) {
			s.fn(topic, rest, now)
		}
	}
}

func (c *Client) pingLoop() {
	t := time.NewTicker(c.ka / 2)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_ = c.write(typePingreq<<4, nil)
		case <-c.done:
			return
		}
	}
}

// Match reports whether topic matches an MQTT filter with + and # wildcards.
func Match(filter, topic string) bool {
	fs := strings.Split(filter, "/")
	ts := strings.Split(topic, "/")
	for i, f := range fs {
		if f == "#" {
			return true
		}
		if i >= len(ts) {
			return false
		}
		if f != "+" && f != ts[i] {
			return false
		}
	}
	return len(fs) == len(ts)
}

func appendString(b []byte, s string) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(len(s)))
	return append(b, s...)
}

func appendVarint(b []byte, n int) []byte {
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 0x80
		}
		b = append(b, d)
		if n == 0 {
			return b
		}
	}
}

func readVarint(r io.ByteReader) (int, error) {
	n, mul := 0, 1
	for i := 0; i < 4; i++ {
		d, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n += int(d&0x7f) * mul
		if d&0x80 == 0 {
			return n, nil
		}
		mul *= 128
	}
	return 0, errors.New("malformed remaining length")
}
