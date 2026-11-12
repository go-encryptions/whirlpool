package whirlpool

import (
	"bytes"
	"encoding/hex"
	"hash"
	"os"
	"strings"
	"testing"
)

type vector struct{ in, out []byte }

// readVec parses the Botan test-vector format: "In = hex" / "Out = hex" pairs,
// '#' comments and [Section] lines ignored; the input may be empty.
func readVec(t testing.TB, path string) []vector {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vs []vector
	var in []byte
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "In ="):
			in, err = hex.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "In =")))
			if err != nil {
				t.Fatal(err)
			}
		case strings.HasPrefix(line, "Out ="):
			out, err := hex.DecodeString(strings.TrimSpace(strings.TrimPrefix(line, "Out =")))
			if err != nil {
				t.Fatal(err)
			}
			vs = append(vs, vector{in, out})
		}
	}
	return vs
}

func unhex(s string) []byte {
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		panic(err)
	}
	return b
}

// ISO/IEC 10118-3 examples, from iso-test-vectors.txt of the reference
// implementation (Barreto and Rijmen, 2003).
var iso = []struct{ in, out string }{
	{"", "19FA61D75522A466 9B44E39C1D2E1726 C530232130D407F8 9AFEE0964997F7A7 3E83BE698B288FEB CF88E3E03C4F0757 EA8964E59B63D937 08B138CC42A66EB3"},
	{"a", "8ACA2602792AEC6F 11A67206531FB7D7 F0DFF59413145E69 73C45001D0087B42 D11BC645413AEFF6 3A42391A39145A59 1A92200D560195E5 3B478584FDAE231A"},
	{"abc", "4E2448A4C6F486BB 16B6562C73B4020B F3043E3A731BCE72 1AE1B303D97E6D4C 7181EEBDB6C57E27 7D0E34957114CBD6 C797FC9D95D8B582 D225292076D4EEF5"},
	{"message digest", "378C84A4126E2DC6 E56DCC7458377AAC 838D00032230F53C E1F5700C0FFB4D3B 8421557659EF55C1 06B4B52AC5A4AAA6 92ED920052838F33 62E86DBD37A8903E"},
	{"abcdefghijklmnopqrstuvwxyz", "F1D754662636FFE9 2C82EBB9212A484A 8D38631EAD4238F5 442EE13B8054E41B 08BF2A9251C30B6A 0B8AAE86177AB4A6 F68F673E7207865D 5D9819A3DBA4EB3B"},
	{"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789", "DC37E008CF9EE69B F11F00ED9ABA2690 1DD7C28CDEC066CC 6AF42E40F82F3A1E 08EBA26629129D8F B7CB57211B9281A6 5517CC879D7B9621 42C65F5A7AF01467"},
	{strings.Repeat("1234567890", 8), "466EF18BABB0154D 25B9D38A6414F5C0 8784372BCCB204D6 549C4AFADB601429 4D5BD8DF2A6C44E5 38CD047B2681A51A 2C60481E88C5A20B 2C2A80CF3A9A083B"},
	{"abcdbcdecdefdefgefghfghighijhijk", "2A987EA40F917061 F5D6F0A0E4644F48 8A7A5A52DEEE6562 07C562F988E95C69 16BDC8031BC5BE1B 7B947639FE050B56 939BAAA0ADFF9AE6 745B7B181C3BE3FD"},
}

const millionA = "0C99005BEB57EFF5 0A7CF005560DDF5D 29057FD86B20BFD6 2DECA0F1CCEA4AF5 1FC15490EDDC47AF 32BB2B66C34FF9AD 8C6008AD677F7712 6953B226E4ED8B01"

func TestISO(t *testing.T) {
	for _, v := range iso {
		want := unhex(v.out)
		got := Sum([]byte(v.in))
		if !bytes.Equal(got[:], want) {
			t.Errorf("Sum(%q) = %x, want %x", v.in, got, want)
		}
	}
}

func TestMillionA(t *testing.T) {
	h := New()
	chunk := bytes.Repeat([]byte{'a'}, 1000)
	for i := 0; i < 1000; i++ {
		h.Write(chunk)
	}
	if got, want := h.Sum(nil), unhex(millionA); !bytes.Equal(got, want) {
		t.Fatalf("one million 'a': %x, want %x", got, want)
	}
}

