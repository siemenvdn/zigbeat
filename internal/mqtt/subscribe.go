package mqtt

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

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
	if err := c.write(typeSubscribe<<4|subscribeBits, vh); err != nil {
		return err
	}
	select {
	case codes := <-ack:
		if len(codes) == 0 || codes[0] == subackFailure {
			return fmt.Errorf("subscription to %q refused", filter)
		}
		return nil
	case <-time.After(ioWait):
		return fmt.Errorf("no SUBACK for %q", filter)
	case <-c.done:
		return c.Err()
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
