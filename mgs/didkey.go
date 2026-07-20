package mgs

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
)

var ErrUnsupportedDID = errors.New("unsupported DID verification method")

// KeyResolver returns the Ed25519 public key bound to a verification method.
// Network-backed DID methods should be implemented in separate, policy-bounded
// adapters; the core resolver is deliberately offline.
type KeyResolver interface {
	ResolveEd25519(context.Context, string) (ed25519.PublicKey, error)
}

type DIDKeyResolver struct{}

func DIDKeyFromPublicKey(publicKey ed25519.PublicKey) (did, verificationMethod string, err error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return "", "", fmt.Errorf("Ed25519 public key has %d bytes, want %d", len(publicKey), ed25519.PublicKeySize)
	}
	// Multicodec ed25519-pub is varint 0xed, encoded as 0xed 0x01.
	multikey := encodeMultibase(append([]byte{0xed, 0x01}, publicKey...))
	did = "did:key:" + multikey
	return did, did + "#" + multikey, nil
}

func (DIDKeyResolver) ResolveEd25519(_ context.Context, verificationMethod string) (ed25519.PublicKey, error) {
	controller, fragment, ok := strings.Cut(verificationMethod, "#")
	if !ok || controller == "" || fragment == "" {
		return nil, fmt.Errorf("%w: did:key verification method requires #fragment", ErrUnsupportedDID)
	}
	if !strings.HasPrefix(controller, "did:key:") {
		return nil, fmt.Errorf("%w: only did:key is available offline", ErrUnsupportedDID)
	}
	multikey := strings.TrimPrefix(controller, "did:key:")
	if fragment != multikey {
		return nil, fmt.Errorf("%w: did:key fragment does not match method-specific identifier", ErrUnsupportedDID)
	}
	decoded, err := decodeMultibase(multikey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnsupportedDID, err)
	}
	if len(decoded) != 2+ed25519.PublicKeySize || decoded[0] != 0xed || decoded[1] != 0x01 {
		return nil, fmt.Errorf("%w: expected Ed25519 multikey", ErrUnsupportedDID)
	}
	return ed25519.PublicKey(append([]byte(nil), decoded[2:]...)), nil
}

func verificationController(verificationMethod string) (string, error) {
	controller, _, ok := strings.Cut(verificationMethod, "#")
	if !ok || controller == "" {
		return "", errors.New("verificationMethod must be an absolute DID URL with a fragment")
	}
	if !strings.HasPrefix(controller, "did:") {
		return "", errors.New("verificationMethod controller is not a DID")
	}
	return controller, nil
}
