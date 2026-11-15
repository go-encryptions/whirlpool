package whirlpool

import "encoding/binary"

// vecConsts are the constants of the AVX-512 kernel (compress_avx512_amd64.s):
// byte shuffles, the round constants as 64-byte vectors, the nibble boxes and
// the bit matrices of the multiplications. The layout is the kernel's; the
// offsets are in vec_test.go. It is built on every architecture, from the
// same arithmetic as the tables, so that the construction is tested
// everywhere even though only amd64 runs the kernel.
type vecConsts struct {
	lo    [64]byte // 0x0f in every byte
	perm  [64]byte // ShiftColumns as VPERMB indices
	bswap [64]byte // byte reversal inside each quadword
	rot   [7][64]byte
	rc    [rounds][64]byte
	box   [4][16]byte // E, E<<4, E^-1, R
	mat   [5]uint64   // multiplication by 2, 4, 5, 8, 9
}

// The three 4-bit boxes the S-box is built from (ISO/IEC 10118-3, Whirlpool:
// the mini-boxes E, E^-1 and R).
var (
	miniE  = [16]byte{1, 11, 9, 12, 13, 6, 15, 3, 14, 8, 7, 4, 10, 2, 5, 0}
	miniEi = [16]byte{15, 0, 13, 7, 11, 14, 5, 10, 9, 2, 12, 1, 3, 4, 8, 6}
	miniR  = [16]byte{7, 12, 11, 13, 14, 4, 9, 15, 6, 3, 8, 10, 2, 5, 1, 0}
)

// subByte is the S-box as the kernel computes it: five lookups in the mini
// boxes on the nibbles of x.
func subByte(x byte) byte {
	l := miniE[x>>4]
	r := miniEi[x&15]
	t := miniR[l^r]
	return miniE[l^t]<<4 | miniEi[r^t]
}

// gfMul multiplies in GF(2^8) modulo x^8 + x^4 + x^3 + x^2 + 1 (0x11d).
func gfMul(a, b byte) byte {
	var p uint64
	v := uint64(a)
	for ; b != 0; b >>= 1 {
		if b&1 != 0 {
			p ^= v
		}
		v = mulx(v)
	}
	return byte(p)
}

// affineMatrix is the 8x8 bit matrix, in the operand layout of
// VGF2P8AFFINEQB, that multiplies every byte by the constant k: output bit j
// is the parity of byte 7-j of the matrix ANDed with the input byte, so that
// byte holds, at bit i, bit j of k times x^i.
func affineMatrix(k byte) uint64 {
	var m uint64
	for j := 0; j < 8; j++ {
		var row byte
		for i := 0; i < 8; i++ {
			row |= (gfMul(k, 1<<i) >> j & 1) << i
		}
		m |= uint64(row) << (8 * (7 - j))
	}
	return m
}

// affine is VGF2P8AFFINEQB with imm 0 on one byte, in software.
func affine(m uint64, x byte) byte {
	var out byte
	for j := 0; j < 8; j++ {
		row := byte(m >> (8 * (7 - j)))
		v := row & x
		v ^= v >> 4
		v ^= v >> 2
		v ^= v >> 1
		out |= (v & 1) << j
	}
	return out
}

func (v *vecConsts) build() {
	for i := range v.lo {
		v.lo[i] = 0x0f
		// Row r, column c of the result is row (r-c)&7, column c of the input.
		r, c := i/8, i%8
		v.perm[i] = byte((r-c+8)%8*8 + c)
		v.bswap[i] = byte(i&^7 + 7 - i&7)
	}
	for k := 1; k <= 7; k++ {
		for i := range v.rot[k-1] {
			// Byte j of a quadword moves to j+k: the control for byte j is j-k.
			v.rot[k-1][i] = byte(i&^7 + (i&7-k+8)&7)
		}
	}
	for r := 1; r <= rounds; r++ {
		binary.BigEndian.PutUint64(v.rc[r-1][:], rc[r])
	}
	for i := range 16 {
		v.box[0][i] = miniE[i]
		v.box[1][i] = miniE[i] << 4
		v.box[2][i] = miniEi[i]
		v.box[3][i] = miniR[i]
	}
	for i, k := range [5]byte{2, 4, 5, 8, 9} {
		v.mat[i] = affineMatrix(k)
	}
}
