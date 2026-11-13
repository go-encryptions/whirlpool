// Command genasm writes compress_arm64.s, the arm64 kernel of the Whirlpool
// compression function, with go-asmgen.
//
// The Go compiler keeps all 128 byte extractions of a round live and spills
// them (a 1.7 KiB frame, seen with go tool objdump), so the lookups share the
// load ports with the spills. This kernel keeps the eight table bases, the
// eight words being read and the eight being built in registers (24 of the 26
// that Go assembly may use) and holds the key and the state on the stack
// between the two halves of a round. Each lookup is one UBFX, one indexed load
// and one EOR.
//
// Usage: go run . [-o compress_arm64.s]
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/arm64"
	"github.com/go-asmgen/asmgen/emit"
	"github.com/spf13/cobra"
)

var osExit = os.Exit

const (
	rounds  = 10
	tabSize = 8 * 256 * 8 // bytes of the eight tables; rc follows them
	frame   = 160
	kSlot   = 8   // key words: 64 bytes at 8(RSP)
	sSlot   = 72  // state words: 64 bytes
	hSlot   = 136 // saved h pointer
	pSlot   = 144 // saved block pointer
)

var (
	base = [8]string{"R2", "R3", "R4", "R5", "R6", "R7", "R8", "R9"}
	in   = [8]string{"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17"}
	out  = [8]string{"R19", "R20", "R21", "R22", "R23", "R24", "R25", "R26"}
	tmp  = [2]string{"R0", "R1"}
)

type gen struct {
	b *arm64.Builder
	n int // lookups emitted, to alternate the temporaries
}

// load reads eight words from slot into in.
func (g *gen) load(slot int) {
	for i := 0; i < 8; i += 2 {
		g.b.Raw("LDP %d(RSP), (%s, %s)", slot+8*i, in[i], in[i+1])
	}
}

// store writes out to slot.
func (g *gen) store(slot int) {
	for i := 0; i < 8; i += 2 {
		g.b.Raw("STP (%s, %s), %d(RSP)", out[i], out[i+1], slot+8*i)
	}
}

// half computes out[i] ^= (or, with first, = ) the eight table words of row i
// of in: term t is table t at byte 7-t of in[(i-t)&7].
func (g *gen) half(first bool, rcOff int) {
	for i := 0; i < 8; i++ {
		for t := 0; t < 8; t++ {
			x := tmp[g.n%2]
			g.n++
			g.b.Raw("UBFX $%d, %s, $8, %s", 56-8*t, in[(i-t)&7], x)
			if first && t == 0 {
				g.b.Raw("MOVD (%s)(%s<<3), %s", base[t], x, out[i])
				continue
			}
			g.b.Raw("MOVD (%s)(%s<<3), %s", base[t], x, x)
			g.b.Raw("EOR %s, %s, %s", x, out[i], out[i])
		}
		if first && i == 0 {
			g.b.Raw("MOVD %d(%s), %s", rcOff, base[0], tmp[0])
			g.b.Raw("EOR %s, %s, %s", tmp[0], out[0], out[0])
		}
	}
}

// build returns the assembly text.
func build() string {
	sig := arm64.Layout(
		[]string{"h", "p", "t"}, []arm64.Type{arm64.Int64, arm64.Int64, arm64.Int64},
		nil, nil,
	)
	g := &gen{b: arm64.NewFuncFlags("compressAsm", sig, frame, "NOSPLIT")}
	b := g.b
	b.LoadArg("h", "R0").LoadArg("p", "R1").LoadArg("t", "R2")
	b.Raw("MOVD R0, %d(RSP)", hSlot).Raw("MOVD R1, %d(RSP)", pSlot)
	// K = h; S = m ^ h, m read big-endian.
	for i := 0; i < 8; i++ {
		b.Raw("MOVD %d(R0), %s", 8*i, in[i])
		b.Raw("MOVD %d(R1), %s", 8*i, out[i])
		b.Raw("REV %s, %s", out[i], out[i])
		b.Raw("EOR %s, %s, %s", in[i], out[i], out[i])
	}
	for i := 0; i < 8; i += 2 {
		b.Raw("STP (%s, %s), %d(RSP)", in[i], in[i+1], kSlot+8*i)
		b.Raw("STP (%s, %s), %d(RSP)", out[i], out[i+1], sSlot+8*i)
	}
	for t := 1; t < 8; t++ {
		b.Raw("ADD $%d, %s, %s", 2048*t, base[0], base[t])
	}
	for r := 1; r <= rounds; r++ {
		g.load(kSlot)
		g.half(true, tabSize+8*r)
		g.store(kSlot)
		g.load(sSlot)
		g.half(false, 0)
		if r < rounds {
			g.store(sSlot)
		}
	}
	// h ^= S ^ m.
	b.Raw("MOVD %d(RSP), R1", pSlot).Raw("MOVD %d(RSP), R0", hSlot)
	for i := 0; i < 8; i++ {
		b.Raw("MOVD %d(R1), %s", 8*i, in[i])
		b.Raw("REV %s, %s", in[i], in[i])
		b.Raw("EOR %s, %s, %s", in[i], out[i], out[i])
		b.Raw("MOVD %d(R0), %s", 8*i, in[i])
		b.Raw("EOR %s, %s, %s", in[i], out[i], out[i])
		b.Raw("MOVD %s, %d(R0)", out[i], 8*i)
	}
	b.Ret()
	f := emit.NewFile("arm64")
	f.Add(b.Func())
	return f.String()
}

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		osExit(1)
	}
}

func rootCmd() *cobra.Command {
	var o string
	cmd := &cobra.Command{
		Use:          "genasm",
		Short:        "write the arm64 Whirlpool kernel",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := os.WriteFile(o, []byte(build()), 0o644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "wrote", o)
			return nil
		},
	}
	cmd.Flags().StringVarP(&o, "output", "o", "compress_arm64.s", "output file")
	return cmd
}
