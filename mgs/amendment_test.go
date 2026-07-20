package mgs

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestControllerRotationUsesChainOrder(t *testing.T) {
	firstKey, firstDID, firstMethod := deterministicKey(t, 3)
	secondKey, secondDID, secondMethod := deterministicKey(t, 4)
	created := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	genesisUnsigned := mustJSON(t, rootGenesis("agent", "agent", firstDID, firstMethod))
	genesis, err := SignRecord(genesisUnsigned, NewProofOptions(created, firstMethod), firstKey)
	if err != nil {
		t.Fatal(err)
	}
	gene, _ := Gene(genesis)

	rotation := AmendmentRecord{
		MGS: Version, RecordType: RecordAmendment, Gene: gene, Seq: 1, Prev: gene,
		AmendmentType: "controller-rotation", Timestamp: "2026-07-20T14:01:00Z",
		Controller: firstDID, Body: map[string]any{"newControllerDid": secondDID},
	}
	rotationSecured, err := SignRecord(mustJSON(t, rotation), NewProofOptions(created.Add(time.Minute), firstMethod), firstKey)
	if err != nil {
		t.Fatal(err)
	}
	rotationHash, _ := Digest(rotationSecured)
	claim := AmendmentRecord{
		MGS: Version, RecordType: RecordAmendment, Gene: gene, Seq: 2, Prev: rotationHash,
		AmendmentType: "capability-claim", Timestamp: "2026-07-20T13:00:00Z",
		Controller: secondDID, Body: map[string]any{"capabilityId": "code.go"},
	}
	// Deliberately backdated author time: sequence, not timestamp, controls the key.
	claimSecured, err := SignRecord(mustJSON(t, claim), NewProofOptions(created.Add(2*time.Minute), secondMethod), secondKey)
	if err != nil {
		t.Fatal(err)
	}
	state, err := VerifyAmendmentChain(
		context.Background(), genesis, [][]byte{rotationSecured, claimSecured}, DIDKeyResolver{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if state.LastSequence != 2 || state.Controller != secondDID {
		t.Fatalf("unexpected state: %+v", state)
	}

	var fork AmendmentRecord
	if err := json.Unmarshal(claimSecured, &fork); err != nil {
		t.Fatal(err)
	}
	fork.Proof = nil
	fork.Prev = gene
	forkSecured, err := SignRecord(mustJSON(t, fork), NewProofOptions(created.Add(2*time.Minute), secondMethod), secondKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyAmendmentChain(
		context.Background(), genesis, [][]byte{rotationSecured, forkSecured}, DIDKeyResolver{},
	); err == nil {
		t.Fatal("fork with wrong prev verified")
	}
}

func TestThirdPartyAttestationIsIndependent(t *testing.T) {
	_, subjectDID, subjectMethod := deterministicKey(t, 5)
	issuerKey, issuerDID, issuerMethod := deterministicKey(t, 6)
	created := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)
	subjectKey, _, _ := deterministicKey(t, 5)
	genesis, err := SignRecord(
		mustJSON(t, rootGenesis("subject", "agent", subjectDID, subjectMethod)),
		NewProofOptions(created, subjectMethod),
		subjectKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	gene, _ := Gene(genesis)
	attestation := AttestationRecord{
		MGS: Version, RecordType: RecordAttestation, SubjectGene: gene,
		AttestationType: "failure-record", Timestamp: "2026-07-20T14:10:00Z",
		Issuer: issuerDID,
		EvidenceProfile: EvidenceProfile{
			Authorship: "third-party", Review: "independent",
			Reproducibility: "partial", RuntimeBinding: "none",
		},
		Body: map[string]any{
			"class": "hallucination.absence-blindness",
			"evidence": []any{map[string]any{
				"uri":  "https://evidence.example/trace.json",
				"hash": gene,
			}},
		},
	}
	secured, err := SignRecord(
		mustJSON(t, attestation),
		NewProofOptions(created.Add(10*time.Minute), issuerMethod),
		issuerKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyAttestation(context.Background(), secured, DIDKeyResolver{}); err != nil {
		t.Fatal(err)
	}
}
