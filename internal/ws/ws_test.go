package ws

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testTimeout = 2 * time.Second

// pipe returns a client and a server joined in memory.
func pipe() (client, server *Conn) {
	a, b := net.Pipe()
	return &Conn{nc: a, r: bufio.NewReader(a), client: true}, &Conn{nc: b, r: bufio.NewReader(b)}
}

func TestAcceptKey(t *testing.T) {
	// The example from RFC 6455, section 1.3.
	if got := acceptKey("dGhlIHNhbXBsZSBub25jZQ=="); got != "s3pPLMBiTxaQ9kYGzzhZRbK+xOo=" {
		t.Errorf("accept key %q", got)
	}
}

func TestClientMasksServerDoesNot(t *testing.T) {
	c, s := pipe()
	go c.WriteText([]byte("hi"))
	raw := make([]byte, 8)
	if _, err := io.ReadFull(s.nc, raw); err != nil {
		t.Fatal(err)
	}
	if raw[0] != 0x81 || raw[1] != 0x82 || raw[6]^raw[2] != 'h' || raw[7]^raw[3] != 'i' {
		t.Errorf("client frame % x", raw)
	}
	go s.WriteText([]byte("hi"))
	raw = make([]byte, 4)
	if _, err := io.ReadFull(c.nc, raw); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, []byte{0x81, 0x02, 'h', 'i'}) {
		t.Errorf("server frame % x", raw)
	}
}

func TestLengthsAndTypes(t *testing.T) {
	c, s := pipe()
	for _, n := range []int{0, 125, 126, 65535, 65536} {
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

func TestFragmentedMessage(t *testing.T) {
	c, s := pipe()
	go func() {
		_, _ = s.nc.Write([]byte{0x01, 2, 'a', 'b'}) // text, not final
		_, _ = s.nc.Write([]byte{0x80, 2, 'c', 'd'}) // continuation, final
	}()
	if m, err := c.Read(); err != nil || m.Binary || string(m.Data) != "abcd" {
		t.Errorf("got %q, %v", m.Data, err)
	}
}

func TestPingIsAnswered(t *testing.T) {
	c, s := pipe()
	go func() {
		_ = s.Ping()
		_ = s.WriteText([]byte("after"))
	}()
	pong := make(chan byte, 1)
	go func() {
		_, op, _, _ := s.readFrame()
		pong <- op
	}()
	if m, err := c.Read(); err != nil || string(m.Data) != "after" {
		t.Fatalf("got %q, %v", m.Data, err)
	}
	if op := <-pong; op != opPong {
		t.Errorf("answer opcode %d, want a pong", op)
	}
}

func TestCloseError(t *testing.T) {
	c, s := pipe()
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
	if _, err := Dial(base+"/elsewhere", testTimeout); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a plain 404 must fail the dial, got %v", err)
	}
	if _, err := Dial("wss://"+srv.Listener.Addr().String(), testTimeout); err == nil {
		t.Error("wss is not supported")
	}
}
