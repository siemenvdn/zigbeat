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
	Set(light string, payload any) error
	Get(light string, payload any) error
	Lost() <-chan struct{} // closed when the connection drops
	Err() error
	Reconnect(ctx context.Context) bool
	Groups(ctx context.Context) ([]string, bool)
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
}

func dialLink(cfg config.MQTT, watch func() []string, push func(event)) (*mqttLink, error) {
	l := &mqttLink{cfg: cfg, watch: watch, push: push}
	return l, l.dial()
}

func (l *mqttLink) dial() error {
	client, err := mqtt.Dial(l.cfg.Broker, mqtt.Options{
		ClientID: fmt.Sprintf("zigbeat-run-%d", os.Getpid()),
		Username: l.cfg.Username,
		Password: l.cfg.Password,
	})
	if err != nil {
		return fmt.Errorf("mqtt: %v", err)
	}
	bridge := z2m.New(client, l.cfg.BaseTopic)
	for _, d := range l.watch() {
		if err := bridge.OnState(d, func(st z2m.State, at time.Time) { l.push(event{light: d, st: st, at: at}) }); err != nil {
			client.Close()
			return fmt.Errorf("subscribe: %v", err)
		}
	}
	err = bridge.OnLog(func(e z2m.LogEntry, at time.Time) {
		if e.Level == "error" {
			l.push(event{msg: e.Message, at: at})
		}
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("subscribe: %v", err)
	}
	l.client, l.bridge = client, bridge
	return nil
}

// Reconnect dials again every cli.RedialEvery until it works or ctx is done.
func (l *mqttLink) Reconnect(ctx context.Context) bool {
	l.client.Close()
	return cli.Redial(ctx, "reconnect", l.dial)
}

func (l *mqttLink) Set(light string, p any) error { return l.bridge.Set(light, p) }
func (l *mqttLink) Get(light string, p any) error { return l.bridge.Get(light, p) }
func (l *mqttLink) Lost() <-chan struct{}         { return l.client.Done() }
func (l *mqttLink) Err() error                    { return l.client.Err() }
func (l *mqttLink) Close()                        { l.client.Close() }

// groupsWait is far above the moment a retained message comes after its SUBACK.
const groupsWait = 2 * time.Second

// Groups reads the retained list of Zigbee2MQTT groups; false when none came within groupsWait.
func (l *mqttLink) Groups(ctx context.Context) ([]string, bool) {
	got := make(chan []string, 1)
	err := l.bridge.OnGroups(func(names []string, _ time.Time) {
		select {
		case got <- names:
		default:
		}
	})
	if err != nil {
		return nil, false
	}
	t := time.NewTimer(groupsWait)
	defer t.Stop()
	select {
	case names := <-got:
		return names, true
	case <-t.C:
	case <-ctx.Done():
	}
	return nil, false
}
