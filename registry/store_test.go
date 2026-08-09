package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
)

var testTime = time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)

func TestStoreAcceptsCanonicalRecordsPersistsAndBacksUp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.db")
	store := openTestStore(t, path, false)
	key, did, method := testKey(t, 1)
	genesis := signedGenesis(t, key, did, method, "cody", "example.org")

	submission, err := store.Submit(context.Background(), genesis)
	if err != nil {
		t.Fatal(err)
	}
	if !submission.Created || submission.Metadata.LogIndex != 0 {
		t.Fatalf("unexpected first submission: %+v", submission)
	}
	if submission.Metadata.AgentID != "urn:mgs:agent:"+submission.Metadata.Gene ||
		!strings.HasPrefix(submission.Metadata.AgentName, "cody ") || submission.Metadata.FamilyName == "" {
		t.Fatalf("agent identity was not indexed: %+v", submission.Metadata)
	}
	again, err := store.Submit(context.Background(), genesis)
	if err != nil {
		t.Fatal(err)
	}
	if again.Created || again.Metadata.Digest != submission.Metadata.Digest {
		t.Fatalf("submission was not idempotent: %+v", again)
	}
	raw, err := store.Get(submission.Metadata.Digest, false)
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := mgs.Canonicalize(genesis)
	if string(raw) != string(canonical) {
		t.Fatal("registry did not retain exact canonical bytes")
	}
	proof, err := store.InclusionProof(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyInclusion(proof); err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err := store.Backup(backup); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(backup); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("backup permissions: info=%v err=%v", info, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openTestStore(t, path, false)
	defer reopened.Close()
	if err := reopened.Verify(); err != nil {
		t.Fatal(err)
	}
	view, err := reopened.Gene(submission.Metadata.Digest)
	if err != nil || view.Genesis.IdentityName != "cody" {
		t.Fatalf("restart view: %+v err=%v", view, err)
	}
	backupStore := openTestStore(t, backup, false)
	defer backupStore.Close()
	if err := backupStore.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestStoreAmendmentConflictAttestationAndTombstone(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), false)
	defer store.Close()
	controller, did, method := testKey(t, 2)
	genesis := signedGenesis(t, controller, did, method, "operator", "example.org")
	genesisSubmission, err := store.Submit(context.Background(), genesis)
	if err != nil {
		t.Fatal(err)
	}
	gene := genesisSubmission.Metadata.Digest

	first := signedAmendment(t, controller, method, gene, 1, gene, "capability-claim", map[string]any{"capabilityId": "code.go"})
	accepted, err := store.Submit(context.Background(), first)
	if err != nil || !accepted.Created {
		t.Fatalf("first amendment: %+v err=%v", accepted, err)
	}
	fork := signedAmendment(t, controller, method, gene, 1, gene, "capability-claim", map[string]any{"capabilityId": "unsafe.shell"})
	conflict, err := store.Submit(context.Background(), fork)
	if err != nil {
		t.Fatal(err)
	}
	if !conflict.Conflict || conflict.Created || conflict.ConflictID == "" {
		t.Fatalf("fork was not retained as conflict evidence: %+v", conflict)
	}
	firstHash, _ := mgs.Digest(first)
	second := signedAmendment(t, controller, method, gene, 2, firstHash, "mitigation", map[string]any{"status": "reviewed"})
	if result, err := store.Submit(context.Background(), second); err != nil || !result.Created {
		t.Fatalf("second amendment after fork: %+v err=%v", result, err)
	}

	issuer, issuerDID, issuerMethod := testKey(t, 3)
	attestation := signedAttestation(t, issuer, issuerDID, issuerMethod, gene)
	if result, err := store.Submit(context.Background(), attestation); err != nil || !result.Created {
		t.Fatalf("attestation: %+v err=%v", result, err)
	}
	view, err := store.Gene(gene)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Amendments) != 2 || len(view.Attestations) != 1 || len(view.Equivocations) != 1 {
		t.Fatalf("unexpected gene view: %+v", view)
	}
	tombstone, err := store.Tombstone(gene, "privacy request", "case-123", "test-admin")
	if err != nil || tombstone.Digest != gene {
		t.Fatalf("tombstone: %+v err=%v", tombstone, err)
	}
	if _, err := store.Get(gene, false); !errors.Is(err, ErrGone) {
		t.Fatalf("tombstoned record returned err %v", err)
	}
	if _, err := store.Get(gene, true); err != nil {
		t.Fatal("tombstone removed immutable object:", err)
	}
	if err := store.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestNamespaceChallengeRequiredConsumedAndExpires(t *testing.T) {
	now := testTime
	store, err := Open(filepath.Join(t.TempDir(), "registry.db"), Options{
		RequireNamespaceGrant: true, Clock: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	key, did, method := testKey(t, 4)
	genesis := signedGenesis(t, key, did, method, "verified", "agents.example")
	if _, err := store.Submit(context.Background(), genesis); !errors.Is(err, ErrNamespaceRequired) {
		t.Fatalf("unverified namespace returned %v", err)
	}
	challenge, token, err := store.CreateNamespaceChallenge("agents.example", did)
	if err != nil {
		t.Fatal(err)
	}
	if challenge.RecordValue != "mgs-registry="+token {
		t.Fatal("challenge did not return its DNS value")
	}
	resolver := fakeTXTResolver{challenge.RecordName: {challenge.RecordValue}}
	grant, err := store.VerifyNamespaceChallenge(context.Background(), challenge.ID, token, resolver)
	if err != nil {
		t.Fatal(err)
	}
	if grant.Namespace != "agents.example" {
		t.Fatalf("unexpected grant: %+v", grant)
	}
	if _, err := store.VerifyNamespaceChallenge(context.Background(), challenge.ID, token, resolver); err == nil {
		t.Fatal("challenge replay was accepted")
	}
	if _, err := store.Submit(context.Background(), genesis); err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * 24 * time.Hour)
	key2, did2, method2 := testKey(t, 5)
	expiredGenesis := signedGenesis(t, key2, did2, method2, "expired", "agents.example")
	if _, err := store.Submit(context.Background(), expiredGenesis); !errors.Is(err, ErrNamespaceRequired) {
		t.Fatalf("different controller used grant: %v", err)
	}
}

func TestMerkleProofsForUnevenTree(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), false)
	defer store.Close()
	for i := byte(10); i < 15; i++ {
		key, did, method := testKey(t, i)
		raw := signedGenesis(t, key, did, method, string([]byte{'a' + i - 10}), "example.org")
		if _, err := store.Submit(context.Background(), raw); err != nil {
			t.Fatal(err)
		}
	}
	for index := uint64(0); index < 5; index++ {
		proof, err := store.InclusionProof(index, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyInclusion(proof); err != nil {
			t.Fatalf("proof %d: %v", index, err)
		}
		proof.Leaf += "tampered"
		if err := VerifyInclusion(proof); err == nil {
			t.Fatalf("tampered proof %d verified", index)
		}
	}
	for first := uint64(1); first <= 5; first++ {
		proof, err := store.ConsistencyProof(first, 5)
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyConsistency(proof); err != nil {
			t.Fatalf("consistency %d->5: %v", first, err)
		}
		if first < 5 {
			proof.SecondRoot = proof.FirstRoot
			if err := VerifyConsistency(proof); err == nil {
				t.Fatalf("tampered consistency proof %d->5 verified", first)
			}
		}
	}
}

