package mgs

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestAgentIdentityIsDerivedFromSignedGene(t *testing.T) {
	raw, err := os.ReadFile("../testdata/vectors/mgs-agent-v0.2.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Profile  string `json:"profile"`
		SeedHex  string `json:"seedHex"`
		Created  string `json:"created"`
		Identity struct {
			Name, Namespace, SubjectType, Version, CreatedDate, GivenName string
		} `json:"identity"`
		Capabilities       []CapabilityClaim         `json:"capabilities"`
		TrainingData       []TrainingDataDeclaration `json:"trainingData"`
		ExpectedGene       string                    `json:"expectedGene"`
		ExpectedAgentID    string                    `json:"expectedAgentId"`
		ExpectedFamilyName string                    `json:"expectedFamilyName"`
		ExpectedAgentName  string                    `json:"expectedAgentName"`
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	seed, err := hex.DecodeString(vector.SeedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		t.Fatal("vector seed must be a 32-byte hexadecimal value")
	}
	key := ed25519.NewKeyFromSeed(seed)
	did, method, err := DIDKeyFromPublicKey(key.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	record := rootGenesis(vector.Identity.Name, vector.Identity.SubjectType, did, method)
	record.MGS = vector.Profile
	record.Identity.Namespace = vector.Identity.Namespace
	record.Identity.Version = vector.Identity.Version
	record.Identity.CreatedDate = vector.Identity.CreatedDate
	record.Identity.AgentName = &AgentNameClaim{Given: vector.Identity.GivenName}
	record.Capabilities = vector.Capabilities
	record.TrainingData = vector.TrainingData
	record.Genesis.Timestamp = vector.Created
	created, err := time.Parse(time.RFC3339, vector.Created)
	if err != nil {
		t.Fatal(err)
	}
	secured, err := SignRecord(mustJSON(t, record), NewProofOptions(created, method), key)
	if err != nil {
		t.Fatal(err)
	}
	gene, err := VerifyGenesis(context.Background(), secured, DIDKeyResolver{})
	if err != nil {
		t.Fatal(err)
	}
	agent, err := DeriveAgentIdentity(&record, gene)
	if err != nil {
		t.Fatal(err)
	}
	if gene != vector.ExpectedGene || agent.ID != vector.ExpectedAgentID ||
		agent.FamilyName != vector.ExpectedFamilyName || agent.Name != vector.ExpectedAgentName {
		t.Fatalf("unexpected derived identity: %+v", agent)
	}
	if !strings.Contains(agent.FamilyName, "-") || len(agent.FamilyName) < 20 {
		t.Fatalf("surname is not readable and collision-resistant: %q", agent.FamilyName)
	}
	second, err := DeriveAgentIdentity(&record, gene)
	if err != nil || second != agent {
		t.Fatalf("identity derivation is not deterministic: %+v err=%v", second, err)
	}
}

func TestV020AgentClaimsFailClosed(t *testing.T) {
	_, did, method := deterministicKey(t, 30)
	record := rootGenesis("agent", "agent", did, method)
	record.Identity.AgentName = nil
	if _, err := ValidateGenesis(mustJSON(t, record), false); err == nil || !strings.Contains(err.Error(), "agentName.given") {
		t.Fatalf("missing agent name returned %v", err)
	}
	record.Identity.AgentName = &AgentNameClaim{Given: "Agent"}
	record.Capabilities = []CapabilityClaim{{ID: "Bad ID", Name: "Bad", Level: "expert"}}
	if _, err := ValidateGenesis(mustJSON(t, record), false); err == nil || !strings.Contains(err.Error(), "canonical id") {
		t.Fatalf("invalid capability returned %v", err)
	}
}
