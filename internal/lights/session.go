package lights

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
	"github.com/siemenvdn/zigbeat/internal/mqtt"
	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// Watch is what a session subscribes to on every dial.
type Watch struct {
	Devices []string                                        // whose states to keep; Read adds more
	OnState func(device string, st z2m.State, at time.Time) // optional; on the MQTT reader, must not block
	OnLog   func(z2m.LogEntry, time.Time)                   // optional; Zigbee2MQTT's log, on the MQTT reader
}

// Session is a command's connection to Zigbee2MQTT: the MQTT client, the
// devices it watches and the last state of each. It is the Conn of the
// snapshot and the restore.
type Session struct {
	broker, base string
	opts         mqtt.Options
	watch        Watch
	client       *mqtt.Client
	bridge       *z2m.Bridge
	dialed       time.Time // the last dial, kept one cli.RedialEvery from the next

	mu      sync.Mutex
	last    map[string]stateAt
	changed chan struct{} // closed and replaced at every state, so Await wakes up
}

type stateAt struct {
	st z2m.State
	at time.Time
}

// Dial connects as zigbeat-<cmd>-<pid> and subscribes to w.
func Dial(m config.MQTT, cmd string, w Watch) (*Session, error) {
	s := &Session{broker: m.Broker, base: m.BaseTopic, watch: w, last: map[string]stateAt{}, changed: make(chan struct{}),
		opts: mqtt.Options{ClientID: fmt.Sprintf("zigbeat-%s-%d", cmd, os.Getpid()), Username: m.Username, Password: m.Password}}
	return s, s.dial()
}

func (s *Session) dial() error {
	s.dialed = time.Now()
	c, err := mqtt.Dial(s.broker, s.opts)
	if err != nil {
		return fmt.Errorf("mqtt: %w", err)
	}
	b := z2m.New(c, s.base)
	for _, d := range s.watch.Devices {
		err = s.subscribe(b, d)
		if err != nil {
			break
		}
	}
	if err == nil && s.watch.OnLog != nil {
		err = b.OnLog(s.watch.OnLog)
	}
	if err != nil {
		c.Close()
		return fmt.Errorf("subscribe: %w", err)
	}
	s.client, s.bridge = c, b
	return nil
}

func (s *Session) subscribe(b *z2m.Bridge, device string) error {
	return b.OnState(device, func(st z2m.State, at time.Time) {
		s.mu.Lock()
		s.last[device] = stateAt{st, at}
		close(s.changed)
		s.changed = make(chan struct{})
		s.mu.Unlock()
		if s.watch.OnState != nil {
			s.watch.OnState(device, st, at)
		}
	})
}

// Reconnect dials again every cli.RedialEvery until it works or ctx is done,
// and subscribes again. The first dial goes at once when the last one is that
// long ago, so a stop after the broker came back does not wait.
func (s *Session) Reconnect(ctx context.Context) bool {
	s.client.Close()
	if ctx.Err() == nil && time.Since(s.dialed) >= cli.RedialEvery && s.dial() == nil {
		return true
	}
	return cli.Redial(ctx, "reconnect", s.dial)
}

func (s *Session) Get(device string, p any) error { return s.bridge.Get(device, p) }
func (s *Session) Set(device string, p any) error { return s.bridge.Set(device, p) }

// Await waits up to d for a state of device that arrived after since. It is
// false when none came, the connection dropped or ctx is done.
func (s *Session) Await(ctx context.Context, device string, since time.Time, d time.Duration) (z2m.State, bool) {
	v, ok := s.await(ctx, device, since, d)
	return v.st, ok
}

func (s *Session) await(ctx context.Context, device string, since time.Time, d time.Duration) (stateAt, bool) {
	t := time.NewTimer(d)
	defer t.Stop()
	for {
		s.mu.Lock()
		v, ok := s.last[device]
		changed := s.changed
		s.mu.Unlock()
		if ok && v.at.After(since) {
			return v, true
		}
		select {
		case <-changed:
		case <-t.C:
			return stateAt{}, false
		case <-s.client.Done():
			return stateAt{}, false
		case <-ctx.Done():
			return stateAt{}, false
		}
	}
}

// Read asks device for its state and returns how long the answer took, waiting
// up to d. A device it did not watch yet is watched from now on.
func (s *Session) Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool) {
	if !slices.Contains(s.watch.Devices, device) {
		if s.subscribe(s.bridge, device) != nil {
			return 0, false
		}
		s.watch.Devices = append(s.watch.Devices, device)
	}
	sent := time.Now()
	if s.Get(device, z2m.StateQuery) != nil {
		return 0, false
	}
	v, ok := s.await(ctx, device, sent, d)
	return v.at.Sub(sent), ok
}

// Last is the latest state of device and when it came; false before the first.
func (s *Session) Last(device string) (z2m.State, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.last[device]
	return v.st, v.at, ok
}

// Online reports whether the connection is still there.
func (s *Session) Online() bool { return !s.client.Closed() }

func (s *Session) Done() <-chan struct{} { return s.client.Done() }
func (s *Session) Err() error            { return s.client.Err() }
func (s *Session) Close()                { s.client.Close() }

// Groups reads the retained list of Zigbee2MQTT groups.
func (s *Session) Groups(ctx context.Context) ([]string, error) { return s.bridge.Groups(ctx) }

// Devices reads the retained list of Zigbee2MQTT devices.
func (s *Session) Devices(ctx context.Context) ([]z2m.Device, error) { return s.bridge.Devices(ctx) }

// Reader reads a device and says how long its answer took.
type Reader interface {
	Read(ctx context.Context, device string, d time.Duration) (time.Duration, bool)
}

// ReadTimes reads device n times, a read every gap, and returns how long each
// answer took; a read without an answer within z2m.LostAfter is left out. It
// stops early when ctx is done.
func ReadTimes(ctx context.Context, r Reader, device string, n int, gap time.Duration) []time.Duration {
	var got []time.Duration
	for i := range n {
		start := time.Now()
		if d, ok := r.Read(ctx, device, z2m.LostAfter); ok {
			got = append(got, d)
		}
		if i < n-1 && !cli.Sleep(ctx, gap-time.Since(start)) {
			break
		}
	}
	return got
}
