package mgs

import (
	"bytes"
	"testing"
)

func FuzzCanonicalizeIsStable(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"a":1,"b":[true,null,"text"]}`),
		[]byte(`{"unicode":"\ud83d\ude80","number":1e-7}`),
		[]byte(`{"nested":{"z":0,"a":-12.5}}`),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		canonical, err := Canonicalize(raw)
		if err != nil {
			return
		}
		again, err := Canonicalize(canonical)
		if err != nil {
			t.Fatalf("canonical output was rejected: %v", err)
		}
		if !bytes.Equal(canonical, again) {
			t.Fatalf("canonicalization is not idempotent:\n%s\n%s", canonical, again)
		}
		first, err := Digest(raw)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Digest(canonical)
		if err != nil {
			t.Fatal(err)
		}
		if first != second {
			t.Fatalf("digest changed after canonicalization: %s != %s", first, second)
		}
	})
}
