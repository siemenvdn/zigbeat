package mqtt

import (
	"encoding/binary"
	"errors"
	"io"
	"time"
)

func (c *Client) write(header byte, body []byte) error {
	if len(body) > maxRemainingLen {
		return errors.New("packet too large")
	}
	pkt := make([]byte, 0, len(body)+5)
	pkt = append(pkt, header)
	pkt = appendVarint(pkt, len(body))
	pkt = append(pkt, body...)
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err := c.conn.Write(pkt)
	if err != nil {
		c.shutdown(err)
	}
	return err
}

func appendString(b []byte, s string) []byte {
	b = binary.BigEndian.AppendUint16(b, uint16(len(s)))
	return append(b, s...)
}

func appendVarint(b []byte, n int) []byte {
	for {
		d := byte(n % 128)
		n /= 128
		if n > 0 {
			d |= 0x80
		}
		b = append(b, d)
		if n == 0 {
			return b
		}
	}
}

func readVarint(r io.ByteReader) (int, error) {
	n, mul := 0, 1
	for i := 0; i < 4; i++ {
		d, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		n += int(d&0x7f) * mul
		if d&0x80 == 0 {
			return n, nil
		}
		mul *= 128
	}
	return 0, errors.New("malformed remaining length")
}
