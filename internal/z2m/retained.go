package z2m

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// RetainedWait is how long a command waits for a retained bridge message after subscribing.
const RetainedWait = 5 * time.Second

// ErrNoRetained is the start of the error when no retained message came within RetainedWait.
var ErrNoRetained = errors.New("no retained")

// Groups waits up to RetainedWait for the retained list of group names.
func (b *Bridge) Groups(ctx context.Context) ([]string, error) {
	return first(ctx, b, "bridge/groups", groupNames)
}

// Devices waits up to RetainedWait for the retained device list.
func (b *Bridge) Devices(ctx context.Context) ([]Device, error) {
	return first(ctx, b, "bridge/devices", func(p []byte) (ds []Device, err error) {
		return ds, json.Unmarshal(p, &ds)
	})
}

func first[T any](ctx context.Context, b *Bridge, topic string, decode func([]byte) (T, error)) (T, error) {
	var none T
	topic = b.base + "/" + topic
	got := make(chan []byte, 1)
	err := b.c.Subscribe(topic, func(_ string, p []byte, _ time.Time) {
		select {
		case got <- p:
		default:
		}
	})
	if err != nil {
		return none, err
	}
	defer b.c.Unsubscribe(topic)
	t := time.NewTimer(RetainedWait)
	defer t.Stop()
	select {
	case p := <-got:
		v, err := decode(p)
		if err != nil {
			return none, fmt.Errorf("%s: %w", topic, err)
		}
		return v, nil
	case <-t.C:
		return none, fmt.Errorf("%w %s within %v", ErrNoRetained, topic, RetainedWait)
	case <-ctx.Done():
		return none, ctx.Err()
	}
}
