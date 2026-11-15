package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func generate(t *testing.T, args ...string) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "k.s")
	cmd := rootCmd()
	var so bytes.Buffer
	cmd.SetOut(&so)
	cmd.SetArgs(append(args, "-o", out))
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(so.String(), "wrote") {
		t.Fatalf("output %q", so.String())
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(got)
}

// TestCommittedKernelsAreCurrent regenerates each kernel and compares it with
// the committed file, so that no .s can drift from the generator.
func TestCommittedKernelsAreCurrent(t *testing.T) {
	for arch, k := range kernels {
		got := generate(t, "-a", arch)
		want, err := os.ReadFile(filepath.Join("..", "..", k.file))
		if err != nil {
			t.Fatal(err)
		}
		if got != string(want) {
			t.Errorf("%s is stale: run go run ./internal/genasm -a %s -o %s from the module root", k.file, arch, k.file)
		}
	}
}

// The arm64 kernel does 10 rounds x 2 halves x 64 lookups, one UBFX each.
func TestArm64Lookups(t *testing.T) {
	if n := strings.Count(generate(t, "-a", "arm64"), "UBFX"); n != 1280 {
		t.Fatalf("%d UBFX, want 1280", n)
	}
}

// The scalar amd64 kernel does the same 1280 lookups as one indexed memory
// operand each: 64 initialising MOVQ and 448 XORQ per half, and no spill of
// them (the frame holds the key and the state only).
func TestAmd64Lookups(t *testing.T) {
	s := generate(t, "-a", "amd64")
	if n := strings.Count(s, ")(DI*8)") + strings.Count(s, ")(BX*8)") + strings.Count(s, ")(DX*8)"); n != 1280 {
		t.Fatalf("%d table lookups, want 1280", n)
	}
	if !strings.Contains(s, "$144-24") {
		t.Fatal("frame is not the 144 bytes of the key, the state and two pointers")
	}
}

// The vector kernel: per round and chain five matrix products, one VPERMB and
// twelve VPSHUFB (five in SubBytes, seven in MixRows), two chains, ten rounds;
// the byte reversal of the load and of the store add two VPSHUFB. Its offsets
// into vecConsts are those vec_test.go checks.
func TestAVX512Shape(t *testing.T) {
	s := generate(t, "-a", "avx512")
	for want, n := range map[string]int{
		"VGF2P8AFFINEQB.BCST": 100,
		"VPERMB":              20,
		"VPSHUFB":             242,
		"VZEROUPPER":          1,
	} {
		if got := strings.Count(s, want); got != n {
			t.Errorf("%d %s, want %d", got, want, n)
		}
	}
	for _, want := range []string{
		"hasAVX512BW", "hasAVX512VBMI", "hasGFNI",
		"VMOVDQU32 64(DX), Z26",    // perm
		"VMOVDQU32 128(DX), Z18",   // bswap
		"VMOVDQU32 192(DX), Z25",   // rot 1
		"VMOVDQU32 576(DX), Z19",   // rot 7
		"VMOVDQU32 640(DX), Z18",   // rc, round 1
		"VMOVDQU32 1216(DX), Z18",  // rc, round 10
		"VBROADCASTI32X4 1280(DX)", // box E
		"VBROADCASTI32X4 1328(DX)", // box R
		"VGF2P8AFFINEQB.BCST $0x00, 1344(DX)",
		"VGF2P8AFFINEQB.BCST $0x00, 1376(DX)",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("kernel lacks %q", want)
		}
	}
}

func TestMatOffRefusesAnUnknownConstant(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("no panic")
		}
	}()
	matOff(3)
}

func TestUnknownKernel(t *testing.T) {
	cmd := rootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-a", "mips", "-o", filepath.Join(t.TempDir(), "k.s")})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "mips") {
		t.Fatalf("error %v", err)
	}
}

func TestUnwritableOutput(t *testing.T) {
	cmd := rootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"-o", filepath.Join(t.TempDir(), "no", "such", "dir.s")})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error")
	}
}

func TestMainExitsOnError(t *testing.T) {
	old, oldArgs := osExit, os.Args
	defer func() { osExit, os.Args = old, oldArgs }()
	code := 0
	osExit = func(c int) { code = c }
	os.Args = []string{"genasm", "--no-such-flag"}
	main()
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}

func TestMainWrites(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"genasm", "-o", filepath.Join(t.TempDir(), "k.s")}
	main()
}

// With no -o the kernel goes to its own file name in the current directory.
func TestDefaultOutputName(t *testing.T) {
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	cmd := rootCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-a", "avx512"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "compress_avx512_amd64.s")); err != nil {
		t.Fatal(err)
	}
}
