package mgs

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"reflect"
	"time"
)

const (
	ProofType        = "DataIntegrityProof"
	ProofCryptosuite = "eddsa-jcs-2022"
	ProofPurpose     = "assertionMethod"
)

var ErrInvalidProof = errors.New("invalid MGS Data Integrity proof")

type Proof struct {
	Type               string `json:"type"`
	Cryptosuite        string `json:"cryptosuite"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue,omitempty"`
}

type ProofOptions struct {
	Type               string `json:"type"`
	Cryptosuite        string `json:"cryptosuite"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
}

func NewProofOptions(created time.Time, verificationMethod string) ProofOptions {
	return ProofOptions{
		Type:               ProofType,
		Cryptosuite:        ProofCryptosuite,
		Created:            created.UTC().Truncate(time.Second).Format(time.RFC3339),
		VerificationMethod: verificationMethod,
		ProofPurpose:       ProofPurpose,
	}
}

func (options ProofOptions) validate() error {
	if options.Type != ProofType {
		return fmt.Errorf("%w: type is %q, want %q", ErrInvalidProof, options.Type, ProofType)
	}
	if options.Cryptosuite != ProofCryptosuite {
		return fmt.Errorf("%w: cryptosuite is %q, want %q", ErrInvalidProof, options.Cryptosuite, ProofCryptosuite)
	}
	if options.ProofPurpose != ProofPurpose {
		return fmt.Errorf("%w: proofPurpose is %q, want %q", ErrInvalidProof, options.ProofPurpose, ProofPurpose)
	}
	if _, err := verificationController(options.VerificationMethod); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProof, err)
	}
	if err := validateTimestamp(options.Created); err != nil {
		return fmt.Errorf("%w: created: %v", ErrInvalidProof, err)
	}
	return nil
}

func validateTimestamp(value string) error {
	if len(value) != len("2006-01-02T15:04:05Z") || value[len(value)-1:] != "Z" {
		return errors.New("timestamp must be RFC 3339 UTC at exact second precision")
	}
	parsed, err := time.Parse("2006-01-02T15:04:05Z", value)
	if err != nil || parsed.Format("2006-01-02T15:04:05Z") != value {
		return errors.New("timestamp must be RFC 3339 UTC at exact second precision")
	}
	return nil
}

// SignDocument adds a W3C eddsa-jcs-2022 proof to an unsecured JSON object.
func SignDocument(unsecured []byte, options ProofOptions, privateKey ed25519.PrivateKey) ([]byte, error) {
	if err := options.validate(); err != nil {
		return nil, err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("Ed25519 private key has %d bytes, want %d", len(privateKey), ed25519.PrivateKeySize)
	}
	object, err := decodeObject(unsecured)
	if err != nil {
		return nil, err
	}
	if _, exists := object["proof"]; exists {
		return nil, errors.New("document is already secured")
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	resolvedDID, resolvedMethod, err := DIDKeyFromPublicKey(publicKey)
	if err == nil {
		controller, _ := verificationController(options.VerificationMethod)
		if controller == resolvedDID && options.VerificationMethod != resolvedMethod {
			return nil, errors.New("verificationMethod does not identify the supplied did:key")
		}
	}
	hashData, err := proofHashData(unsecured, options)
	if err != nil {
		return nil, err
	}
	signature := ed25519.Sign(privateKey, hashData)
	proof := map[string]any{
		"type":               options.Type,
		"cryptosuite":        options.Cryptosuite,
		"created":            options.Created,
		"verificationMethod": options.VerificationMethod,
		"proofPurpose":       options.ProofPurpose,
		"proofValue":         encodeMultibase(signature),
	}
	if contextValue, exists := object["@context"]; exists {
		proof["@context"] = contextValue
	}
	object["proof"] = proof
	return pretty(object)
}

// VerifyDocument verifies the proof and returns the verification method used.
func VerifyDocument(ctx context.Context, secured []byte, resolver KeyResolver) (string, error) {
	object, err := decodeObject(secured)
	if err != nil {
		return "", err
	}
	proofValue, exists := object["proof"]
	if !exists {
		return "", fmt.Errorf("%w: missing proof", ErrInvalidProof)
	}
	proofObject, ok := proofValue.(map[string]any)
	if !ok {
		return "", fmt.Errorf("%w: proof is not an object", ErrInvalidProof)
	}
	for key := range proofObject {
		switch key {
		case "type", "cryptosuite", "created", "verificationMethod", "proofPurpose", "proofValue", "@context":
		default:
			return "", fmt.Errorf("%w: unsupported proof option %q", ErrInvalidProof, key)
		}
	}
	if len(proofObject) < 6 {
		return "", fmt.Errorf("%w: proof is missing a required member", ErrInvalidProof)
	}
	if documentContext, exists := object["@context"]; exists {
		proofContext, proofHasContext := proofObject["@context"]
		if !proofHasContext || !reflect.DeepEqual(proofContext, documentContext) {
			return "", fmt.Errorf("%w: proof @context must equal document @context", ErrInvalidProof)
		}
	} else if _, exists := proofObject["@context"]; exists {
		return "", fmt.Errorf("%w: proof has @context but document does not", ErrInvalidProof)
	}
	proofRaw, err := pretty(proofObject)
	if err != nil {
		return "", err
	}
	var proof Proof
	if err := unmarshal(proofRaw, &proof); err != nil {
		return "", err
	}
	options := ProofOptions{
		Type:               proof.Type,
		Cryptosuite:        proof.Cryptosuite,
		Created:            proof.Created,
		VerificationMethod: proof.VerificationMethod,
		ProofPurpose:       proof.ProofPurpose,
	}
	if err := options.validate(); err != nil {
		return "", err
	}
	signature, err := decodeMultibase(proof.ProofValue)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return "", fmt.Errorf("%w: proofValue is not a 64-byte base58btc Ed25519 signature", ErrInvalidProof)
	}
	delete(object, "proof")
	unsecured, err := pretty(object)
	if err != nil {
		return "", err
	}
	hashData, err := proofHashData(unsecured, options)
	if err != nil {
		return "", err
	}
	publicKey, err := resolver.ResolveEd25519(ctx, proof.VerificationMethod)
	if err != nil {
		return "", fmt.Errorf("%w: resolve verification method: %v", ErrInvalidProof, err)
	}
	if !ed25519.Verify(publicKey, hashData, signature) {
		return "", fmt.Errorf("%w: Ed25519 signature mismatch", ErrInvalidProof)
	}
	return proof.VerificationMethod, nil
}

func proofHashData(unsecured []byte, options ProofOptions) ([]byte, error) {
	documentCanonical, err := Canonicalize(unsecured)
	if err != nil {
		return nil, err
	}
	optionsObject := map[string]any{
		"type":               options.Type,
		"cryptosuite":        options.Cryptosuite,
		"created":            options.Created,
		"verificationMethod": options.VerificationMethod,
		"proofPurpose":       options.ProofPurpose,
	}
	documentObject, err := decodeObject(unsecured)
	if err != nil {
		return nil, err
	}
	if contextValue, exists := documentObject["@context"]; exists {
		optionsObject["@context"] = contextValue
	}
	optionsCanonical, err := encodeObject(optionsObject)
	if err != nil {
		return nil, err
	}
	proofConfigHash := sha256.Sum256(optionsCanonical)
	transformedDocumentHash := sha256.Sum256(documentCanonical)
	hashData := make([]byte, 0, len(proofConfigHash)+len(transformedDocumentHash))
	hashData = append(hashData, proofConfigHash[:]...)
	hashData = append(hashData, transformedDocumentHash[:]...)
	return hashData, nil
}
