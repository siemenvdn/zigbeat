package mqtt

import (
	"io"
	"testing"
)

func TestMatch(t *testing.T) {
	cases := []struct {
		filter, topic string
		want          bool
	}{
		{"zigbee2mqtt/lamp", "zigbee2mqtt/lamp", true},
		{"zigbee2mqtt/lamp", "zigbee2mqtt/lamp/set", false},
		{"zigbee2mqtt/+/set", "zigbee2mqtt/lamp/set", true},
		{"zigbee2mqtt/+/set", "zigbee2mqtt/lamp/get", false},
		{"zigbee2mqtt/#", "zigbee2mqtt/bridge/logging", true},
		{"zigbee2mqtt/bridge/logging", "zigbee2mqtt/bridge", false},
	}
	for _, c := range cases {
		if got := Match(c.filter, c.topic); got != c.want {
			t.Errorf("Match(%q, %q) = %v, want %v", c.filter, c.topic, got, c.want)
		}
	}
}

func TestVarintRoundTrip(t *testing.T) {
	for _, n := range []int{0, 127, 128, 16383, 16384, 2097151, 2097152, maxRemainingLen} {
		b := appendVarint(nil, n)
		got, err := readVarint(&byteReader{b: b})
		if err != nil || got != n {
			t.Errorf("varint %d: got %d, %v", n, got, err)
		}
	}
}

type byteReader struct {
	b []byte
	i int
}

func (r *byteReader) ReadByte() (byte, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	r.i++
	return r.b[r.i-1], nil
}
