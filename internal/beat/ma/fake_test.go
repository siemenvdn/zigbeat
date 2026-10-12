package ma

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/siemenvdn/zigbeat/internal/ws"
)

const offset = 37 * time.Second // the fake's clock runs this far ahead

// fakeMA is a relay with its own clock. Every authenticated connection is handed to the test.
type fakeMA struct {
	base  time.Time
	conns chan *ws.Conn
	url   string
}

func (f *fakeMA) now() int64 { return time.Since(f.base).Microseconds() + offset.Microseconds() }

// local is the time on the test's clock of ts on the fake's.
func (f *fakeMA) local(ts int64) time.Time {
	return f.base.Add(time.Duration(ts)*time.Microsecond - offset)
}

func startMA(t *testing.T) *fakeMA {
	f := &fakeMA{base: time.Now(), conns: make(chan *ws.Conn, 4)}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := ws.Upgrade(w, r)
		if err != nil {
			return
		}
		m, err := c.Read()
		if err != nil || !strings.Contains(string(m.Data), `"token":"good"`) {
			c.CloseWith(4001, "Invalid or expired token")
			return
		}
		_ = c.WriteText([]byte(`{"type": "auth_ok"}`))
		if r.URL.Query().Get("player") == "nobody" {
			_ = c.WriteText([]byte(`{"type":"error","message":"no such player"}`))
			c.Close()
			return
		}
		_ = c.WriteText([]byte(`{"type":"stream/start","payload":{"visualizer":{"types":["waveform","beat"]}}}`))
		f.conns <- c
		for {
			m, err := c.Read()
			if err != nil {
				return
			}
			var req struct {
				Payload struct {
					ClientTransmitted int64 `json:"client_transmitted"`
				} `json:"payload"`
			}
			if json.Unmarshal(m.Data, &req) == nil && strings.Contains(string(m.Data), "client/time") {
				now := f.now()
				b, _ := json.Marshal(map[string]any{"type": "server/time", "payload": map[string]int64{
					"client_transmitted": req.Payload.ClientTransmitted, "server_received": now, "server_transmitted": now}})
				_ = c.WriteText(b)
			}
		}
	}))
	t.Cleanup(srv.Close)
	f.url = "ws" + strings.TrimPrefix(srv.URL, "http")
	return f
}

func beatFrame(ts int64, down bool) []byte {
	b := binary.BigEndian.AppendUint64([]byte{beatTag}, uint64(ts))
	if down {
		return append(b, 1)
	}
	return append(b, 0)
}

// syncBuf collects the notes; the source writes from its own goroutine.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuf) waitFor(t *testing.T, want string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		s.mu.Lock()
		got := s.b.String()
		s.mu.Unlock()
		if strings.Contains(got, want) {
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	t.Fatalf("notes %q lack %q", s.b.String(), want)
}
