package mqtt

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

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
