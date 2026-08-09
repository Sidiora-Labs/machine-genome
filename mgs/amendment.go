package mgs

import (
	"context"
	"errors"
	"fmt"
)

type AmendmentRecord struct {
	MGS           string         `json:"mgs"`
	RecordType    string         `json:"recordType"`
	Gene          string         `json:"gene"`
	Seq           uint64         `json:"seq"`
	Prev          string         `json:"prev"`
	AmendmentType string         `json:"amendmentType"`
	Timestamp     string         `json:"timestamp"`
	Controller    string         `json:"controller"`
	Body          map[string]any `json:"body"`
	Proof         *Proof         `json:"proof,omitempty"`
}

func ValidateAmendment(raw []byte, requireProof bool) (*AmendmentRecord, error) {
	var record AmendmentRecord
	if err := unmarshal(raw, &record); err != nil {
		return nil, err
	}
	if !supportedVersion(record.MGS) || record.RecordType != RecordAmendment {
		return nil, errors.New("record is not a supported MGS amendment")
	}
	if err := ValidateDigest(record.Gene); err != nil {
		return nil, fmt.Errorf("gene: %w", err)
	}
	if record.Seq == 0 {
		return nil, errors.New("amendment seq starts at 1")
	}
	if err := ValidateDigest(record.Prev); err != nil {
		return nil, fmt.Errorf("prev: %w", err)
	}
	if record.AmendmentType == "" || record.Body == nil {
		return nil, errors.New("amendmentType and body are required")
	}
	if err := validateEvidence(record.Body); err != nil {
		return nil, err
	}
	if err := validateTimestamp(record.Timestamp); err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	if err := validateDID(record.Controller); err != nil {
		return nil, fmt.Errorf("controller: %w", err)
	}
	if requireProof != (record.Proof != nil) {
		if requireProof {
			return nil, errors.New("secured amendment requires proof")
		}
		return nil, errors.New("unsigned amendment must not contain proof")
	}
	return &record, nil
}

type ChainState struct {
	Gene            string
	Controller      string
	LastSequence    uint64
	LastHash        string
	Terminal        bool
	EquivocationKey string
}

func VerifyAmendment(ctx context.Context, raw []byte, keys KeyResolver) (string, error) {
	record, err := ValidateAmendment(raw, true)
	if err != nil {
		return "", err
	}
	method, err := VerifyDocument(ctx, raw, keys)
	if err != nil {
		return "", err
	}
	signer, _ := verificationController(method)
	if signer != record.Controller {
		return "", errors.New("amendment proof signer does not equal declared controller")
	}
	return Digest(raw)
}

// VerifyAmendmentChain verifies one branch. Competing valid records at the same
// sequence are not silently resolved; callers should retain them as
// equivocation evidence.
func VerifyAmendmentChain(ctx context.Context, genesis []byte, amendments [][]byte, keys KeyResolver) (*ChainState, error) {
	genesisRecord, err := ValidateGenesis(genesis, true)
	if err != nil {
		return nil, err
	}
	gene, err := VerifyGenesis(ctx, genesis, keys)
	if err != nil {
		return nil, err
	}
	state := &ChainState{
		Gene:       gene,
		Controller: genesisRecord.Controller.DID,
		LastHash:   gene,
	}
	for index, raw := range amendments {
		record, err := ValidateAmendment(raw, true)
		if err != nil {
			return nil, fmt.Errorf("amendment[%d]: %w", index, err)
		}
		expectedSequence := state.LastSequence + 1
		if record.Gene != gene || record.Seq != expectedSequence || record.Prev != state.LastHash {
			return nil, fmt.Errorf(
				"amendment[%d]: chain mismatch (gene=%t seq=%d want=%d prev=%t)",
				index, record.Gene == gene, record.Seq, expectedSequence, record.Prev == state.LastHash,
			)
		}
		if state.Terminal && record.AmendmentType != "dispute" {
			return nil, fmt.Errorf("amendment[%d]: terminal chain permits only dispute records", index)
		}
		method, err := VerifyDocument(ctx, raw, keys)
		if err != nil {
			return nil, fmt.Errorf("amendment[%d]: %w", index, err)
		}
		signer, _ := verificationController(method)
		if record.Controller != state.Controller || signer != state.Controller {
			return nil, fmt.Errorf("amendment[%d]: signer/controller is not active controller %q", index, state.Controller)
		}
		hash, err := Digest(raw)
		if err != nil {
			return nil, err
		}
		state.LastSequence = record.Seq
		state.LastHash = hash
		state.EquivocationKey = fmt.Sprintf("%s/%d/%s", gene, record.Seq, record.Prev)
		switch record.AmendmentType {
		case "controller-rotation":
			next, ok := record.Body["newControllerDid"].(string)
			if !ok || validateDID(next) != nil {
				return nil, fmt.Errorf("amendment[%d]: controller-rotation requires newControllerDid", index)
			}
			state.Controller = next
		case "supersession-notice":
			if terminal, _ := record.Body["terminal"].(bool); terminal {
				state.Terminal = true
			}
		}
	}
	return state, nil
}
