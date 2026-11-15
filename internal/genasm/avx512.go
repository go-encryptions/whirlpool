package main

import (
	"fmt"

	"github.com/go-asmgen/asmgen/amd64"
	"github.com/go-asmgen/asmgen/emit"
)

// The AVX-512 kernel holds the state of a block, a 8x8 matrix of bytes with
// row r at bytes 8r..8r+7 of a ZMM register (byte c of a row is column c, the
// most significant byte of the row's big-endian word first), and applies the
// round to it as Botan does: SubBytes with sixteen-entry boxes on nibbles
// (VPSHUFB), ShiftColumns as one VPERMB, MixRows as the circulant
// cir(1, 1, 4, 1, 8, 5, 2, 9) over GF(2^8) mod 0x11d. Two things differ from
// Botan's kernel. The multiples 2, 4, 5, 8 and 9 of the state are one
// VGF2P8AFFINEQB each (multiplication by a constant is linear over GF(2), so
// it is an 8x8 bit matrix whatever the polynomial) instead of three chained
// doublings with a mask blend, and the three-input XORs are VPTERNLOGD.
//
// The key and the state are two independent chains; the state takes the key of
// the same round only in the last instruction, so the core overlaps them.
// Constants come from the vecConsts of the Go side (vec.go); the layout below
// must match it, and vec_test.go checks the offsets.
const (
	vcLo    = 0    // 0x0f in every byte
	vcPerm  = 64   // ShiftColumns, as VPERMB indices
	vcBswap = 128  // byte reversal inside each quadword
	vcRot   = 192  // seven masks, rotation by 1..7 bytes inside each quadword
	vcRC    = 640  // ten round constants, row 0 of a block-sized vector
	vcBox   = 1280 // E, E<<4, E^-1, R: four 16-byte boxes
	vcMat   = 1344 // the matrices of 2, 4, 5, 8, 9
)

// Registers. Z0..Z7 are the key chain (state Z0), Z8..Z15 the state chain.
const (
	zH    = amd64.Z16
	zM    = amd64.Z17
	zRC   = amd64.Z18
	zBswp = amd64.Z18 // shares Z18 with the round constant: needed only outside the rounds
	zPerm = amd64.Z26
	zLo   = amd64.Z31
	zE    = amd64.Z30
	zE4   = amd64.Z29
	zEi   = amd64.Z28
	zR    = amd64.Z27
)

// zRot is the register holding the rotation by k bytes, 1 <= k <= 7: Z25..Z19.
func zRot(k int) amd64.ZReg { return amd64.ZReg(26 - k) }

// matrixOrder lists the constants whose matrices follow each other in vcMat.
var matrixOrder = [5]int{2, 4, 5, 8, 9}

func matOff(c int) int {
	for i, m := range matrixOrder {
		if m == c {
			return vcMat + 8*i
		}
	}
	panic("no matrix for constant")
}

// chain returns the instructions of one round on the state in x, using the
// seven registers t[0..6] and ending with x = round(x) ^ y.
func chain(b *amd64.Builder, x amd64.ZReg, t [7]amd64.ZReg, y amd64.ZReg) []func() {
	lo, hi, l, r, tt := t[0], t[1], t[2], t[3], t[4]
	r1, r3 := t[0], t[1]
	var ops []func()
	add := func(f func()) { ops = append(ops, f) }
	// SubBytes: L = E[hi], R = E^-1[lo], T = R[L^R], then
	// out = E[L^T] << 4 | E^-1[R^T], with the shift folded into the box E4.
	add(func() { b.VPANDD(lo, x, zLo) })
	add(func() { b.VPSRLW512(hi, x, 4) })
	add(func() { b.VPANDD(hi, hi, zLo) })
	add(func() { b.VPSHUFB512(l, zE, hi) })
	add(func() { b.VPSHUFB512(r, zEi, lo) })
	add(func() { b.VPXORD(tt, l, r) })
	add(func() { b.VPSHUFB512(tt, zR, tt) })
	add(func() { b.VPXORD(l, l, tt) })
	add(func() { b.VPXORD(r, r, tt) })
	add(func() { b.VPSHUFB512(l, zE4, l) })
	add(func() { b.VPSHUFB512(r, zEi, r) })
	add(func() { b.VPORD(x, l, r) })
	// ShiftColumns.
	add(func() { b.VPERMB(x, zPerm, x) })
	// MixRows: out = x ^ rot1(x) ^ rot3(x) ^ rot2(4x) ^ rot4(8x) ^ rot5(5x) ^
	// rot6(2x) ^ rot7(9x), the row r of the result being the row r of x times
	// the circulant.
	add(func() { b.VPSHUFB512(r1, x, zRot(1)) })
	add(func() { b.VPSHUFB512(r3, x, zRot(3)) })
	for i, c := range []int{4, 8, 5, 2, 9} {
		reg := t[2+i]
		add(func() { b.VGF2P8AFFINEQBBcst(0, reg, x, fmt.Sprintf("%d(DX)", matOff(c))) })
	}
	for i, k := range []int{2, 4, 5, 6, 7} {
		reg := t[2+i]
		add(func() { b.VPSHUFB512(reg, reg, zRot(k)) })
	}
	add(func() { b.VPTERNLOGD(0x96, x, r1, t[2]) })
	add(func() { b.VPTERNLOGD(0x96, r3, t[3], t[4]) })
	add(func() { b.VPTERNLOGD(0x96, t[5], t[6], r3) })
	add(func() { b.VPTERNLOGD(0x96, x, t[5], y) })
	return ops
}

