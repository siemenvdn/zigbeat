package ws

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"time"
)

// The frame header of RFC 6455 5.2.
const (
	finBit  = 0x80 // first byte
	opMask  = 0x0f
	maskBit = 0x80 // second byte
	lenMask = 0x7f
	len16   = 126 // a 16-bit length follows
	len64   = 127 // a 64-bit length follows
	maskLen = 4
)

func (c *Conn) readFrame() (fin bool, op byte, payload []byte, err error) {
	var h [2]byte
	if _, err = io.ReadFull(c.r, h[:]); err != nil {
		return
	}
	fin, op = h[0]&finBit != 0, h[0]&opMask
	n := uint64(h[1] & lenMask)
	switch n {
	case len16:
		var b [2]byte
		_, err = io.ReadFull(c.r, b[:])
		n = uint64(binary.BigEndian.Uint16(b[:]))
	case len64:
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
	var mask [maskLen]byte
	if h[1]&maskBit != 0 {
		if _, err = io.ReadFull(c.r, mask[:]); err != nil {
			return
		}
	}
	payload = make([]byte, n)
	if _, err = io.ReadFull(c.r, payload); err != nil {
		return
	}
	if h[1]&maskBit != 0 {
		for i := range payload {
			payload[i] ^= mask[i%maskLen]
		}
	}
	return
}

func (c *Conn) write(op byte, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	b := []byte{finBit | op}
	var masked byte
	if c.client {
		masked = maskBit
	}
	switch n := len(payload); {
	case n < len16:
		b = append(b, masked|byte(n))
	case n <= math.MaxUint16:
		b = binary.BigEndian.AppendUint16(append(b, masked|len16), uint16(n))
	default:
		b = binary.BigEndian.AppendUint64(append(b, masked|len64), uint64(n))
	}
	start := len(b)
	if c.client {
		var mask [maskLen]byte
		_, _ = rand.Read(mask[:])
		b = append(b, mask[:]...)
		start += maskLen
		b = append(b, payload...)
		for i := range payload {
			b[start+i] ^= mask[i%maskLen]
		}
	} else {
		b = append(b, payload...)
	}
	_ = c.nc.SetWriteDeadline(time.Now().Add(writeTimeout))
	_, err := c.nc.Write(b)
	return err
}
