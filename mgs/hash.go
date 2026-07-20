package mgs

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
)

const (
	sha2256MultihashCode = byte(0x12)
	sha2256DigestLength  = byte(0x20)
)

var ErrInvalidMultihash = errors.New("invalid MGS SHA-256 multihash")

// Digest returns multibase(base58btc(multihash(sha2-256, SHA-256(JCS(raw))))).
func Digest(raw []byte) (string, error) {
	canonical, err := Canonicalize(raw)
	if err != nil {
		return "", err
	}
	return ArtifactDigest(canonical), nil
}

// ArtifactDigest returns the MGS multibase SHA-256 commitment for arbitrary
// bytes. Unlike Digest, it does not interpret or canonicalize its input.
func ArtifactDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	multihash := make([]byte, 2+len(sum))
	multihash[0] = sha2256MultihashCode
	multihash[1] = sha2256DigestLength
	copy(multihash[2:], sum[:])
	return encodeMultibase(multihash)
}

func ValidateDigest(value string) error {
	decoded, err := decodeMultibase(value)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMultihash, err)
	}
	if len(decoded) != 34 || decoded[0] != sha2256MultihashCode || decoded[1] != sha2256DigestLength {
		return fmt.Errorf("%w: expected 0x12 0x20 plus 32 digest bytes", ErrInvalidMultihash)
	}
	return nil
}

func DigestMatches(raw []byte, expected string) error {
	if err := ValidateDigest(expected); err != nil {
		return err
	}
	actual, err := Digest(raw)
	if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) != 1 {
		return fmt.Errorf("content digest mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// Gene derives the stable identifier for a complete, secured genesis record.
func Gene(genesis []byte) (string, error) {
	var header struct {
		MGS        string         `json:"mgs"`
		RecordType string         `json:"recordType"`
		Proof      map[string]any `json:"proof"`
	}
	if _, err := Canonicalize(genesis); err != nil {
		return "", err
	}
	if err := unmarshal(genesis, &header); err != nil {
		return "", err
	}
	if header.RecordType != RecordGenesis {
		return "", fmt.Errorf("gene input recordType is %q, want %q", header.RecordType, RecordGenesis)
	}
	if len(header.Proof) == 0 {
		return "", errors.New("gene requires a secured genesis with proof")
	}
	return Digest(genesis)
}