// buildAVX512 returns the amd64 AVX-512 assembly text.
func buildAVX512() string {
	sig := amd64.Layout(
		[]string{"h", "p", "n", "c"},
		[]amd64.Type{amd64.Int64, amd64.Int64, amd64.Int64, amd64.Int64},
		nil, nil,
	)
	b := amd64.NewFuncFlags("compressAVX512", sig, 0, "NOSPLIT")
	b.LoadArg("h", "DI").LoadArg("p", "SI").LoadArg("n", "CX").LoadArg("c", "DX")
	b.Raw("TESTQ CX, CX").Raw("JZ done")
	b.VMOVDQU32(zLo, fmt.Sprintf("%d(DX)", vcLo))
	b.VMOVDQU32(zPerm, fmt.Sprintf("%d(DX)", vcPerm))
	b.VMOVDQU32(zBswp, fmt.Sprintf("%d(DX)", vcBswap))
	for k := 1; k <= 7; k++ {
		b.VMOVDQU32(zRot(k), fmt.Sprintf("%d(DX)", vcRot+64*(k-1)))
	}
	for i, z := range []amd64.ZReg{zE, zE4, zEi, zR} {
		b.VBROADCASTI32X4(z, fmt.Sprintf("%d(DX)", vcBox+16*i))
	}
	b.VMOVDQU32(zH, "(DI)")
	b.VPSHUFB512(zH, zH, zBswp)
	b.Label("block")
	b.VMOVDQU32(zM, "(SI)")
	b.VMOVDQA32(amd64.Z0, zH)
	b.VPXORD(amd64.Z8, zH, zM)
	kt := [7]amd64.ZReg{amd64.Z1, amd64.Z2, amd64.Z3, amd64.Z4, amd64.Z5, amd64.Z6, amd64.Z7}
	bt := [7]amd64.ZReg{amd64.Z9, amd64.Z10, amd64.Z11, amd64.Z12, amd64.Z13, amd64.Z14, amd64.Z15}
	for r := 0; r < rounds; r++ {
		b.VMOVDQU32(zRC, fmt.Sprintf("%d(DX)", vcRC+64*r))
		k := chain(b, amd64.Z0, kt, zRC)
		s := chain(b, amd64.Z8, bt, amd64.Z0)
		for i := range k {
			k[i]()
			s[i]()
		}
	}
	// h ^= S ^ m.
	b.VPTERNLOGD(0x96, zH, zM, amd64.Z8)
	b.Raw("ADDQ $64, SI").Raw("DECQ CX").Raw("JNZ block")
	b.VMOVDQU32(zBswp, fmt.Sprintf("%d(DX)", vcBswap))
	b.VPSHUFB512(zH, zH, zBswp)
	b.VMOVDQU32Store("(DI)", zH)
	b.Label("done")
	b.VZEROUPPER()
	b.Ret()
	f := emit.NewFile("amd64")
	f.Add(b.Func())
	f.Add(amd64.FeatureProbe("hasAVX512BW", amd64.AVX512BW))
	f.Add(amd64.FeatureProbe("hasAVX512VBMI", amd64.AVX512VBMI))
	f.Add(amd64.FeatureProbe("hasGFNI", amd64.GFNI))
	return f.String()
}