func checkVectors(t *testing.T, path string, atLeast int) {
	vs := readVec(t, path)
	if len(vs) < atLeast {
		t.Fatalf("%s: %d vectors, want at least %d", path, len(vs), atLeast)
	}
	for i, v := range vs {
		got := Sum(v.in)
		if !bytes.Equal(got[:], v.out) {
			t.Errorf("%s #%d (%d bytes): %x, want %x", path, i, len(v.in), got, v.out)
		}
		// The same input fed in uneven pieces must agree.
		h := New()
		for off, step := 0, 1; off < len(v.in); off, step = off+step, step%67+1 {
			end := min(off+step, len(v.in))
			h.Write(v.in[off:end])
		}
		if got := h.Sum(nil); !bytes.Equal(got, v.out) {
			t.Errorf("%s #%d chunked: %x, want %x", path, i, got, v.out)
		}
	}
}

func TestBotanVectors(t *testing.T) { checkVectors(t, "testdata/botan-whirlpool.vec", 10) }
func TestBotanDiff(t *testing.T)    { checkVectors(t, "testdata/botan-diff.vec", 500) }

func TestHashContract(t *testing.T) {
	var h hash.Hash = New()
	if h.Size() != 64 || h.BlockSize() != 64 {
		t.Fatal("Size or BlockSize")
	}
	h.Write([]byte("abc"))
	first := h.Sum([]byte("prefix"))
	if !bytes.HasPrefix(first, []byte("prefix")) || len(first) != 6+64 {
		t.Fatal("Sum must append")
	}
	if second := h.Sum([]byte("prefix")); !bytes.Equal(first, second) {
		t.Fatal("Sum changed the state")
	}
	h.Write([]byte("def"))
	want := Sum([]byte("abcdef"))
	if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Fatal("writing after Sum")
	}
	h.Reset()
	h.Write([]byte("abc"))
	want = Sum([]byte("abc"))
	if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Fatal("Reset")
	}
}

// Every length around the padding boundaries (a block, and 32 bytes short of
// it) agrees between one-shot and byte-by-byte writes.
func TestBoundaries(t *testing.T) {
	data := make([]byte, 200)
	for i := range data {
		data[i] = byte(i * 7)
	}
	for n := 0; n <= len(data); n++ {
		want := Sum(data[:n])
		h := New()
		for i := 0; i < n; i++ {
			h.Write(data[i : i+1])
		}
		if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
			t.Fatalf("length %d: bytewise %x, one-shot %x", n, got, want)
		}
	}
}

func TestSBoxIsPermutation(t *testing.T) {
	New()
	var seen [256]bool
	for x := 0; x < 256; x++ {
		seen[byte(c[0][x]>>56)] = true // the top byte of C[0][x] is S[x]
	}
	for v, ok := range seen {
		if !ok {
			t.Fatalf("S-box misses %#x", v)
		}
	}
}

// lookup is the reference loop form of the round transformation (Barreto and
// Rijmen's Whirlpool.java), the oracle for the unrolled step.
func lookup(s *[8]uint64, i int) uint64 {
	var v uint64
	for t := 0; t < 8; t++ {
		v ^= c[t][byte(s[(i-t)&7]>>(56-8*t))]
	}
	return v
}

func TestStepMatchesReference(t *testing.T) {
	New()
	s := [8]uint64{0x0123456789abcdef, 0xfedcba9876543210, 1, 2 << 40, ^uint64(0), 0, 0x8000000000000000, 0xdeadbeefcafef00d}
	for round := 0; round < 50; round++ {
		var want [8]uint64
		for i := range want {
			want[i] = lookup(&s, i)
		}
		var got [8]uint64
		got[0], got[1], got[2], got[3], got[4], got[5], got[6], got[7] = step(&c, s[0], s[1], s[2], s[3], s[4], s[5], s[6], s[7])
		if got != want {
			t.Fatalf("round %d: step %x, reference %x", round, got, want)
		}
		s = got
	}
}

func FuzzChunking(f *testing.F) {
	f.Add([]byte(""), uint8(1))
	f.Add([]byte("abc"), uint8(2))
	f.Add(bytes.Repeat([]byte{0xff}, 130), uint8(63))
	f.Fuzz(func(t *testing.T, data []byte, step uint8) {
		want := Sum(data)
		h := New()
		s := int(step)%70 + 1
		for off := 0; off < len(data); off += s {
			h.Write(data[off:min(off+s, len(data))])
		}
		if got := h.Sum(nil); !bytes.Equal(got, want[:]) {
			t.Fatalf("chunk %d: %x vs %x", s, got, want)
		}
		if !bytes.Equal(h.Sum(nil), h.Sum(nil)) {
			t.Fatal("Sum is not idempotent")
		}
	})
}
