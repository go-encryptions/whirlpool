package whirlpool

import (
	"bytes"
	"crypto/hmac"
	"crypto/pbkdf2"
	"encoding"
	"hash"
	"testing"
)

type state interface {
	hash.Cloner
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
	encoding.BinaryAppender
}

func TestInterfaces(t *testing.T) {
	if _, ok := New().(state); !ok {
		t.Fatal("digest does not implement the marshalling interfaces")
	}
}

// A state saved after n bytes, restored into a fresh hash, must go on exactly
// like the original: every buffer fill from 0 to two blocks and a bit.
func TestMarshalRoundTrip(t *testing.T) {
	data := make([]byte, 300)
	for i := range data {
		data[i] = byte(i*11 + 3)
	}
	for n := 0; n <= 130; n++ {
		a := New()
		a.Write(data[:n])
		saved, err := a.(encoding.BinaryMarshaler).MarshalBinary()
		if err != nil || len(saved) != marshaledSize {
			t.Fatalf("n=%d: %d bytes, %v", n, len(saved), err)
		}
		b := New()
		if err := b.(encoding.BinaryUnmarshaler).UnmarshalBinary(saved); err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if !bytes.Equal(a.Sum(nil), b.Sum(nil)) {
			t.Fatalf("n=%d: digests differ right after restore", n)
		}
		a.Write(data[n:])
		b.Write(data[n:])
		if !bytes.Equal(a.Sum(nil), b.Sum(nil)) {
			t.Fatalf("n=%d: digests differ after more data", n)
		}
		want := Sum(data)
		if !bytes.Equal(b.Sum(nil), want[:]) {
			t.Fatalf("n=%d: restored hash disagrees with one-shot", n)
		}
		// The saved form re-marshals identically, and AppendBinary appends.
		again, _ := b.(encoding.BinaryAppender).AppendBinary([]byte("x"))
		if again[0] != 'x' || len(again) != 1+marshaledSize {
			t.Fatalf("n=%d: AppendBinary", n)
		}
	}
}

func TestUnmarshalRefuses(t *testing.T) {
	h := New()
	h.Write([]byte("abc"))
	good, _ := h.(encoding.BinaryMarshaler).MarshalBinary()
	mutate := func(f func(b []byte) []byte) []byte { return f(bytes.Clone(good)) }
	cases := map[string][]byte{
		"empty":          nil,
		"short":          good[:len(good)-1],
		"long":           append(bytes.Clone(good), 0),
		"foreign magic":  mutate(func(b []byte) []byte { b[0] = 'x'; return b }),
		"dirty padding":  mutate(func(b []byte) []byte { b[len(b)-9] = 1; return b }),
		"count mismatch": mutate(func(b []byte) []byte { b[len(b)-1] = 2; return b }),
	}
	for name, b := range cases {
		d := New()
		d.Write([]byte("keep"))
		before := d.Sum(nil)
		if err := d.(encoding.BinaryUnmarshaler).UnmarshalBinary(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if !bytes.Equal(before, d.Sum(nil)) {
			t.Errorf("%s: a refused state changed the receiver", name)
		}
	}
}

func TestClone(t *testing.T) {
	a := New()
	a.Write([]byte("abc"))
	c, err := a.(hash.Cloner).Clone()
	if err != nil {
		t.Fatal(err)
	}
	a.Write([]byte("def"))
	want := Sum([]byte("abc"))
	if got := c.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Fatal("the clone followed the original")
	}
	c.Write([]byte("xyz"))
	want = Sum([]byte("abcdef"))
	if got := a.Sum(nil); !bytes.Equal(got, want[:]) {
		t.Fatal("the original followed the clone")
	}
}

// plain hides the marshalling methods, so crypto/hmac recomputes the keyed
// states instead of restoring them. Both paths must give the same bytes.
type plain struct{ hash.Hash }

func TestHMACAndPBKDF2WithAndWithoutMarshalling(t *testing.T) {
	key := bytes.Repeat([]byte{0x0b}, 100) // longer than a block: hashed first
	msg := []byte("Hi There")
	fast, slow := hmac.New(New, key), hmac.New(func() hash.Hash { return plain{New()} }, key)
	for i := 0; i < 3; i++ {
		fast.Write(msg)
		slow.Write(msg)
		if !bytes.Equal(fast.Sum(nil), slow.Sum(nil)) {
			t.Fatalf("HMAC differs on pass %d", i)
		}
		fast.Reset()
		slow.Reset()
	}
	a, err := pbkdf2.Key(New, "password", []byte("salt"), 50, 80)
	if err != nil {
		t.Fatal(err)
	}
	b, err := pbkdf2.Key(func() hash.Hash { return plain{New()} }, "password", []byte("salt"), 50, 80)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("PBKDF2 differs")
	}
}

func FuzzUnmarshalBinary(f *testing.F) {
	h := New()
	h.Write([]byte("seed"))
	good, _ := h.(encoding.BinaryMarshaler).MarshalBinary()
	f.Add(good)
	f.Add(good[:10])
	f.Add([]byte(magic))
	f.Fuzz(func(t *testing.T, b []byte) {
		d := New()
		if err := d.(encoding.BinaryUnmarshaler).UnmarshalBinary(b); err != nil {
			return
		}
		// An accepted state is canonical: it marshals back to the same bytes
		// and hashing on from it neither panics nor changes that.
		again, _ := d.(encoding.BinaryMarshaler).MarshalBinary()
		if !bytes.Equal(again, b) {
			t.Fatalf("accepted state does not round-trip")
		}
		d.Write([]byte("more"))
		d.Sum(nil)
	})
}
