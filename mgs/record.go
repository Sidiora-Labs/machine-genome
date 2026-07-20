package mgs

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
)

const (
	Version             = "0.1.1"
	RecordGenesis       = "genesis"
	RecordAmendment     = "amendment"
	RecordAttestation   = "attestation"
	RecordAuthorization = "lineage-authorization"
)

var (
	subjectTypes = map[string]bool{
		"model": true, "agent": true, "agent-class": true,
		"agent-instance": true, "harness": true, "dataset": true,
	}
	edgeTypes = map[string]bool{
		"fine-tune-of": true, "distilled-from": true, "merge-of": true,
		"quantization-of": true, "model-instance-of": true,
		"scaffold-fork-of": true, "instance-of": true,
		"prompt-descendant-of": true, "ensemble-of": true,
		"trained-on": true, "successor-of": true,
	}
)

type Identity struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	SubjectType string `json:"subjectType"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	DID         string `json:"did,omitempty"`
}

type Controller struct {
	DID          string `json:"did"`
	GenesisKeyID string `json:"genesisKeyId"`
}

type GenesisMetadata struct {
	Timestamp           string               `json:"timestamp"`
	Registrar           string               `json:"registrar,omitempty"`
	ArtifactCommitments []ArtifactCommitment `json:"artifactCommitments"`
}

type ArtifactCommitment struct {
	Kind    string `json:"kind"`
	Hash    string `json:"hash"`
	Present bool   `json:"present"`
	URI     string `json:"uri,omitempty"`
	Note    string `json:"note,omitempty"`
}

type LineageEdge struct {
	EdgeType           string `json:"edgeType"`
	ParentGene         string `json:"parentGene,omitempty"`
	Authorized         bool   `json:"authorized"`
	AuthorizationProof *Proof `json:"authorizationProof,omitempty"`
	Unresolved         bool   `json:"unresolved,omitempty"`
	ExternalRef        string `json:"externalRef,omitempty"`
}

type GenesisRecord struct {
	MGS        string          `json:"mgs"`
	RecordType string          `json:"recordType"`
	Identity   Identity        `json:"identity"`
	Parentage  []LineageEdge   `json:"parentage"`
	Controller Controller      `json:"controller"`
	Genesis    GenesisMetadata `json:"genesis"`
	Proof      *Proof          `json:"proof,omitempty"`
}

func ValidateGenesis(raw []byte, requireProof bool) (*GenesisRecord, error) {
	var record GenesisRecord
	if err := unmarshal(raw, &record); err != nil {
		return nil, err
	}
	if record.MGS != Version {
		return nil, fmt.Errorf("mgs is %q, want %q", record.MGS, Version)
	}
	if record.RecordType != RecordGenesis {
		return nil, fmt.Errorf("recordType is %q, want %q", record.RecordType, RecordGenesis)
	}
	if record.Identity.Name == "" || record.Identity.Namespace == "" || record.Identity.Version == "" {
		return nil, errors.New("identity name, namespace, and version are required")
	}
	if record.Identity.DID != "" {
		if err := validateDID(record.Identity.DID); err != nil {
			return nil, fmt.Errorf("identity.did: %w", err)
		}
	}
	if !subjectTypes[record.Identity.SubjectType] {
		return nil, fmt.Errorf("unknown subjectType %q", record.Identity.SubjectType)
	}
	if record.Controller.DID == "" || record.Controller.GenesisKeyID == "" {
		return nil, errors.New("controller.did and controller.genesisKeyId are required")
	}
	keyController, err := verificationController(record.Controller.GenesisKeyID)
	if err != nil || keyController != record.Controller.DID {
		return nil, errors.New("controller.genesisKeyId must be a DID URL controlled by controller.did")
	}
	if err := validateTimestamp(record.Genesis.Timestamp); err != nil {
		return nil, fmt.Errorf("genesis.timestamp: %w", err)
	}
	for index, artifact := range record.Genesis.ArtifactCommitments {
		if artifact.Kind == "" || artifact.Hash == "" {
			return nil, fmt.Errorf("artifactCommitments[%d] requires kind and hash", index)
		}
		if err := ValidateDigest(artifact.Hash); err != nil {
			return nil, fmt.Errorf("artifactCommitments[%d].hash: %w", index, err)
		}
		if artifact.URI != "" && !artifact.Present {
			return nil, fmt.Errorf("artifactCommitments[%d]: uri requires present true", index)
		}
	}
	for index, edge := range record.Parentage {
		if !edgeTypes[edge.EdgeType] {
			return nil, fmt.Errorf("parentage[%d]: unknown edgeType %q", index, edge.EdgeType)
		}
		if edge.Unresolved {
			if edge.Authorized || edge.ParentGene != "" || edge.ExternalRef == "" || edge.AuthorizationProof != nil {
				return nil, fmt.Errorf("parentage[%d]: unresolved edge must be unauthorized, have externalRef, and omit parentGene/proof", index)
			}
			continue
		}
		if err := ValidateDigest(edge.ParentGene); err != nil {
			return nil, fmt.Errorf("parentage[%d].parentGene: %w", index, err)
		}
		if edge.Authorized != (edge.AuthorizationProof != nil) {
			return nil, fmt.Errorf("parentage[%d]: authorized must exactly match presence of authorizationProof", index)
		}
	}
	if requireProof && record.Proof == nil {
		return nil, errors.New("secured genesis requires proof")
	}
	if !requireProof && record.Proof != nil {
		return nil, errors.New("unsigned genesis must not contain proof")
	}
	if requireProof && record.Proof.Created != record.Genesis.Timestamp {
		return nil, errors.New("genesis proof created must equal genesis.timestamp")
	}
	return &record, nil
}

func VerifyGenesis(ctx context.Context, raw []byte, resolver KeyResolver) (string, error) {
	record, err := ValidateGenesis(raw, true)
	if err != nil {
		return "", err
	}
	method, err := VerifyDocument(ctx, raw, resolver)
	if err != nil {
		return "", err
	}
	if method != record.Controller.GenesisKeyID {
		return "", errors.New("genesis proof verificationMethod does not equal controller.genesisKeyId")
	}
	controller, err := verificationController(method)
	if err != nil || controller != record.Controller.DID {
		return "", errors.New("genesis proof signer does not equal controller.did")
	}
	return Gene(raw)
}

func SignRecord(raw []byte, options ProofOptions, privateKey ed25519.PrivateKey) ([]byte, error) {
	var header struct {
		RecordType string `json:"recordType"`
	}
	if err := unmarshal(raw, &header); err != nil {
		return nil, err
	}
	switch header.RecordType {
	case RecordGenesis:
		record, err := ValidateGenesis(raw, false)
		if err != nil {
			return nil, err
		}
		if options.VerificationMethod != record.Controller.GenesisKeyID {
			return nil, errors.New("proof verificationMethod must equal controller.genesisKeyId")
		}
	case RecordAmendment:
		record, err := ValidateAmendment(raw, false)
		if err != nil {
			return nil, err
		}
		controller, _ := verificationController(options.VerificationMethod)
		if controller != record.Controller {
			return nil, errors.New("amendment proof signer must equal controller")
		}
	case RecordAttestation:
		record, err := ValidateAttestation(raw, false)
		if err != nil {
			return nil, err
		}
		controller, _ := verificationController(options.VerificationMethod)
		if controller != record.Issuer {
			return nil, errors.New("attestation proof signer must equal issuer")
		}
	default:
		return nil, fmt.Errorf("unsupported recordType %q", header.RecordType)
	}
	return SignDocument(raw, options, privateKey)
}

func validateDID(value string) error {
	if !strings.HasPrefix(value, "did:") || len(strings.Split(value, ":")) < 3 {
		return errors.New("value is not a DID")
	}
	return nil
}
