package mgs

import (
	"context"
	"errors"
	"fmt"
)

type EvidenceProfile struct {
	Authorship      string `json:"authorship"`
	Review          string `json:"review"`
	Reproducibility string `json:"reproducibility"`
	RuntimeBinding  string `json:"runtimeBinding"`
}

type AttestationRecord struct {
	MGS             string          `json:"mgs"`
	RecordType      string          `json:"recordType"`
	SubjectGene     string          `json:"subjectGene"`
	AttestationType string          `json:"attestationType"`
	Timestamp       string          `json:"timestamp"`
	Issuer          string          `json:"issuer"`
	EvidenceProfile EvidenceProfile `json:"evidenceProfile"`
	Body            map[string]any  `json:"body"`
	Proof           *Proof          `json:"proof,omitempty"`
}

func ValidateAttestation(raw []byte, requireProof bool) (*AttestationRecord, error) {
	var record AttestationRecord
	if err := unmarshal(raw, &record); err != nil {
		return nil, err
	}
	if !supportedVersion(record.MGS) || record.RecordType != RecordAttestation {
		return nil, errors.New("record is not a supported MGS attestation")
	}
	if err := ValidateDigest(record.SubjectGene); err != nil {
		return nil, fmt.Errorf("subjectGene: %w", err)
	}
	if record.AttestationType == "" || record.Body == nil {
		return nil, errors.New("attestationType and body are required")
	}
	if err := validateEvidence(record.Body); err != nil {
		return nil, err
	}
	if err := validateTimestamp(record.Timestamp); err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	if err := validateDID(record.Issuer); err != nil {
		return nil, fmt.Errorf("issuer: %w", err)
	}
	if !oneOf(record.EvidenceProfile.Authorship, "self", "operator", "third-party") ||
		!oneOf(record.EvidenceProfile.Review, "none", "independent") ||
		!oneOf(record.EvidenceProfile.Reproducibility, "unavailable", "partial", "reproducible") ||
		!oneOf(record.EvidenceProfile.RuntimeBinding, "none", "hardware") {
		return nil, errors.New("invalid evidenceProfile dimension")
	}
	if requireProof != (record.Proof != nil) {
		if requireProof {
			return nil, errors.New("secured attestation requires proof")
		}
		return nil, errors.New("unsigned attestation must not contain proof")
	}
	return &record, nil
}

func VerifyAttestation(ctx context.Context, raw []byte, keys KeyResolver) (string, error) {
	record, err := ValidateAttestation(raw, true)
	if err != nil {
		return "", err
	}
	method, err := VerifyDocument(ctx, raw, keys)
	if err != nil {
		return "", err
	}
	signer, _ := verificationController(method)
	if signer != record.Issuer {
		return "", errors.New("attestation proof signer does not equal issuer")
	}
	return Digest(raw)
}

func oneOf(value string, choices ...string) bool {
	for _, choice := range choices {
		if value == choice {
			return true
		}
	}
	return false
}

func validateEvidence(body map[string]any) error {
	value, exists := body["evidence"]
	if !exists {
		return nil
	}
	items, ok := value.([]any)
	if !ok {
		return errors.New("body.evidence must be an array")
	}
	for index, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			return fmt.Errorf("body.evidence[%d] must be an object", index)
		}
		hasURI := false
		for _, key := range []string{"uri", "artifactUri"} {
			if uri, ok := object[key].(string); ok && uri != "" {
				hasURI = true
			}
		}
		if !hasURI {
			continue
		}
		hash, ok := object["hash"].(string)
		if !ok {
			return fmt.Errorf("body.evidence[%d]: every URI requires hash", index)
		}
		if err := ValidateDigest(hash); err != nil {
			return fmt.Errorf("body.evidence[%d].hash: %w", index, err)
		}
	}
	return nil
}
