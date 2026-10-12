package ws

import (
	"bufio"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

// The handshake of RFC 6455.
const (
	defaultPort = "80" // of a ws:// URL without one
	keyLen      = 16   // bytes of the Sec-WebSocket-Key nonce
	version     = "13"
	acceptGUID  = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

// StatusError is a handshake the server answered with another HTTP status.
type StatusError struct {
	Code   int
	Status string
}

func (e *StatusError) Error() string { return "ws: " + e.Status }

// Dial opens a client connection to a ws:// URL, giving up after timeout.
func Dial(rawURL string, timeout time.Duration) (*Conn, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "ws" {
		return nil, fmt.Errorf("ws: unsupported scheme %q", u.Scheme)
	}
	addr := u.Host
	if u.Port() == "" {
		addr = net.JoinHostPort(u.Hostname(), defaultPort)
	}
	nc, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, err
	}
	_ = nc.SetDeadline(time.Now().Add(timeout))
	var k [keyLen]byte
	_, _ = rand.Read(k[:])
	key := base64.StdEncoding.EncodeToString(k[:])
	_, err = io.WriteString(nc, "GET "+u.RequestURI()+" HTTP/1.1\r\nHost: "+u.Host+
		"\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Key: "+key+
		"\r\nSec-WebSocket-Version: "+version+"\r\n\r\n")
	if err != nil {
		nc.Close()
		return nil, err
	}
	r := bufio.NewReader(nc)
	resp, err := http.ReadResponse(r, &http.Request{Method: http.MethodGet})
	if err != nil {
		nc.Close()
		return nil, err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		nc.Close()
		return nil, &StatusError{Code: resp.StatusCode, Status: resp.Status}
	}
	if resp.Header.Get("Sec-WebSocket-Accept") != acceptKey(key) {
		nc.Close()
		return nil, fmt.Errorf("ws: wrong Sec-WebSocket-Accept")
	}
	_ = nc.SetDeadline(time.Time{})
	return &Conn{nc: nc, r: r, client: true}, nil
}

// acceptKey is the server's answer to a handshake key.
func acceptKey(key string) string {
	h := sha1.Sum([]byte(key + acceptGUID))
	return base64.StdEncoding.EncodeToString(h[:])
}
