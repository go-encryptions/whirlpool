// Package whirlpool implements the Whirlpool hash function (ISO/IEC 10118-3,
// 512-bit digest) in pure Go: the final 2003 version, with the tweaked S-box
// and the diffusion matrix cir(1, 1, 4, 1, 8, 5, 2, 9) of Barreto and Rijmen.
//
// It is a plain table-driven implementation. The tables are indexed by secret
// data, so it is not constant-time; do not use it where a local cache-timing
// attacker matters.
package whirlpool

import (
	"encoding/binary"
	"encoding/hex"
	"hash"
	"sync"
)

const (
	// Size is the digest size in bytes.
	Size = 64
	// BlockSize is the block size in bytes.
	BlockSize = 64

	rounds = 10
)

// sboxHex is the 8-bit S-box of the final Whirlpool, 256 bytes, as printed
// in Barreto and Rijmen's reference implementation (Whirlpool.java, 2003).
const sboxHex = "1823c6e887b8014f36a6d2f5796f9152" +
	"60bc9b8ea30c7b351de0d7c22e4bfe57" +
	"157737e59ff04ada58c9290ab1a06b85" +
	"bd5d10f4cb3e0567e427418ba77d95d8" +
	"fbee7c66dd17479eca2dbf07ad5a8333" +
	"6302aa71c81949d9f2e35b889a2632b0" +
	"e90fd580becd3448ff7a905f20681aae" +
	"b454932264f173124008c3ecdba18d3d" +
	"9700cf2b7682d61bb5af6a5045f330ef" +
	"3f55a2ea65ba2fc0de1cfd4d9275068a" +
	"b2e60e1f62d4a896f9c525598472394c" +
	"5e78388cd1a5e261b3219c1e43c7fc04" +
	"51996d0dfadf7e243babce118f4eb7eb" +
	"3c8194f7b9132cd3e76ec40356447fa9" +
	"2abbc153dc0b9d6c3174f646ac8914e1" +
	"163a690970b6d0edcc4298a4285cf886"

var (
	tableOnce sync.Once
	// c[t][x] is S[x] times the circulant row [1 1 4 1 8 5 2 9] rotated right
	// by t bytes.
	c  [8][256]uint64
	rc [rounds + 1]uint64
)

func mulx(v uint64) uint64 {
	v <<= 1
	if v >= 0x100 {
		v ^= 0x11d
	}
	return v
}

func buildTables() {
	var sbox [256]byte
	// The constant is fixed, so Decode cannot fail on it (a test checks that
	// it yields 256 distinct bytes).
	hex.Decode(sbox[:], []byte(sboxHex))
	for x := 0; x < 256; x++ {
		v1 := uint64(sbox[x])
		v2 := mulx(v1)
		v4 := mulx(v2)
		v5 := v4 ^ v1
		v8 := mulx(v4)
		v9 := v8 ^ v1
		c[0][x] = v1<<56 | v1<<48 | v4<<40 | v1<<32 | v8<<24 | v5<<16 | v2<<8 | v9
		for t := 1; t < 8; t++ {
			c[t][x] = c[t-1][x]>>8 | c[t-1][x]<<56
		}
	}
	for r := 1; r <= rounds; r++ {
		i := 8 * (r - 1)
		rc[r] = c[0][i]&0xff00000000000000 ^
			c[1][i+1]&0x00ff000000000000 ^
			c[2][i+2]&0x0000ff0000000000 ^
			c[3][i+3]&0x000000ff00000000 ^
			c[4][i+4]&0x00000000ff000000 ^
			c[5][i+5]&0x0000000000ff0000 ^
			c[6][i+6]&0x000000000000ff00 ^
			c[7][i+7]&0x00000000000000ff
	}
}

