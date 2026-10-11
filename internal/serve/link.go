package serve

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
)

// link is serve's own MQTT connection to Home Assistant, apart from the show's.
// After connect, only run touches the client.
type link struct {
	broker string
	opts   mqtt.Options
	disc   []byte
	cmds   chan bool // switch commands, ON is true; newest wins
	client *mqtt.Client

	mu   sync.Mutex
	last []byte        // the state to publish
	wake chan struct{} // tells run to publish last
}

func newLink(cfg config.Config, version string) *link {
	return &link{
		broker: cfg.MQTT.Broker,
		opts: mqtt.Options{
			ClientID:    fmt.Sprintf("zigbeat-serve-%d", os.Getpid()),
			Username:    cfg.MQTT.Username,
			Password:    cfg.MQTT.Password,
			KeepAlive:   20 * time.Second,
			WillTopic:   availTopic,
			WillPayload: []byte("offline"),
		},
		disc: discovery(version),
		cmds: make(chan bool, 1),
		last: state{Show: "OFF", Status: "off"}.json(),
		wake: make(chan struct{}, 1),
	}
}

// connect dials and announces zigbeat. The command topic is cleared before the
// subscription, so a stale retained ON can never start a show.
func (l *link) connect() error {
	c, err := mqtt.Dial(l.broker, l.opts)
	if err != nil {
		return fmt.Errorf("mqtt: %v", err)
	}
	err = c.PublishRetained(commandTopic, nil)
	if err == nil {
		err = c.Subscribe(commandTopic, l.command)
	}
	if err == nil {
		err = c.PublishRetained(availTopic, []byte("online"))
	}
	if err == nil {
		err = c.PublishRetained(discoveryTopic, l.disc)
	}
	if err == nil {
		err = c.PublishRetained(stateTopic, l.current())
	}
	if err != nil {
		c.Close()
		return fmt.Errorf("home assistant: %v", err)
	}
	l.client = c
	return nil
}

// command runs on the MQTT reader. Every command gets the state back, so a
// switch waiting for an answer gets one even when nothing changes.
func (l *link) command(_ string, p []byte, _ time.Time) {
	switch string(p) {
	case "ON":
		newest(l.cmds, true)
	case "OFF":
		newest(l.cmds, false)
	default:
		return
	}
	l.signal()
}

// put sets the state to publish. It never blocks, so the show loop may call it.
func (l *link) put(st state) {
	l.mu.Lock()
	l.last = st.json()
	l.mu.Unlock()
	l.signal()
}

func (l *link) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *link) current() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last
}

// run publishes each new state and dials again every 5 s after the connection
// drops, until ctx is done. Then it publishes the last state and offline, and
// disconnects; a clean disconnect does not trigger the will.
func (l *link) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			_ = l.client.PublishRetained(stateTopic, l.current())
			_ = l.client.PublishRetained(availTopic, []byte("offline"))
			l.client.Close()
			return
		case <-l.wake:
			_ = l.client.PublishRetained(stateTopic, l.current())
		case <-l.client.Done():
			fmt.Printf("%s  home assistant link lost (%v); reconnecting every 5 s\n", cli.Stamp(), l.client.Err())
			if !l.redial(ctx) {
				return
			}
			fmt.Printf("%s  home assistant link back\n", cli.Stamp())
		}
	}
}

// redial connects again every 5 s until it works or ctx is done.
func (l *link) redial(ctx context.Context) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(5 * time.Second):
		}
		err := l.connect()
		if err == nil {
			return true
		}
		fmt.Fprintf(os.Stderr, "home assistant reconnect: %v\n", err)
	}
}
