package whirlpool

import "encoding/binary"

// compress is the Miyaguchi-Preneel step h = W_h(m) ^ h ^ m. The round
// transformation is the combined SubBytes, ShiftColumns and MixRows of the
// reference implementation, written out for all eight words of the key and of
// the state inside the round loop. It was a separate step function called
// twice per round; the call forced sixteen live words through the stack and
// kept the two independent chains apart, and inlining it is 14 % faster on
// arm64. whirlpool_test.go keeps the reference loop form as an oracle.
func compressGo(h *[8]uint64, block []byte) {
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
		n0 := c[0][byte(k0>>56)] ^ c[1][byte(k7>>48)] ^ c[2][byte(k6>>40)] ^ c[3][byte(k5>>32)] ^ c[4][byte(k4>>24)] ^ c[5][byte(k3>>16)] ^ c[6][byte(k2>>8)] ^ c[7][byte(k1)] ^ rc[r]
		n1 := c[0][byte(k1>>56)] ^ c[1][byte(k0>>48)] ^ c[2][byte(k7>>40)] ^ c[3][byte(k6>>32)] ^ c[4][byte(k5>>24)] ^ c[5][byte(k4>>16)] ^ c[6][byte(k3>>8)] ^ c[7][byte(k2)]
		n2 := c[0][byte(k2>>56)] ^ c[1][byte(k1>>48)] ^ c[2][byte(k0>>40)] ^ c[3][byte(k7>>32)] ^ c[4][byte(k6>>24)] ^ c[5][byte(k5>>16)] ^ c[6][byte(k4>>8)] ^ c[7][byte(k3)]
		n3 := c[0][byte(k3>>56)] ^ c[1][byte(k2>>48)] ^ c[2][byte(k1>>40)] ^ c[3][byte(k0>>32)] ^ c[4][byte(k7>>24)] ^ c[5][byte(k6>>16)] ^ c[6][byte(k5>>8)] ^ c[7][byte(k4)]
		n4 := c[0][byte(k4>>56)] ^ c[1][byte(k3>>48)] ^ c[2][byte(k2>>40)] ^ c[3][byte(k1>>32)] ^ c[4][byte(k0>>24)] ^ c[5][byte(k7>>16)] ^ c[6][byte(k6>>8)] ^ c[7][byte(k5)]
		n5 := c[0][byte(k5>>56)] ^ c[1][byte(k4>>48)] ^ c[2][byte(k3>>40)] ^ c[3][byte(k2>>32)] ^ c[4][byte(k1>>24)] ^ c[5][byte(k0>>16)] ^ c[6][byte(k7>>8)] ^ c[7][byte(k6)]
		n6 := c[0][byte(k6>>56)] ^ c[1][byte(k5>>48)] ^ c[2][byte(k4>>40)] ^ c[3][byte(k3>>32)] ^ c[4][byte(k2>>24)] ^ c[5][byte(k1>>16)] ^ c[6][byte(k0>>8)] ^ c[7][byte(k7)]
		n7 := c[0][byte(k7>>56)] ^ c[1][byte(k6>>48)] ^ c[2][byte(k5>>40)] ^ c[3][byte(k4>>32)] ^ c[4][byte(k3>>24)] ^ c[5][byte(k2>>16)] ^ c[6][byte(k1>>8)] ^ c[7][byte(k0)]
		k0, k1, k2, k3, k4, k5, k6, k7 = n0, n1, n2, n3, n4, n5, n6, n7
		t0 := c[0][byte(s0>>56)] ^ c[1][byte(s7>>48)] ^ c[2][byte(s6>>40)] ^ c[3][byte(s5>>32)] ^ c[4][byte(s4>>24)] ^ c[5][byte(s3>>16)] ^ c[6][byte(s2>>8)] ^ c[7][byte(s1)] ^ k0
		t1 := c[0][byte(s1>>56)] ^ c[1][byte(s0>>48)] ^ c[2][byte(s7>>40)] ^ c[3][byte(s6>>32)] ^ c[4][byte(s5>>24)] ^ c[5][byte(s4>>16)] ^ c[6][byte(s3>>8)] ^ c[7][byte(s2)] ^ k1
		t2 := c[0][byte(s2>>56)] ^ c[1][byte(s1>>48)] ^ c[2][byte(s0>>40)] ^ c[3][byte(s7>>32)] ^ c[4][byte(s6>>24)] ^ c[5][byte(s5>>16)] ^ c[6][byte(s4>>8)] ^ c[7][byte(s3)] ^ k2
		t3 := c[0][byte(s3>>56)] ^ c[1][byte(s2>>48)] ^ c[2][byte(s1>>40)] ^ c[3][byte(s0>>32)] ^ c[4][byte(s7>>24)] ^ c[5][byte(s6>>16)] ^ c[6][byte(s5>>8)] ^ c[7][byte(s4)] ^ k3
		t4 := c[0][byte(s4>>56)] ^ c[1][byte(s3>>48)] ^ c[2][byte(s2>>40)] ^ c[3][byte(s1>>32)] ^ c[4][byte(s0>>24)] ^ c[5][byte(s7>>16)] ^ c[6][byte(s6>>8)] ^ c[7][byte(s5)] ^ k4
		t5 := c[0][byte(s5>>56)] ^ c[1][byte(s4>>48)] ^ c[2][byte(s3>>40)] ^ c[3][byte(s2>>32)] ^ c[4][byte(s1>>24)] ^ c[5][byte(s0>>16)] ^ c[6][byte(s7>>8)] ^ c[7][byte(s6)] ^ k5
		t6 := c[0][byte(s6>>56)] ^ c[1][byte(s5>>48)] ^ c[2][byte(s4>>40)] ^ c[3][byte(s3>>32)] ^ c[4][byte(s2>>24)] ^ c[5][byte(s1>>16)] ^ c[6][byte(s0>>8)] ^ c[7][byte(s7)] ^ k6
		t7 := c[0][byte(s7>>56)] ^ c[1][byte(s6>>48)] ^ c[2][byte(s5>>40)] ^ c[3][byte(s4>>32)] ^ c[4][byte(s3>>24)] ^ c[5][byte(s2>>16)] ^ c[6][byte(s1>>8)] ^ c[7][byte(s0)] ^ k7
		s0, s1, s2, s3, s4, s5, s6, s7 = t0, t1, t2, t3, t4, t5, t6, t7
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
