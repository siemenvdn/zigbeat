package run

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// link is the show's connection to Zigbee2MQTT; tests replace it with a fake.
type link interface {
	Set(light string, payload any) error // a state for light, on its set topic
	Get(light string, payload any) error // ask light for its state
	Lost() <-chan struct{}               // closed when the connection drops
	Err() error                          // why it dropped
	// Reconnect dials until it works, and subscribes again; false when ctx is done first.
	Reconnect(ctx context.Context) bool
	Groups(ctx context.Context) ([]string, error)      // the retained group names
	Devices(ctx context.Context) ([]z2m.Device, error) // the retained device list
	Close()
}

// mqttLink is the real link: an MQTT client with subscriptions for the lights,
// the canary and the Zigbee2MQTT log.
type mqttLink struct {
	cfg    config.MQTT
	watch  func() []string // devices to subscribe to, read at every dial
	push   func(event)
	client *mqtt.Client
	bridge *z2m.Bridge
	dialed time.Time // the last dial, kept one cli.RedialEvery from the next
}

func dialLink(cfg config.MQTT, watch func() []string, push func(event)) (*mqttLink, error) {
	l := &mqttLink{cfg: cfg, watch: watch, push: push}
	return l, l.dial()
}

func (l *mqttLink) dial() error {
	l.dialed = time.Now()
	client, err := mqtt.Dial(l.cfg.Broker, mqtt.Options{
		ClientID: fmt.Sprintf("zigbeat-run-%d", os.Getpid()),
		Username: l.cfg.Username,
		Password: l.cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("mqtt: %w", err)
	}
	bridge := z2m.New(client, l.cfg.BaseTopic)
	for _, d := range l.watch() {
		if err := bridge.OnState(d, func(st z2m.State, at time.Time) { l.push(event{light: d, st: st, at: at}) }); err != nil {
			client.Close()
			return fmt.Errorf("subscribe: %w", err)
		}
	}
	err = bridge.OnLog(func(e z2m.LogEntry, at time.Time) {
		if e.Level == "error" {
			l.push(event{msg: e.Message, at: at})
		}
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("subscribe: %w", err)
	}
	l.client, l.bridge = client, bridge
	return nil
}

// Reconnect dials again every cli.RedialEvery until it works or ctx is done.
// The first dial goes at once when the last one is that long ago, so a stop
// after the broker came back does not wait.
func (l *mqttLink) Reconnect(ctx context.Context) bool {
	l.client.Close()
	if ctx.Err() == nil && time.Since(l.dialed) >= cli.RedialEvery && l.dial() == nil {
		return true
	}
	return cli.Redial(ctx, "reconnect", l.dial)
}

func (l *mqttLink) Set(light string, p any) error { return l.bridge.Set(light, p) }
func (l *mqttLink) Get(light string, p any) error { return l.bridge.Get(light, p) }
func (l *mqttLink) Lost() <-chan struct{}         { return l.client.Done() }
func (l *mqttLink) Err() error                    { return l.client.Err() }
func (l *mqttLink) Close()                        { l.client.Close() }

// Groups reads the retained list of Zigbee2MQTT groups.
func (l *mqttLink) Groups(ctx context.Context) ([]string, error) { return l.bridge.Groups(ctx) }

// Devices reads the retained list of Zigbee2MQTT devices.
func (l *mqttLink) Devices(ctx context.Context) ([]z2m.Device, error) { return l.bridge.Devices(ctx) }