type fakeTXTResolver map[string][]string

func (resolver fakeTXTResolver) LookupTXT(_ context.Context, name string) ([]string, error) {
	return resolver[name], nil
}

func openTestStore(t *testing.T, path string, requireNamespace bool) *Store {
	t.Helper()
	store, err := Open(path, Options{
		RequireNamespaceGrant: requireNamespace,
		Clock:                 func() time.Time { return testTime },
	})
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testKey(t *testing.T, value byte) (ed25519.PrivateKey, string, string) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = value
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	did, method, err := mgs.DIDKeyFromPublicKey(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatal(err)
	}
	return privateKey, did, method
}

func signedGenesis(t *testing.T, key ed25519.PrivateKey, did, method, name, namespace string) []byte {
	t.Helper()
	record := mgs.GenesisRecord{
		MGS: mgs.Version, RecordType: mgs.RecordGenesis,
		Identity: mgs.Identity{
			Name: name, Namespace: namespace, SubjectType: "agent", Version: "1.0.0", DID: did,
			CreatedDate: "2026-07-20", AgentName: &mgs.AgentNameClaim{Given: name},
		},
		Parentage:  []mgs.LineageEdge{},
		Controller: mgs.Controller{DID: did, GenesisKeyID: method},
		Genesis: mgs.GenesisMetadata{
			Timestamp: testTime.Format(time.RFC3339), ArtifactCommitments: []mgs.ArtifactCommitment{},
		},
		Capabilities: []mgs.CapabilityClaim{{
			ID: "general.assistance", Name: "General assistance", Level: "working",
		}},
		TrainingData: []mgs.TrainingDataDeclaration{},
	}
	unsigned, _ := json.Marshal(record)
	secured, err := mgs.SignRecord(unsigned, mgs.NewProofOptions(testTime, method), key)
	if err != nil {
		t.Fatal(err)
	}
	return secured
}

func signedAmendment(t *testing.T, key ed25519.PrivateKey, method, gene string, sequence uint64, previous, kind string, body map[string]any) []byte {
	t.Helper()
	controller := method[:strings.LastIndex(method, "#")]
	record := mgs.AmendmentRecord{
		MGS: mgs.Version, RecordType: mgs.RecordAmendment, Gene: gene,
		Seq: sequence, Prev: previous, AmendmentType: kind,
		Timestamp:  testTime.Add(time.Duration(sequence) * time.Minute).Format(time.RFC3339),
		Controller: controller, Body: body,
	}
	unsigned, _ := json.Marshal(record)
	secured, err := mgs.SignRecord(unsigned, mgs.NewProofOptions(testTime.Add(time.Duration(sequence)*time.Minute), method), key)
	if err != nil {
		t.Fatal(err)
	}
	return secured
}

func signedAttestation(t *testing.T, key ed25519.PrivateKey, did, method, gene string) []byte {
	t.Helper()
	record := mgs.AttestationRecord{
		MGS: mgs.Version, RecordType: mgs.RecordAttestation, SubjectGene: gene,
		AttestationType: "review", Timestamp: testTime.Add(10 * time.Minute).Format(time.RFC3339),
		Issuer: did, EvidenceProfile: mgs.EvidenceProfile{
			Authorship: "third-party", Review: "independent",
			Reproducibility: "partial", RuntimeBinding: "none",
		},
		Body: map[string]any{"summary": "independently reviewed"},
	}
	unsigned, _ := json.Marshal(record)
	secured, err := mgs.SignRecord(unsigned, mgs.NewProofOptions(testTime.Add(10*time.Minute), method), key)
	if err != nil {
		t.Fatal(err)
	}
	return secured
}
