package mqtt

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Subscribe registers fn for messages matching filter (QoS 0) and waits for
// SUBACK. When that fails, nothing of the subscription is left behind.
func (c *Client) Subscribe(filter string, fn Handler) error {
	sub := &subscription{filter: filter, fn: fn}
	c.smu.Lock()
	id := c.packetID()
	ack := make(chan []byte, 1)
	c.acks[id] = ack
	c.subs = append(c.subs, sub)
	c.smu.Unlock()

	err := c.subscribe(id, ack, filter)
	if err != nil {
		c.smu.Lock()
		delete(c.acks, id)
		c.subs = without(c.subs, func(s *subscription) bool { return s == sub })
		c.smu.Unlock()
	}
	return err
}

func (c *Client) subscribe(id uint16, ack <-chan []byte, filter string) error {
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

// Unsubscribe drops every handler for filter and tells the broker. It does not
// wait for the UNSUBACK: no message reaches a handler from here on anyway.
func (c *Client) Unsubscribe(filter string) error {
	c.smu.Lock()
	id := c.packetID()
	c.subs = without(c.subs, func(s *subscription) bool { return s.filter == filter })
	c.smu.Unlock()

	var vh []byte
	vh = binary.BigEndian.AppendUint16(vh, id)
	vh = appendString(vh, filter)
	return c.write(typeUnsubscribe<<4|subscribeBits, vh)
}

// packetID returns the next packet identifier, skipping 0, which MQTT does not
// allow. The caller holds smu.
func (c *Client) packetID() uint16 {
	c.nextID++
	if c.nextID == 0 {
		c.nextID = 1
	}
	return c.nextID
}

// without returns subs without the ones drop picks, in a new array:
// handlePublish ranges over the old one outside the lock.
func without(subs []*subscription, drop func(*subscription) bool) []*subscription {
	return slices.DeleteFunc(slices.Clone(subs), drop)
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
