package whirlpool

import (
	"testing"
	"unsafe"
)

// The five lookups of subByte must be the S-box: this is the construction the
// kernel runs sixty-four bytes at a time.
func TestSubByteIsTheSBox(t *testing.T) {
	New()
	for x := 0; x < 256; x++ {
		if got, want := subByte(byte(x)), byte(c[0][x]>>56); got != want {
			t.Fatalf("subByte(%#02x) = %#02x, S-box %#02x", x, got, want)
		}
	}
}

// Multiplication by each constant of the circulant, as a bit matrix and as the
// software model of VGF2P8AFFINEQB, against the shift-and-add product, for
// every byte. The matrices are also checked against the table entries: C[0][x]
// holds S[x] times 1, 1, 4, 1, 8, 5, 2, 9.
func TestAffineMatrices(t *testing.T) {
	New()
	for _, k := range []byte{1, 2, 4, 5, 8, 9} {
		m := affineMatrix(k)
		for x := 0; x < 256; x++ {
			if got, want := affine(m, byte(x)), gfMul(k, byte(x)); got != want {
				t.Fatalf("matrix of %d on %#02x: %#02x, product %#02x", k, x, got, want)
			}
		}
	}
	if affineMatrix(1) != 0x0102040810204080 {
		t.Fatalf("identity matrix %#016x", affineMatrix(1))
	}
	for x := 0; x < 256; x++ {
		s := byte(c[0][x] >> 56)
		var want uint64
		for i, k := range []byte{1, 1, 4, 1, 8, 5, 2, 9} {
			want |= uint64(gfMul(k, s)) << (56 - 8*i)
		}
		if c[0][x] != want {
			t.Fatalf("C[0][%#02x] = %#016x, circulant row of S[x] is %#016x", x, c[0][x], want)
		}
	}
	if gfMul(0x80, 2) != 0x1d || gfMul(2, 0x80) != 0x1d {
		t.Fatal("the reduction polynomial is not 0x11d")
	}
}

// The kernel addresses vecConsts by constant offsets (genasm's avx512.go); a
// moved field would make it read the wrong constants and still compile.
func TestVecLayout(t *testing.T) {
	var v vecConsts
	for _, f := range []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"lo", unsafe.Offsetof(v.lo), 0},
		{"perm", unsafe.Offsetof(v.perm), 64},
		{"bswap", unsafe.Offsetof(v.bswap), 128},
		{"rot", unsafe.Offsetof(v.rot), 192},
		{"rc", unsafe.Offsetof(v.rc), 640},
		{"box", unsafe.Offsetof(v.box), 1280},
		{"mat", unsafe.Offsetof(v.mat), 1344},
		{"size", unsafe.Sizeof(v), 1384},
	} {
		if f.got != f.want {
			t.Errorf("vecConsts.%s at %d, the kernel expects %d", f.name, f.got, f.want)
		}
	}
}

func TestVecConstants(t *testing.T) {
	New()
	// perm is a permutation, and puts row (r-c)&7 of column c in row r.
	var seen [64]bool
	for _, p := range vec.perm {
		seen[p] = true
	}
	for i, ok := range seen {
		if !ok {
			t.Fatalf("perm misses %d", i)
		}
	}
	if vec.perm[1*8+0] != 1*8+0 || vec.perm[0*8+1] != 7*8+1 || vec.perm[7*8+7] != 0*8+7 {
		t.Fatalf("perm %v", vec.perm)
	}
	// bswap reverses each quadword, rot[k-1] moves byte j to j+k inside it.
	for i := 0; i < 64; i++ {
		if int(vec.bswap[i]) != i&^7+7-i&7 {
			t.Fatalf("bswap[%d] = %d", i, vec.bswap[i])
		}
		for k := 1; k <= 7; k++ {
			if int(vec.rot[k-1][i]) != i&^7+(i&7-k+8)&7 {
				t.Fatalf("rot[%d][%d] = %d", k-1, i, vec.rot[k-1][i])
			}
		}
		if vec.lo[i] != 0x0f {
			t.Fatalf("lo[%d] = %#02x", i, vec.lo[i])
		}
	}
	// The round constants are row 0, columns in order, zero elsewhere.
	for r := 1; r <= rounds; r++ {
		for i, b := range vec.rc[r-1] {
			want := byte(0)
			if i < 8 {
				want = byte(rc[r] >> (56 - 8*i))
			}
			if b != want {
				t.Fatalf("rc[%d][%d] = %#02x, want %#02x", r, i, b, want)
			}
		}
	}
	for i := 0; i < 16; i++ {
		if vec.box[1][i] != vec.box[0][i]<<4 {
			t.Fatalf("E<<4 box differs at %d", i)
		}
	}
	for i, k := range []byte{2, 4, 5, 8, 9} {
		if vec.mat[i] != affineMatrix(k) {
			t.Fatalf("mat[%d] is not the matrix of %d", i, k)
		}
	}
}
