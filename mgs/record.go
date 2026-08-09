package mgs

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	LegacyVersion       = "0.1.1"
	Version             = "0.2.0"
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
		"trained-on": true, "successor-of": true, "powered-by": true,
	}
	capabilityIDPattern = regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*$`)
)

type Identity struct {
	Name        string          `json:"name"`
	Namespace   string          `json:"namespace"`
	SubjectType string          `json:"subjectType"`
	Version     string          `json:"version"`
	Description string          `json:"description,omitempty"`
	DID         string          `json:"did,omitempty"`
	CreatedDate string          `json:"createdDate,omitempty"`
	AgentName   *AgentNameClaim `json:"agentName,omitempty"`
}

// AgentNameClaim is the controller-chosen part of an agent's public name. The
// registry appends a deterministic GenomeSurname derived from the secured Gene.
type AgentNameClaim struct {
	Given string `json:"given"`
}

// CapabilityClaim is a controller-authored, machine-readable self-description.
// Consumers decide how much evidence or trust to require for each claim.
type CapabilityClaim struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category,omitempty"`
	Level       string   `json:"level"`
	Description string   `json:"description,omitempty"`
	Evidence    []string `json:"evidence,omitempty"`
}

// TrainingDataDeclaration records what is known about one training stage. It
// permits an explicit undisclosed or unknown statement instead of encouraging
// invented dataset precision.
type TrainingDataDeclaration struct {
	Stage       string `json:"stage"`
	Name        string `json:"name"`
	Disclosure  string `json:"disclosure"`
	Description string `json:"description,omitempty"`
	URI         string `json:"uri,omitempty"`
	Digest      string `json:"digest,omitempty"`
	License     string `json:"license,omitempty"`
	DataCutoff  string `json:"dataCutoff,omitempty"`
}

// ModelProfile gives agents a stable provider/model coordinate to reference.
// Foundation is a classification claim, not vendor authorization.
type ModelProfile struct {
	Provider      string   `json:"provider"`
	Providers     []string `json:"providers,omitempty"`
	ModelID       string   `json:"modelId"`
	Family        string   `json:"family,omitempty"`
	ContextWindow uint64   `json:"contextWindow,omitempty"`
	Foundation    bool     `json:"foundation"`
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
	MGS          string                    `json:"mgs"`
	RecordType   string                    `json:"recordType"`
	Identity     Identity                  `json:"identity"`
	Parentage    []LineageEdge             `json:"parentage"`
	Controller   Controller                `json:"controller"`
	Genesis      GenesisMetadata           `json:"genesis"`
	Capabilities []CapabilityClaim         `json:"capabilities"`
	TrainingData []TrainingDataDeclaration `json:"trainingData"`
	Model        *ModelProfile             `json:"model,omitempty"`
	Proof        *Proof                    `json:"proof,omitempty"`
}

