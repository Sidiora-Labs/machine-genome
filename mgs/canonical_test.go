package mgs

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestCanonicalizeRFC8785Example(t *testing.T) {
	raw, err := os.ReadFile("../testdata/vectors/jcs-rfc8785.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Input     string `json:"input"`
		Canonical string `json:"canonical"`
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	got, err := Canonicalize([]byte(vector.Input))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(vector.Canonical)) {
		t.Fatalf("canonical JSON mismatch\n got: %s\nwant: %s", got, vector.Canonical)
	}
}

func TestCanonicalizeRejectsAmbiguousOrInvalidJSON(t *testing.T) {
	tests := map[string][]byte{
		"duplicate member":       []byte(`{"a":1,"a":2}`),
		"negative zero integer":  []byte(`{"n":-0}`),
		"negative zero decimal":  []byte(`{"n":-0.0}`),
		"negative zero exponent": []byte(`{"n":-0e9}`),
		"unpaired high surrogate": []byte(
			`{"s":"\ud800"}`,
		),
		"unpaired low surrogate": []byte(`{"s":"\udc00"}`),
		"trailing value":         []byte(`{"a":1} {"b":2}`),
		"top-level array":        []byte(`[1,2,3]`),
		"invalid utf8":           {'{', '"', 's', '"', ':', '"', 0xff, '"', '}'},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := Canonicalize(input); err == nil {
				t.Fatal("expected canonicalization to fail")
			}
		})
	}
}

func TestCanonicalizeAcceptsSurrogatePair(t *testing.T) {
	got, err := Canonicalize([]byte(`{"s":"\ud83d\ude80"}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"s":"🚀"}` {
		t.Fatalf("got %s", got)
	}
}

func TestDigestIsIndependentOfJSONFormatting(t *testing.T) {
	first, err := Digest([]byte(`{"b":[2,3],"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Digest([]byte("{\n  \"a\": 1.0,\n  \"b\": [2, 3]\n}"))
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("equivalent JSON produced different digests: %s != %s", first, second)
	}
}
