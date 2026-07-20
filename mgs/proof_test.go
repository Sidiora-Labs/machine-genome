package mgs

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"
)

func TestW3CEdDSAJCS2022Vector(t *testing.T) {
	secret, err := decodeMultibase("z3u2en7t5LR2WtQH5PfFqMqwVHBeXouLzo6haApm8XHqvjxq")
	if err != nil {
		t.Fatal(err)
	}
	// Multikey ed25519-priv uses varint multicodec 0x1300 (0x80 0x26).
	if len(secret) != 2+ed25519.SeedSize || secret[0] != 0x80 || secret[1] != 0x26 {
		t.Fatalf("W3C secret is not an Ed25519 private Multikey")
	}
	privateKey := ed25519.NewKeyFromSeed(secret[2:])
	unsecured := []byte(`{
	  "@context": [
	    "https://www.w3.org/ns/credentials/v2",
	    "https://www.w3.org/ns/credentials/examples/v2"
	  ],
	  "id": "urn:uuid:58172aac-d8ba-11ed-83dd-0b3aef56cc33",
	  "type": ["VerifiableCredential", "AlumniCredential"],
	  "name": "Alumni Credential",
	  "description": "A minimum viable example of an Alumni Credential.",
	  "issuer": "https://vc.example/issuers/5678",
	  "validFrom": "2023-01-01T00:00:00Z",
	  "credentialSubject": {
	    "id": "did:example:abcdefgh",
	    "alumniOf": "The School of Examples"
	  }
	}`)
	options := ProofOptions{
		Type:               ProofType,
		Cryptosuite:        ProofCryptosuite,
		Created:            "2023-02-24T23:36:38Z",
		VerificationMethod: "did:key:z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2#z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2",
		ProofPurpose:       ProofPurpose,
	}
	secured, err := SignDocument(unsecured, options, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Proof Proof `json:"proof"`
	}
	if err := json.Unmarshal(secured, &result); err != nil {
		t.Fatal(err)
	}
	const want = "z2HnFSSPPBzR36zdDgK8PbEHeXbR56YF24jwMpt3R1eHXQzJDMWS93FCzpvJpwTWd3GAVFuUfjoJdcnTMuVor51aX"
	if result.Proof.ProofValue != want {
		t.Fatalf("proofValue mismatch\n got: %s\nwant: %s", result.Proof.ProofValue, want)
	}
	if _, err := VerifyDocument(context.Background(), secured, DIDKeyResolver{}); err != nil {
		t.Fatal(err)
	}
}

func TestProofTamperingFailsClosed(t *testing.T) {
	privateKey, _, method := deterministicKey(t, 9)
	raw := []byte(`{"message":"unaltered"}`)
	secured, err := SignDocument(raw, NewProofOptions(time.Unix(1_700_000_000, 0), method), privateKey)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(secured, &object); err != nil {
		t.Fatal(err)
	}
	object["message"] = "tampered"
	tampered, _ := json.Marshal(object)
	if _, err := VerifyDocument(context.Background(), tampered, DIDKeyResolver{}); err == nil {
		t.Fatal("tampered document verified")
	}
}

func deterministicKey(t *testing.T, value byte) (ed25519.PrivateKey, string, string) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	did, method, err := DIDKeyFromPublicKey(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	return privateKey, did, method
}
