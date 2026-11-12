package whirlpool

// step is lookup unrolled for all eight output words: output word i is the
// XOR over t of c[t][byte (7-t) of input word (i-t) mod 8].
func step(c *[8][256]uint64, s0, s1, s2, s3, s4, s5, s6, s7 uint64) (uint64, uint64, uint64, uint64, uint64, uint64, uint64, uint64) {
	r0 := c[0][byte(s0>>56)] ^ c[1][byte(s7>>48)] ^ c[2][byte(s6>>40)] ^ c[3][byte(s5>>32)] ^ c[4][byte(s4>>24)] ^ c[5][byte(s3>>16)] ^ c[6][byte(s2>>8)] ^ c[7][byte(s1)]
	r1 := c[0][byte(s1>>56)] ^ c[1][byte(s0>>48)] ^ c[2][byte(s7>>40)] ^ c[3][byte(s6>>32)] ^ c[4][byte(s5>>24)] ^ c[5][byte(s4>>16)] ^ c[6][byte(s3>>8)] ^ c[7][byte(s2)]
	r2 := c[0][byte(s2>>56)] ^ c[1][byte(s1>>48)] ^ c[2][byte(s0>>40)] ^ c[3][byte(s7>>32)] ^ c[4][byte(s6>>24)] ^ c[5][byte(s5>>16)] ^ c[6][byte(s4>>8)] ^ c[7][byte(s3)]
	r3 := c[0][byte(s3>>56)] ^ c[1][byte(s2>>48)] ^ c[2][byte(s1>>40)] ^ c[3][byte(s0>>32)] ^ c[4][byte(s7>>24)] ^ c[5][byte(s6>>16)] ^ c[6][byte(s5>>8)] ^ c[7][byte(s4)]
	r4 := c[0][byte(s4>>56)] ^ c[1][byte(s3>>48)] ^ c[2][byte(s2>>40)] ^ c[3][byte(s1>>32)] ^ c[4][byte(s0>>24)] ^ c[5][byte(s7>>16)] ^ c[6][byte(s6>>8)] ^ c[7][byte(s5)]
	r5 := c[0][byte(s5>>56)] ^ c[1][byte(s4>>48)] ^ c[2][byte(s3>>40)] ^ c[3][byte(s2>>32)] ^ c[4][byte(s1>>24)] ^ c[5][byte(s0>>16)] ^ c[6][byte(s7>>8)] ^ c[7][byte(s6)]
	r6 := c[0][byte(s6>>56)] ^ c[1][byte(s5>>48)] ^ c[2][byte(s4>>40)] ^ c[3][byte(s3>>32)] ^ c[4][byte(s2>>24)] ^ c[5][byte(s1>>16)] ^ c[6][byte(s0>>8)] ^ c[7][byte(s7)]
	r7 := c[0][byte(s7>>56)] ^ c[1][byte(s6>>48)] ^ c[2][byte(s5>>40)] ^ c[3][byte(s4>>32)] ^ c[4][byte(s3>>24)] ^ c[5][byte(s2>>16)] ^ c[6][byte(s1>>8)] ^ c[7][byte(s0)]
	return r0, r1, r2, r3, r4, r5, r6, r7
}
