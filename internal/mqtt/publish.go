package mqtt

func (c *Client) Publish(topic string, payload []byte) error { return c.publish(topic, payload, 0) }

// PublishRetained publishes a message the broker keeps for later subscribers.
func (c *Client) PublishRetained(topic string, payload []byte) error {
	return c.publish(topic, payload, flagRetain)
}

func (c *Client) publish(topic string, payload []byte, flags byte) error {
	var vh []byte
	vh = appendString(vh, topic)
	vh = append(vh, payload...)
	return c.write(typePublish<<4|flags, vh)
}
