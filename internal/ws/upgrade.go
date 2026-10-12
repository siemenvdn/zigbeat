package ws

import (
	"errors"
	"net/http"
	"strings"
)

// Upgrade answers a client's handshake and returns the server side.
func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	key := r.Header.Get("Sec-WebSocket-Key")
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") || key == "" {
		http.Error(w, "websocket handshake expected", http.StatusBadRequest)
		return nil, errors.New("ws: not a websocket handshake")
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("ws: the response cannot be hijacked")
	}
	nc, rw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}
	_, err = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: " +
		acceptKey(key) + "\r\n\r\n")
	if err == nil {
		err = rw.Flush()
	}
	if err != nil {
		nc.Close()
		return nil, err
	}
	return &Conn{nc: nc, r: rw.Reader}, nil
}