func ValidateGenesis(raw []byte, requireProof bool) (*GenesisRecord, error) {
	var record GenesisRecord
	if err := unmarshal(raw, &record); err != nil {
		return nil, err
	}
	if !supportedVersion(record.MGS) {
		return nil, fmt.Errorf("unsupported mgs profile %q", record.MGS)
	}
	if record.MGS == LegacyVersion {
		object, err := decodeObject(raw)
		if err != nil {
			return nil, err
		}
		for _, field := range []string{"capabilities", "trainingData", "model"} {
			if _, exists := object[field]; exists {
				return nil, fmt.Errorf("MGS %s genesis does not define %s", LegacyVersion, field)
			}
		}
		if identity, ok := object["identity"].(map[string]any); ok {
			for _, field := range []string{"createdDate", "agentName"} {
				if _, exists := identity[field]; exists {
					return nil, fmt.Errorf("MGS %s identity does not define %s", LegacyVersion, field)
				}
			}
		}
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
	if record.MGS == Version {
		if err := validateV020Identity(&record); err != nil {
			return nil, err
		}
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
	if record.MGS == Version {
		createdDate, _ := time.Parse("2006-01-02", record.Identity.CreatedDate)
		genesisTime, _ := time.Parse(time.RFC3339, record.Genesis.Timestamp)
		if createdDate.After(genesisTime) {
			return nil, errors.New("identity.createdDate must not be after genesis.timestamp")
		}
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
		if record.MGS == LegacyVersion && edge.EdgeType == "powered-by" {
			return nil, fmt.Errorf("parentage[%d]: edgeType %q requires MGS 0.2.0", index, edge.EdgeType)
		}
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
	capabilityIDs := make(map[string]bool, len(record.Capabilities))
	for index, capability := range record.Capabilities {
		if !capabilityIDPattern.MatchString(capability.ID) || strings.TrimSpace(capability.Name) == "" ||
			!oneOf(capability.Level, "basic", "working", "advanced", "expert") {
			return nil, fmt.Errorf("capabilities[%d] requires a canonical id, name, and valid level", index)
		}
		if capabilityIDs[capability.ID] {
			return nil, fmt.Errorf("capabilities[%d] duplicates id %q", index, capability.ID)
		}
		capabilityIDs[capability.ID] = true
		for evidenceIndex, digest := range capability.Evidence {
			if err := ValidateDigest(digest); err != nil {
				return nil, fmt.Errorf("capabilities[%d].evidence[%d]: %w", index, evidenceIndex, err)
			}
		}
	}
	for index, declaration := range record.TrainingData {
		if !oneOf(declaration.Stage, "pretraining", "post-training", "fine-tuning", "reinforcement", "distillation", "runtime-learning", "other") ||
			!oneOf(declaration.Disclosure, "full", "partial", "summary", "undisclosed", "unknown", "not-applicable") ||
			strings.TrimSpace(declaration.Name) == "" {
			return nil, fmt.Errorf("trainingData[%d] requires a name, valid stage, and disclosure", index)
		}
		if declaration.Digest != "" {
			if err := ValidateDigest(declaration.Digest); err != nil {
				return nil, fmt.Errorf("trainingData[%d].digest: %w", index, err)
			}
		}
		if declaration.URI != "" {
			parsed, err := url.ParseRequestURI(declaration.URI)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return nil, fmt.Errorf("trainingData[%d].uri must be an absolute URI", index)
			}
			if declaration.Digest == "" {
				return nil, fmt.Errorf("trainingData[%d].uri requires digest", index)
			}
		}
		if declaration.DataCutoff != "" {
			if _, err := time.Parse("2006-01-02", declaration.DataCutoff); err != nil {
				return nil, fmt.Errorf("trainingData[%d].dataCutoff must be YYYY-MM-DD", index)
			}
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

func supportedVersion(version string) bool {
	return version == LegacyVersion || version == Version
}

func validateV020Identity(record *GenesisRecord) error {
	if record.Capabilities == nil || record.TrainingData == nil {
		return errors.New("MGS 0.2.0 genesis requires capabilities and trainingData arrays")
	}
	if record.Identity.CreatedDate == "" {
		return errors.New("MGS 0.2.0 identity.createdDate is required")
	}
	if _, err := time.Parse("2006-01-02", record.Identity.CreatedDate); err != nil {
		return errors.New("identity.createdDate must be YYYY-MM-DD")
	}
	if record.Identity.SubjectType == "agent" || record.Identity.SubjectType == "agent-instance" {
		if record.Identity.AgentName == nil || strings.TrimSpace(record.Identity.AgentName.Given) == "" {
			return errors.New("MGS 0.2.0 agents require identity.agentName.given")
		}
	}
	if record.Identity.AgentName != nil && !validGivenName(record.Identity.AgentName.Given) {
		return errors.New("identity.agentName.given must be 1-32 letters with optional internal apostrophe or hyphen")
	}
	if record.Identity.SubjectType == "model" {
		if record.Model == nil || strings.TrimSpace(record.Model.Provider) == "" || strings.TrimSpace(record.Model.ModelID) == "" {
			return errors.New("MGS 0.2.0 models require model.provider and model.modelId")
		}
		providers := make(map[string]bool, len(record.Model.Providers))
		for index, provider := range record.Model.Providers {
			if strings.TrimSpace(provider) == "" {
				return fmt.Errorf("model.providers[%d] must not be empty", index)
			}
			canonical := strings.ToLower(strings.TrimSpace(provider))
			if providers[canonical] {
				return fmt.Errorf("model.providers[%d] duplicates %q", index, provider)
			}
			providers[canonical] = true
		}
		if len(record.Model.Providers) > 0 && !providers[strings.ToLower(strings.TrimSpace(record.Model.Provider))] {
			return errors.New("model.providers must include model.provider")
		}
	} else if record.Model != nil {
		return errors.New("model profile is only valid for subjectType model")
	}
	return nil
}

func validGivenName(value string) bool {
	if value != strings.TrimSpace(value) {
		return false
	}
	runes := []rune(value)
	if len(runes) == 0 || len(runes) > 32 {
		return false
	}
	for index, char := range runes {
		if unicode.IsLetter(char) {
			continue
		}
		if (char == '-' || char == '\'' || char == '’') && index > 0 && index < len(runes)-1 &&
			unicode.IsLetter(runes[index-1]) && unicode.IsLetter(runes[index+1]) {
			continue
		}
		return false
	}
	return true
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
