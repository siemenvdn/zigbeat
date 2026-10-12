package ws

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"time"
)

func (c *Conn) readFrame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.r, h[:]); err != nil {
		return
	}
	fin, op = h[0]&0x80 != 0, h[0]&0x0f
	n := uint64(h[1] & 0x7f)
	switch n {
	case 126:
		var b [2]byte
		_, err = io.ReadFull(c.r, b[:])
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case 127:
		var b [8]byte
		_, err = io.ReadFull(c.r, b[:])
		n = binary.BigEndian.Uint64(b[:])
	}
	if err != nil {
		return
	}
	if n > maxMessage {
		return false, 0, nil, errors.New("ws: frame too large")
	}
	var mask [4]byte
	if h[1]&0x80 != 0 {
		if _, err = io.ReadFull(c.r, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.r, payload); err != nil {
		return
	}
	if h[1]&0x80 != 0 {
		for i := range payload {
			payload[i] ^= mask[i%4]
		}
	}
	return
}

func (c *Conn) write(op byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	b := []byte{0x80 | op}
	var maskBit byte
	if c.client {
		maskBit = 0x80
	}
	switch n := len(payload); {
	case n < 126:
		b = append(b, maskBit|byte(n))
	case n <= 0xffff:
		b = binary.BigEndian.AppendUint16(append(b, maskBit|126), uint16(n))
	default:
		b = binary.BigEndian.AppendUint64(append(b, maskBit|127), uint64(n))
	}
	start := len(b)
	if c.client {
		var mask [4]byte
		_, _ = rand.Read(mask[:])
		b = append(b, mask[:]...)
		start += 4
		b = append(b, payload...)
		for i := range payload {
			b[start+i] ^= mask[i%4]
		}
	} else {
		b = append(b, payload...)
	}
	_ = c.nc.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := c.nc.Write(b)
	return err
}
