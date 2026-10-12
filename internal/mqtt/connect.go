package mqtt

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

var connackErrors = map[byte]string{
	1: "unacceptable protocol version",
	2: "client identifier rejected",
	3: "server unavailable",
	4: "bad username or password",
	5: "not authorized",
}

func Dial(addr string, opts Options) (*Client, error) {
	if opts.KeepAlive == 0 {
		opts.KeepAlive = defaultKeepAlive
	}
	conn, err := net.DialTimeout("tcp", addr, ioWait)
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
	flags := byte(flagCleanSession)
	if o.WillTopic != "" {
		flags |= flagWill | flagWillRetain // QoS 0
	}
	if o.Username != "" {
		flags |= flagUsername
	}
	if o.Password != "" {
		flags |= flagPassword
	}
	vh = append(vh, protocolLevel, flags)
	vh = binary.BigEndian.AppendUint16(vh, uint16(o.KeepAlive/time.Second))
	vh = appendString(vh, o.ClientID)
	if o.WillTopic != "" {
		vh = appendString(vh, o.WillTopic)
		vh = appendString(vh, string(o.WillPayload))
	}
	if o.Username != "" {
		vh = appendString(vh, o.Username)
	}
	if o.Password != "" {
		vh = appendString(vh, o.Password)
	}
	if err := c.write(typeConnect<<4, vh); err != nil {
		return err
	}
	_ = c.conn.SetReadDeadline(time.Now().Add(ioWait))
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

func (c *Client) pingLoop() {
	t := time.NewTicker(c.ka / pingsPerKeepAlive)
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
