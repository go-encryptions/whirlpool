package whirlpool

import (
	"encoding/binary"
	"errors"
	"hash"
)

// The state is serialised as the magic, the eight chaining words, the 64-byte
// block buffer (bytes past the buffered count are zero) and the byte count,
// all big-endian. The buffered count is the byte count modulo 64, because a
// block is compressed as soon as it is full.
const (
	magic         = "wrl\x01"
	marshaledSize = len(magic) + 8*8 + BlockSize + 8
)

var (
	errShort = errors.New("whirlpool: state is too short or too long")
	errMagic = errors.New("whirlpool: not a Whirlpool state")
	errState = errors.New("whirlpool: inconsistent state")
)

// MarshalBinary implements encoding.BinaryMarshaler. crypto/hmac and
// crypto/pbkdf2 use it to keep the keyed states of HMAC instead of
// recomputing them.
func (d *digest) MarshalBinary() ([]byte, error) {
	return d.AppendBinary(make([]byte, 0, marshaledSize))
}

// AppendBinary implements encoding.BinaryAppender.
func (d *digest) AppendBinary(b []byte) ([]byte, error) {
	b = append(b, magic...)
	for _, w := range d.h {
		b = binary.BigEndian.AppendUint64(b, w)
	}
	b = append(b, d.buf[:d.nbuf]...)
	b = append(b, make([]byte, BlockSize-d.nbuf)...)
	return binary.BigEndian.AppendUint64(b, d.len), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler. It refuses a state of
// the wrong length, a foreign magic, or a buffer that disagrees with the byte
// count, and leaves the receiver unchanged in that case.
func (d *digest) UnmarshalBinary(b []byte) error {
	if len(b) != marshaledSize {
		return errShort
	}
	if string(b[:len(magic)]) != magic {
		return errMagic
	}
	b = b[len(magic):]
	var e digest
	for i := range e.h {
		e.h[i] = binary.BigEndian.Uint64(b)
		b = b[8:]
	}
	buf := b[:BlockSize]
	e.len = binary.BigEndian.Uint64(b[BlockSize:])
	e.nbuf = int(e.len % BlockSize)
	for _, x := range buf[e.nbuf:] {
		if x != 0 {
			return errState
		}
	}
	copy(e.buf[:], buf[:e.nbuf])
	*d = e
	return nil
}

// Clone implements hash.Cloner: an independent copy of the running state.
func (d *digest) Clone() (hash.Cloner, error) {
	c := *d
	return &c, nil
}
