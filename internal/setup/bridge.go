package setup

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/run/flow"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// bridge is what setup needs from Zigbee2MQTT; tests replace it with a fake.
type bridge interface {
	Devices(ctx context.Context) ([]z2m.Device, bool)
	// Read asks name for its state and returns how long the answer took.
	Read(ctx context.Context, name string) (time.Duration, bool)
}

type mqttBridge struct {
	client  *mqtt.Client
	z2m     *z2m.Bridge
	watched string // the device answers come from
	answers chan time.Time
}

func dial(m config.MQTT) (*mqttBridge, error) {
	c, err := mqtt.Dial(m.Broker, mqtt.Options{
		ClientID: fmt.Sprintf("zigbeat-setup-%d", os.Getpid()),
		Username: m.Username,
		Password: m.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("mqtt: %v", err)
	}
	return &mqttBridge{client: c, z2m: z2m.New(c, m.BaseTopic)}, nil
}

func (b *mqttBridge) Devices(ctx context.Context) ([]z2m.Device, bool) { return b.z2m.Devices(ctx) }

func (b *mqttBridge) Read(ctx context.Context, name string) (time.Duration, bool) {
	if b.watched != name {
		answers := make(chan time.Time, 1)
		err := b.z2m.OnState(name, func(_ z2m.State, at time.Time) {
			select {
			case answers <- at:
			default:
			}
		})
		if err != nil {
			return 0, false
		}
		b.watched, b.answers = name, answers
	}
	select {
	case <-b.answers: // an answer from before this read
	default:
	}
	sent := time.Now()
	if b.z2m.Get(name, map[string]string{"state": ""}) != nil {
		return 0, false
	}
	t := time.NewTimer(flow.LostAfter)
	defer t.Stop()
	select {
	case at := <-b.answers:
		return at.Sub(sent), true
	case <-t.C:
	case <-ctx.Done():
	}
	return 0, false
}

func (b *mqttBridge) Close() { b.client.Close() }
