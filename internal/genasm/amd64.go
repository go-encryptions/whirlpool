package main

import (
	"github.com/go-asmgen/asmgen/amd64"
	"github.com/go-asmgen/asmgen/emit"
)

// The amd64 kernel works one source word at a time. Word w of the key (or of
// the state) contributes table t of its byte 7-t to row (w+t)&7, so the eight
// output words are eight accumulators in registers (R8 to R15) and every
// lookup is one XORQ with a memory operand: the table base in SI, the table
// number in the displacement, the byte as the scaled index. A source word
// sits in AX or CX, whose AL, AH, CL and CH give two bytes without a shift;
// one SHRQ $16 reaches the next two. That is 20 instructions per source word
// and no spill in the round, where the Go compiler keeps the 128 byte
// extractions of a round live in a 1.7 KiB frame.
//
// The key and the state live on the stack between the two halves of a round.
// The key half of round r+1 depends on the key half of round r only, so the
// out-of-order core overlaps it with the state half.
const (
	amd64Frame = 144
	amd64K     = 0   // key words: 64 bytes at 0(SP)
	amd64S     = 64  // state words: 64 bytes
	amd64H     = 128 // saved h pointer
	amd64P     = 136 // saved block pointer
)

var (
	accs   = [8]string{"R8", "R9", "R10", "R11", "R12", "R13", "R14", "R15"}
	words  = [2][3]string{{"AX", "AL", "AH"}, {"CX", "CL", "CH"}}
	idxReg = [3]string{"DI", "BX", "DX"}
)

type amd64Gen struct {
	b   *amd64.Builder
	idx int // index registers handed out, to rotate them
}

// half computes accs[i] = the XOR over w, t of table t at byte 7-t of the
// word w of slot, for the row i = (w+t)&7.
func (g *amd64Gen) half(slot int) {
	for w := 0; w < 8; w++ {
		reg := words[w%2]
		g.b.Raw("MOVQ %d(SP), %s", slot+8*w, reg[0])
		for p := 0; p < 4; p++ {
			for k := 0; k < 2; k++ {
				j := 2*p + k // byte j counted from the least significant
				t := 7 - j
				x := idxReg[g.idx%len(idxReg)]
				g.idx++
				g.b.Raw("MOVBLZX %s, %s", reg[1+k], x)
				op := "XORQ"
				if w == 0 {
					op = "MOVQ"
				}
				g.b.Raw("%s %d(SI)(%s*8), %s", op, 2048*t, x, accs[(w+t)&7])
			}
			if p < 3 {
				g.b.Raw("SHRQ $16, %s", reg[0])
			}
		}
	}
}

// store writes the accumulators to slot.
func (g *amd64Gen) store(slot int) {
	for i, a := range accs {
		g.b.Raw("MOVQ %s, %d(SP)", a, slot+8*i)
	}
}

// buildAmd64 returns the amd64 assembly text.
func buildAmd64() string {
	sig := amd64.Layout(
		[]string{"h", "p", "t"}, []amd64.Type{amd64.Int64, amd64.Int64, amd64.Int64},
		nil, nil,
	)
	g := &amd64Gen{b: amd64.NewFuncFlags("compressAsm", sig, amd64Frame, "NOSPLIT")}
	b := g.b
	b.LoadArg("h", "DI").LoadArg("p", "BX").LoadArg("t", "SI")
	b.Raw("MOVQ DI, %d(SP)", amd64H).Raw("MOVQ BX, %d(SP)", amd64P)
	// K = h; S = m ^ h, m read big-endian.
	for i := 0; i < 8; i++ {
		b.Raw("MOVQ %d(DI), AX", 8*i)
		b.Raw("MOVQ AX, %d(SP)", amd64K+8*i)
		b.Raw("MOVQ %d(BX), CX", 8*i)
		b.Raw("BSWAPQ CX")
		b.Raw("XORQ AX, CX")
		b.Raw("MOVQ CX, %d(SP)", amd64S+8*i)
	}
	for r := 1; r <= rounds; r++ {
		g.half(amd64K)
		b.Raw("XORQ %d(SI), %s", tabSize+8*r, accs[0])
		g.store(amd64K)
		g.half(amd64S)
		for i, a := range accs {
			b.Raw("XORQ %d(SP), %s", amd64K+8*i, a)
		}
		if r < rounds {
			g.store(amd64S)
		}
	}
	// h ^= S ^ m.
	b.Raw("MOVQ %d(SP), DI", amd64H).Raw("MOVQ %d(SP), BX", amd64P)
	for i, a := range accs {
		b.Raw("MOVQ %d(BX), AX", 8*i)
		b.Raw("BSWAPQ AX")
		b.Raw("XORQ AX, %s", a)
		b.Raw("XORQ %d(DI), %s", 8*i, a)
		b.Raw("MOVQ %s, %d(DI)", a, 8*i)
	}
	b.Ret()
	f := emit.NewFile("amd64")
	f.Add(b.Func())
	return f.String()
}
