package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommittedKernelIsCurrent regenerates the kernel and compares it with the
// committed compress_arm64.s, so the .s cannot drift from the generator.
func TestCommittedKernelIsCurrent(t *testing.T) {
	out := filepath.Join(t.TempDir(), "k.s")
	cmd := rootCmd()
	var so bytes.Buffer
	cmd.SetOut(&so)
	cmd.SetArgs([]string{"-o", out})
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
	want, err := os.ReadFile(filepath.Join("..", "..", "compress_arm64.s"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("compress_arm64.s is stale: run go run ./internal/genasm -o compress_arm64.s from the module root")
	}
	// 10 rounds x 2 halves x 64 lookups, one UBFX each.
	if n := strings.Count(string(got), "UBFX"); n != 1280 {
		t.Fatalf("%d UBFX, want 1280", n)
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
