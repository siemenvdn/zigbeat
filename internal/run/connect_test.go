package run

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestConnectStopsOnCtrlC(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err == nil {
			t.Cleanup(func() { c.Close() })
		}
	}()
	cfg := testConfig("separate")
	cfg.MQTT.Broker = ln.Addr().String()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := connect(ctx, cfg); err == nil {
		t.Fatal("connected to a broker that never answered")
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("connect took %s after Ctrl-C", d)
	}
}
