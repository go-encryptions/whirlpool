package whirlpool

import "testing"

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
