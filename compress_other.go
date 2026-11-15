//go:build !arm64 && !amd64

package whirlpool

// compress is the Miyaguchi-Preneel step h = W_h(m) ^ h ^ m for one 64-byte
// block; only arm64 and amd64 have an assembly kernel.
func compress(h *[8]uint64, block []byte) { compressGo(h, block) }
