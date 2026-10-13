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

type haLink struct {
	broker  string
	opts    mqtt.Options
	disc    []byte
	cmds    chan bool
	styles  chan config.Effects
	offsets chan time.Duration
	base    config.Config
	client  *mqtt.Client

	mu    sync.Mutex
	state ha.State
	style config.Config
	wake  chan struct{}
	lines chan string
}

const lineBuffer = 32

func newHALink(cfg config.Config, version string) *haLink {
	return &haLink{
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
		lines:   make(chan string, lineBuffer),
	}
}

func (l *haLink) connect() error {
	c, err := mqtt.Dial(l.broker, l.opts)
	if err != nil {
		return fmt.Errorf("mqtt: %w", err)
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
		return fmt.Errorf("home assistant: %w", err)
	}
	l.client = c
	return nil
}

func (l *haLink) command(_ string, p []byte, _ time.Time) {
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

func (l *haLink) put(st ha.State) {
	l.mu.Lock()
	l.state = st
	l.mu.Unlock()
	l.signal()
}

func (l *haLink) say(format string, args ...any) {
	select {
	case l.lines <- cli.Stamp() + "  " + fmt.Sprintf(format, args...):
	default:
	}
}

func (l *haLink) signal() {
	select {
	case l.wake <- struct{}{}:
	default:
	}
}

func (l *haLink) current() []byte {
	l.mu.Lock()
	defer l.mu.Unlock()
	st := l.state
	st.Style = ha.StyleOf(l.style)
	return st.JSON()
}

func (l *haLink) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			for len(l.lines) > 0 {
				fmt.Println(<-l.lines)
			}
			_ = l.client.PublishRetained(ha.StateTopic, l.current())
			_ = l.client.PublishRetained(ha.AvailTopic, []byte("offline"))
			l.client.Close()
			return
		case <-l.wake:
			_ = l.client.PublishRetained(ha.StateTopic, l.current())
		case line := <-l.lines:
			fmt.Println(line)
		case <-l.client.Done():
			cli.Say("home assistant link lost (%v); reconnecting every %s", l.client.Err(), cli.Seconds(cli.RedialEvery))
			if !cli.Redial(ctx, "home assistant reconnect", l.connect) {
				return
			}
			cli.Say("home assistant link back")
		}
	}
}
