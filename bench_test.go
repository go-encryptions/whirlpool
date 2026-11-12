package whirlpool

import (
	"crypto/pbkdf2"
	"testing"
)

// BenchmarkPBKDF2HMAC is the VeraCrypt shape: PBKDF2-HMAC, 64-byte output,
// 1000 iterations. crypto/hmac saves the keyed inner and outer states only if
// the hash marshals its state, which halves the compressions per iteration.
func BenchmarkPBKDF2HMAC(b *testing.B) {
	salt := make([]byte, 64)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := pbkdf2.Key(New, "password", salt, 1000, 64); err != nil {
			b.Fatal(err)
		}
	}
}

func benchSum(b *testing.B, size int) {
	buf := make([]byte, size)
	h := New()
	out := make([]byte, 0, Size)
	b.SetBytes(int64(size))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Reset()
		h.Write(buf)
		out = h.Sum(out[:0])
	}
}

func BenchmarkSum1K(b *testing.B)  { benchSum(b, 1024) }
func BenchmarkSum64K(b *testing.B) { benchSum(b, 65536) }
