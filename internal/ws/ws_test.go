package ws

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testTimeout = 2 * time.Second

// pair returns both ends of a connection through a real HTTP handshake.
func pair(t *testing.T) (client, server *Conn) {
	t.Helper()
	servers := make(chan *Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := Upgrade(w, r)
		if err != nil {
			t.Error(err)
			return
		}
		servers <- c
	}))
	t.Cleanup(srv.Close)
	client, err := Dial("ws"+strings.TrimPrefix(srv.URL, "http"), testTimeout)
	if err != nil {
		t.Fatal(err)
	}
	server = <-servers
	t.Cleanup(func() { client.Close(); server.Close() })
	return client, server
}

func TestLengthsAndTypes(t *testing.T) {
	c, s := pair(t)
	for _, n := range []int{0, 125, 126, 65535, 65536, 1 << 20} {
		data := bytes.Repeat([]byte{'x'}, n)
		go s.WriteBinary(data)
		if m, err := c.Read(); err != nil || !m.Binary || !bytes.Equal(m.Data, data) {
			t.Fatalf("%d bytes to the client: binary %v, %d bytes, %v", n, m.Binary, len(m.Data), err)
		}
		go c.WriteText(data)
		if m, err := s.Read(); err != nil || m.Binary || !bytes.Equal(m.Data, data) {
			t.Fatalf("%d bytes to the server: binary %v, %d bytes, %v", n, m.Binary, len(m.Data), err)
		}
	}
}

func TestWritesFromManyGoroutines(t *testing.T) {
	c, s := pair(t)
	const n = 50
	for range n {
		go c.WriteText([]byte("hi"))
	}
	for i := range n {
		if m, err := s.Read(); err != nil || string(m.Data) != "hi" {
			t.Fatalf("message %d: %q, %v", i, m.Data, err)
		}
	}
}

func TestPingIsAnsweredNotRead(t *testing.T) {
	c, s := pair(t)
	go func() {
		_ = s.Ping()
		_ = s.WriteText([]byte("after"))
	}()
	if m, err := c.Read(); err != nil || string(m.Data) != "after" {
		t.Fatalf("got %q, %v", m.Data, err)
	}
}

func TestCloseError(t *testing.T) {
	c, s := pair(t)
	go s.CloseWith(4001, "Invalid or expired token")
	_, err := c.Read()
	var ce *CloseError
	if !errors.As(err, &ce) || ce.Code != 4001 || ce.Reason != "Invalid or expired token" {
		t.Errorf("got %v", err)
	}
}

func TestDialAndUpgrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/milkdrop_visualizer" {
			http.NotFound(w, r)
			return
		}
		conn, err := Upgrade(w, r)
		if err != nil {
			return
		}
		defer conn.Close()
		m, err := conn.Read()
		if err != nil {
			return
		}
		_ = conn.WriteText([]byte(r.URL.Query().Get("player") + ": " + string(m.Data)))
	}))
	defer srv.Close()
	base := "ws" + strings.TrimPrefix(srv.URL, "http")

	c, err := Dial(base+"/milkdrop_visualizer?player=lamp+1", testTimeout)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.WriteText([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if m, err := c.Read(); err != nil || string(m.Data) != "lamp 1: hi" {
		t.Errorf("got %q, %v", m.Data, err)
	}
	var se *StatusError
	if _, err := Dial(base+"/elsewhere", testTimeout); !errors.As(err, &se) || se.Code != http.StatusNotFound {
		t.Errorf("a plain 404 must fail the dial with its status, got %v", err)
	}
	if _, err := Dial("wss://"+srv.Listener.Addr().String(), testTimeout); err == nil {
		t.Error("wss is not supported")
	}
}

func TestIdleTimeoutRenewsPerMessage(t *testing.T) {
	c, s := pair(t)
	c.SetIdleTimeout(100 * time.Millisecond)
	go func() {
		for range 4 {
			time.Sleep(60 * time.Millisecond)
			_ = s.WriteText([]byte("beat"))
		}
	}()
	for i := range 4 {
		if m, err := c.Read(); err != nil || string(m.Data) != "beat" {
			t.Fatalf("messages 60 ms apart keep a 100 ms idle timeout alive: message %d: %q, %v", i, m.Data, err)
		}
	}
	_ = c.SetReadDeadline(time.Now().Add(testTimeout)) // only a bound: the idle timeout wins
	start := time.Now()
	if _, err := c.Read(); err == nil || time.Since(start) > testTimeout/2 {
		t.Errorf("silence must end the read within the idle timeout: %v after %s", err, time.Since(start))
	}
}

func TestReadDeadline(t *testing.T) {
	c, _ := pair(t)
	_ = c.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	start := time.Now()
	if _, err := c.Read(); err == nil || time.Since(start) > testTimeout {
		t.Errorf("a passed deadline must end the read: %v after %s", err, time.Since(start))
	}
}
