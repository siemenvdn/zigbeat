package mqtt

import (
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Subscribe registers fn for messages matching filter (QoS 0) and waits for the SUBACK.
func (c *Client) Subscribe(filter string, fn Handler) error { return c.subscribe(filter, fn, false) }

// SubscribeLive is Subscribe without the retained messages the broker sends for a new subscription.
func (c *Client) SubscribeLive(filter string, fn Handler) error { return c.subscribe(filter, fn, true) }

func (c *Client) subscribe(filter string, fn Handler, live bool) error {
	t := c.c.Subscribe(filter, qos0, func(_ paho.Client, m paho.Message) {
		if !live || !m.Retained() {
			fn(m.Topic(), m.Payload(), time.Now())
		}
	})
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