// compress is the Miyaguchi-Preneel step h = W_h(m) ^ h ^ m. The round
// transformation (step) is the combined SubBytes, ShiftColumns and MixRows
// of the reference implementation, unrolled; whirlpool_test.go keeps the
// reference loop form as an oracle.
func compress(h *[8]uint64, block []byte) {
	m0 := binary.BigEndian.Uint64(block[0:])
	m1 := binary.BigEndian.Uint64(block[8:])
	m2 := binary.BigEndian.Uint64(block[16:])
	m3 := binary.BigEndian.Uint64(block[24:])
	m4 := binary.BigEndian.Uint64(block[32:])
	m5 := binary.BigEndian.Uint64(block[40:])
	m6 := binary.BigEndian.Uint64(block[48:])
	m7 := binary.BigEndian.Uint64(block[56:])
	k0, k1, k2, k3, k4, k5, k6, k7 := h[0], h[1], h[2], h[3], h[4], h[5], h[6], h[7]
	s0, s1, s2, s3, s4, s5, s6, s7 := m0^k0, m1^k1, m2^k2, m3^k3, m4^k4, m5^k5, m6^k6, m7^k7
	for r := 1; r <= rounds; r++ {
		k0, k1, k2, k3, k4, k5, k6, k7 = step(&c, k0, k1, k2, k3, k4, k5, k6, k7)
		k0 ^= rc[r]
		s0, s1, s2, s3, s4, s5, s6, s7 = step(&c, s0, s1, s2, s3, s4, s5, s6, s7)
		s0, s1, s2, s3, s4, s5, s6, s7 = s0^k0, s1^k1, s2^k2, s3^k3, s4^k4, s5^k5, s6^k6, s7^k7
	}
	h[0] ^= s0 ^ m0
	h[1] ^= s1 ^ m1
	h[2] ^= s2 ^ m2
	h[3] ^= s3 ^ m3
	h[4] ^= s4 ^ m4
	h[5] ^= s5 ^ m5
	h[6] ^= s6 ^ m6
	h[7] ^= s7 ^ m7
}

type digest struct {
	h    [8]uint64
	buf  [BlockSize]byte
	nbuf int
	len  uint64 // bytes written
}

// New returns a hash.Hash computing the Whirlpool-512 digest.
func New() hash.Hash {
	tableOnce.Do(buildTables)
	return &digest{}
}

func (d *digest) Size() int      { return Size }
func (d *digest) BlockSize() int { return BlockSize }

func (d *digest) Reset() {
	d.h = [8]uint64{}
	d.nbuf = 0
	d.len = 0
}

func (d *digest) Write(p []byte) (int, error) {
	n := len(p)
	d.len += uint64(n)
	if d.nbuf > 0 {
		k := copy(d.buf[d.nbuf:], p)
		d.nbuf += k
		p = p[k:]
		if d.nbuf < BlockSize {
			return n, nil
		}
		compress(&d.h, d.buf[:])
		d.nbuf = 0
	}
	for len(p) >= BlockSize {
		compress(&d.h, p[:BlockSize])
		p = p[BlockSize:]
	}
	d.nbuf = copy(d.buf[:], p)
	return n, nil
}

// Sum appends the digest of the data written so far to b without changing the
// state. Padding is 0x80, zeros up to 32 bytes short of a block boundary, then
// the message length in bits as a 256-bit big-endian integer.
func (d *digest) Sum(b []byte) []byte {
	e := *d
	var pad [BlockSize + 32]byte
	pad[0] = 0x80
	n := BlockSize - e.nbuf
	if n < 33 {
		n += BlockSize
	}
	// n counts the padding bytes including the 32-byte length field.
	lenOff := n - 32
	binary.BigEndian.PutUint64(pad[lenOff+16:], e.len>>61)
	binary.BigEndian.PutUint64(pad[lenOff+24:], e.len<<3)
	// Write would grow e.len, which is not used afterwards.
	e.Write(pad[:n])
	for _, w := range e.h {
		b = binary.BigEndian.AppendUint64(b, w)
	}
	return b
}

// Sum returns the Whirlpool-512 digest of data.
func Sum(data []byte) [Size]byte {
	d := New()
	d.Write(data)
	var out [Size]byte
	d.Sum(out[:0])
	return out
}
