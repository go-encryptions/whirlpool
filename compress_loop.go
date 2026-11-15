//go:build !amd64

package whirlpool

// compressBlocks runs the whole blocks of p (a multiple of BlockSize bytes)
// through compress; only amd64 has a kernel that takes several blocks.
func compressBlocks(h *[8]uint64, p []byte) {
	for len(p) > 0 {
		compress(h, p[:BlockSize])
		p = p[BlockSize:]
	}
}
