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
	"github.com/siemenvdn/zigbeat/internal/serve/ha"
)

// link is serve's own MQTT connection to Home Assistant, apart from the show's.
// After connect, only run touches the client.
type link struct {
	broker  string
	opts    mqtt.Options
	disc    []byte
	cmds    chan bool           // switch commands, ON is true; newest wins
	styles  chan config.Effects // the style after a choice; newest wins
	offsets chan time.Duration  // the audio offset after a choice; newest wins
	base    config.Config       // the config, without the choices
	client  *mqtt.Client

	mu    sync.Mutex
	state ha.State      // the state to publish, without the style
	style config.Config // the config with the choices over it
	wake  chan struct{} // tells run to publish the state
}

func newLink(cfg config.Config, version string) *link {
	return &link{
		broker: cfg.MQTT.Broker,
		opts: mqtt.Options{
			ClientID:    fmt.Sprintf("zigbeat-serve-%d", os.Getpid()),
			Username:    cfg.MQTT.Username,
			Password:    cfg.MQTT.Password,
			WillTopic:   ha.AvailTopic,
			WillPayload: []byte("offline"),
		},
		disc:    ha.Discovery(version),
		cmds:    make(chan bool, 1),
		styles:  make(chan config.Effects, 1),
		offsets: make(chan time.Duration, 1),
		base:    cfg,
		state:   ha.Off,
		style:   cfg,
		wake:    make(chan struct{}, 1),
	}
}

// connect dials and announces zigbeat. The command topic is cleared before the
// subscription, so a stale retained ON can never start a show. The style
// topics keep their retained choices, which arrive as commands.
func (l *link) connect() error {
	c, err := mqtt.Dial(l.broker, l.opts)
	if err != nil {
		return fmt.Errorf("mqtt: %v", err)
	}
	err = c.PublishRetained(ha.CommandTopic, nil)
	if err == nil {
		err = c.Subscribe(ha.CommandTopic, l.command)
	}
	if err == nil {
		err = c.Subscribe(ha.StyleFilter, l.choose)
	}
	if err == nil {
		err = c.PublishRetained(ha.AvailTopic, []byte("online"))
	}
	if err == nil {
		err = c.PublishRetained(ha.DiscoveryTopic, l.disc)
	}
	if err == nil {
		err = c.PublishRetained(ha.StateTopic, l.current())
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
func (l *link) put(st ha.State) {
	l.mu.Lock()
	l.state = st
	l.mu.Unlock()
	l.signal()
}

func (l *link) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

// current is the state to publish, with the style in force.
func (l *link) current() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.state
	st.Style = ha.StyleOf(l.style)
	return st.JSON()
}

// run publishes each new state and dials again every cli.RedialEvery after the connection
// drops, until ctx is done. Then it publishes the last state and offline, and
// disconnects; a clean disconnect does not trigger the will.
func (l *link) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			_ = l.client.PublishRetained(ha.StateTopic, l.current())
			_ = l.client.PublishRetained(ha.AvailTopic, []byte("offline"))
			l.client.Close()
			return
		case <-l.wake:
			_ = l.client.PublishRetained(ha.StateTopic, l.current())
		case <-l.client.Done():
			fmt.Printf("%s  home assistant link lost (%v); reconnecting every %s\n", cli.Stamp(), l.client.Err(), cli.Seconds(cli.RedialEvery))
			if !cli.Redial(ctx, "home assistant reconnect", l.connect) {
				return
			}
			fmt.Printf("%s  home assistant link back\n", cli.Stamp())
		}
	}
}
