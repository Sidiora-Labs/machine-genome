package mgs

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestAuthorizedLineageIsNonCircularAndVerifiable(t *testing.T) {
	parentKey, parentDID, parentMethod := deterministicKey(t, 1)
	childKey, childDID, childMethod := deterministicKey(t, 2)
	created := time.Date(2026, 7, 20, 14, 0, 0, 0, time.UTC)

	parentUnsigned := mustJSON(t, rootGenesis("parent-model", "model", parentDID, parentMethod))
	parentSecured, err := SignRecord(parentUnsigned, NewProofOptions(created, parentMethod), parentKey)
	if err != nil {
		t.Fatal(err)
	}
	parentGene, err := VerifyGenesis(context.Background(), parentSecured, DIDKeyResolver{})
	if err != nil {
		t.Fatal(err)
	}

	child := rootGenesis("child-agent", "agent", childDID, childMethod)
	child.Parentage = []LineageEdge{{
		EdgeType:   "model-instance-of",
		ParentGene: parentGene,
		Authorized: true,
	}}
	childUnsigned := mustJSON(t, child)
	commitmentBefore, err := ChildCommitment(childUnsigned)
	if err != nil {
		t.Fatal(err)
	}
	childAuthorized, err := AuthorizeEdge(
		childUnsigned, 0, NewProofOptions(created, parentMethod), parentKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	commitmentAfterAuthorization, err := ChildCommitment(childAuthorized)
	if err != nil {
		t.Fatal(err)
	}
	if commitmentBefore != commitmentAfterAuthorization {
		t.Fatal("embedding authorization changed child commitment")
	}
	childSecured, err := SignRecord(childAuthorized, NewProofOptions(created, childMethod), childKey)
	if err != nil {
		t.Fatal(err)
	}
	commitmentAfterGenesisProof, err := ChildCommitment(childSecured)
	if err != nil {
		t.Fatal(err)
	}
	if commitmentBefore != commitmentAfterGenesisProof {
		t.Fatal("embedding genesis proof changed child commitment")
	}
	if _, err := VerifyGenesis(context.Background(), childSecured, DIDKeyResolver{}); err != nil {
		t.Fatal(err)
	}
	parents := MapGenesisResolver{parentGene: parentSecured}
	if err := VerifyLineage(context.Background(), childSecured, DIDKeyResolver{}, parents); err != nil {
		t.Fatal(err)
	}

	// Change the edge and legitimately re-sign the child. The child proof passes,
	// but the old parent's authorization must not be reusable for a new claim.
	var tampered GenesisRecord
	if err := json.Unmarshal(childAuthorized, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered.Parentage[0].EdgeType = "fine-tune-of"
	tamperedUnsigned := mustJSON(t, tampered)
	tamperedSecured, err := SignRecord(tamperedUnsigned, NewProofOptions(created, childMethod), childKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyGenesis(context.Background(), tamperedSecured, DIDKeyResolver{}); err != nil {
		t.Fatalf("child re-sign should be structurally valid: %v", err)
	}
	if err := VerifyLineage(context.Background(), tamperedSecured, DIDKeyResolver{}, parents); err == nil {
		t.Fatal("parent authorization replayed across a changed edge")
	}
}

func TestMGSGeneGoldenVector(t *testing.T) {
	raw, err := os.ReadFile("../testdata/vectors/mgs-gene-v0.1.1.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		SeedHex  string `json:"seedHex"`
		Created  string `json:"created"`
		Identity struct {
			Name        string `json:"name"`
			Namespace   string `json:"namespace"`
			SubjectType string `json:"subjectType"`
			Version     string `json:"version"`
		} `json:"identity"`
		ExpectedGene string `json:"expectedGene"`
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	seed, err := hex.DecodeString(vector.SeedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		t.Fatal("vector seed must be a 32-byte hexadecimal value")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	did, method, err := DIDKeyFromPublicKey(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	genesis := rootGenesis(vector.Identity.Name, vector.Identity.SubjectType, did, method)
	genesis.Identity.Namespace = vector.Identity.Namespace
	genesis.Identity.Version = vector.Identity.Version
	genesis.Genesis.Timestamp = vector.Created
	unsigned := mustJSON(t, genesis)
	created, err := time.Parse(time.RFC3339, vector.Created)
	if err != nil {
		t.Fatal(err)
	}
	secured, err := SignRecord(
		unsigned,
		NewProofOptions(created, method),
		privateKey,
	)
	if err != nil {
		t.Fatal(err)
	}
	gene, err := Gene(secured)
	if err != nil {
		t.Fatal(err)
	}
	if gene != vector.ExpectedGene {
		t.Fatalf("MGS Gene vector mismatch\n got: %s\nwant: %s", gene, vector.ExpectedGene)
	}
}

func rootGenesis(name, subjectType, did, method string) GenesisRecord {
	return GenesisRecord{
		MGS:        Version,
		RecordType: RecordGenesis,
		Identity: Identity{
			Name:        name,
			Namespace:   "example.test",
			SubjectType: subjectType,
			Version:     "1.0.0",
			DID:         did,
		},
		Parentage: []LineageEdge{},
		Controller: Controller{
			DID:          did,
			GenesisKeyID: method,
		},
		Genesis: GenesisMetadata{
			Timestamp:           "2026-07-20T14:00:00Z",
			ArtifactCommitments: []ArtifactCommitment{},
		},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
