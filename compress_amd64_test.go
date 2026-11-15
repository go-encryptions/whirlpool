package whirlpool

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unsafe"
)

// goKernel stands in for compressAVX512, block by block with the portable
// compress, so the dispatch around the kernel runs (and is covered) on a
// processor that lacks the instructions.
func goKernel(h *[8]uint64, p *byte, n int, _ *vecConsts) {
	b := unsafe.Slice(p, n*BlockSize)
	for ; len(b) > 0; b = b[BlockSize:] {
		compressGo(h, b[:BlockSize])
	}
}

// withKernel runs f with the vector gate and kernel set, restoring both.
func withKernel(use bool, k func(*[8]uint64, *byte, int, *vecConsts), f func()) {
	u, old := useVec, vecKernel
	useVec, vecKernel = use, k
	defer func() { useVec, vecKernel = u, old }()
	f()
}

// refSum is the digest of data computed with the reference loop form of the
// compression function: padding as the standard says, no dispatch.
func refSum(data []byte) []byte {
	New()
	n := len(data)
	pad := 1 + (BlockSize-(n+1+32)%BlockSize)%BlockSize + 32
	msg := make([]byte, n+pad)
	copy(msg, data)
	msg[n] = 0x80
	binary.BigEndian.PutUint64(msg[len(msg)-8:], uint64(n)<<3)
	binary.BigEndian.PutUint64(msg[len(msg)-16:], uint64(n)>>61)
	var h [8]uint64
	for b := msg; len(b) > 0; b = b[BlockSize:] {
		compressReference(&h, b[:BlockSize])
	}
	out := make([]byte, 0, Size)
	for _, w := range h {
		out = binary.BigEndian.AppendUint64(out, w)
	}
	return out
}

func skipWithoutVec(t *testing.T) {
	t.Helper()
	if !(hasAVX512BW() && hasAVX512VBMI() && hasGFNI()) {
		t.Skip("no AVX-512 BW, VBMI and GFNI on this CPU")
	}
}

func testData(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*131 + i>>8*17 + 5)
	}
	return b
}

// The kernel on its own, from several starting states, one block and many,
// against the portable compress; n = 0 must do nothing.
func TestVecKernelMatchesPortable(t *testing.T) {
	skipWithoutVec(t)
	New()
	for _, n := range []int{1, 2, 3, 17, 64} {
		src := testData(n * BlockSize)
		for seed := uint64(0); seed < 3; seed++ {
			var got, want [8]uint64
			for i := range got {
				got[i] = seed * (0x9e3779b97f4a7c15 + uint64(i)*0x1234567)
			}
			want = got
			compressAVX512(&got, &src[0], n, &vec)
			for b := src; len(b) > 0; b = b[BlockSize:] {
				compressGo(&want, b[:BlockSize])
			}
			if got != want {
				t.Fatalf("%d blocks, seed %d: kernel %x, portable %x", n, seed, got, want)
			}
		}
	}
	var h, zero [8]uint64
	h = [8]uint64{1, 2, 3, 4, 5, 6, 7, 8}
	zero = h
	src := testData(BlockSize)
	compressAVX512(&h, &src[0], 0, &vec)
	if h != zero {
		t.Fatal("n=0 changed the state")
	}
}

// Every path agrees with the reference: the vector kernel if there is one, the
// Go stand-in behind the same dispatch, and the scalar kernel; the lengths
// reach the padding boundaries and a message of more than maxVecBlocks blocks,
// which takes several kernel calls.
func TestEveryPathAgreesWithTheReference(t *testing.T) {
	lengths := []int{0, 1, 55, 56, 63, 64, 65, 95, 96, 127, 128, 129, 1000, BlockSize*maxVecBlocks - 1, BlockSize * maxVecBlocks, BlockSize*maxVecBlocks + 70}
	paths := []struct {
		name string
		use  bool
		k    func(*[8]uint64, *byte, int, *vecConsts)
		ok   bool
	}{
		{"vector kernel", true, compressAVX512, hasAVX512BW() && hasAVX512VBMI() && hasGFNI()},
		{"stand-in kernel", true, goKernel, true},
		{"scalar kernel", false, vecKernel, true},
	}
	for _, n := range lengths {
		data := testData(n)
		want := refSum(data)
		for _, p := range paths {
			if !p.ok {
				continue
			}
			withKernel(p.use, p.k, func() {
				if got := Sum(data); !bytes.Equal(got[:], want) {
					t.Errorf("%s, %d bytes: %x, reference %x", p.name, n, got, want)
				}
				// The same data in uneven pieces.
				h := New()
				for off, step := 0, 1; off < len(data); off, step = off+step, step*3%257+1 {
					h.Write(data[off:min(off+step, len(data))])
				}
				if got := h.Sum(nil); !bytes.Equal(got, want) {
					t.Errorf("%s, %d bytes in pieces: %x, reference %x", p.name, n, got, want)
				}
			})
		}
	}
}

func TestFeatureProbesAreStable(t *testing.T) {
	a, b, c := hasAVX512BW(), hasAVX512VBMI(), hasGFNI()
	for range 50 {
		if hasAVX512BW() != a || hasAVX512VBMI() != b || hasGFNI() != c {
			t.Fatal("a feature probe changed its answer")
		}
	}
}
