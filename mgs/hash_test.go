package mgs

import "testing"

func TestArtifactDigestUsesRawBytes(t *testing.T) {
	raw := []byte("{ \"b\": 2, \"a\": 1 }")
	canonical, err := Canonicalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := Digest(raw)
	if err != nil {
		t.Fatal(err)
	}
	if digest != ArtifactDigest(canonical) {
		t.Fatal("JSON digest is not the raw digest of its canonical bytes")
	}
	if digest == ArtifactDigest(raw) {
		t.Fatal("raw artifact digest unexpectedly canonicalized its input")
	}
	if err := ValidateDigest(ArtifactDigest([]byte("artifact"))); err != nil {
		t.Fatal(err)
	}
}
