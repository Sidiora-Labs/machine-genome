package registry

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
)

func TestHTTPRegistryJourneyAndSecurityBoundaries(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), false)
	defer store.Close()
	controller, did, method := testKey(t, 20)
	checkpointKey, checkpointDID, checkpointMethod := testKey(t, 21)
	const adminToken = "0123456789abcdef0123456789abcdef"
	service, err := NewHTTPService(HTTPConfig{
		Store: store, AdminToken: adminToken, CheckpointKey: checkpointKey,
		CheckpointDID: checkpointDID, CheckpointMethod: checkpointMethod,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RatePerMinute: 1000,
		BackupDir: filepath.Join(t.TempDir(), "backups"),
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	genesis := signedGenesis(t, controller, did, method, "web-agent", "example.org")
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/records", bytes.NewReader(genesis))
	request.Header.Set("Content-Type", recordMediaType)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("submit status %d: %s", response.StatusCode, readBody(response))
	}
	if response.Header.Get("X-Content-Type-Options") != "nosniff" || response.Header.Get("Content-Security-Policy") == "" {
		t.Fatal("security headers are missing")
	}
	var submitted Submission
	decodeBody(t, response, &submitted)
	gene := submitted.Metadata.Digest

	response = get(t, server.URL+"/v1/records/"+gene)
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != recordMediaType {
		t.Fatalf("record response: %d %s", response.StatusCode, response.Header.Get("Content-Type"))
	}
	raw := readBodyBytes(response)
	canonical, _ := mgs.Canonicalize(genesis)
	if !bytes.Equal(raw, canonical) {
		t.Fatal("HTTP did not return canonical record bytes")
	}
	response = get(t, server.URL+"/")
	home := readBody(response)
	if !strings.Contains(home, "Identity for machines") || !strings.Contains(home, "Identity directory") ||
		!strings.Contains(home, "Agent ID, name, model, or Gene") || !strings.Contains(home, "Page 1 of 1") {
		t.Fatal("registry explorer home did not render its primary journey")
	}
	response = get(t, server.URL+"/genes/"+gene)
	genePage := readBody(response)
	if response.StatusCode != http.StatusOK || !strings.Contains(genePage, "web-agent") ||
		!strings.Contains(genePage, "Included & signed") ||
		!strings.Contains(genePage, "Identity genome") || !strings.Contains(genePage, submitted.Metadata.AgentID) ||
		!strings.Contains(genePage, "This is a root record with no declared parents") {
		t.Fatal("plain-language explorer did not render gene")
	}

	childGenesis := signedGenesisWithParent(t, controller, did, method, "child-agent", "example.org", gene)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v1/records", bytes.NewReader(childGenesis))
	request.Header.Set("Content-Type", recordMediaType)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("submit child status %d: %s", response.StatusCode, readBody(response))
	}
	var child Submission
	decodeBody(t, response, &child)
	response = get(t, server.URL+"/genes/"+child.Metadata.Digest)
	childPage := readBody(response)
	if !strings.Contains(childPage, "web-agent") || !strings.Contains(childPage, "model-instance-of") ||
		!strings.Contains(childPage, "Observed claim") {
		t.Fatal("registry explorer did not render parent lineage and authorization state")
	}

	response = get(t, server.URL+"/v1/log/checkpoint")
	var checkpoint Checkpoint
	decodeBody(t, response, &checkpoint)
	if checkpoint.TreeSize != 2 {
		t.Fatalf("checkpoint size %d", checkpoint.TreeSize)
	}
	if err := VerifyCheckpoint(checkpoint, checkpointKey.Public().(ed25519.PublicKey)); err != nil {
		t.Fatal(err)
	}
	response = get(t, server.URL+"/v1/log/inclusion/0")
	var proof InclusionProof
	decodeBody(t, response, &proof)
	if err := VerifyInclusion(proof); err != nil {
		t.Fatal(err)
	}
	response = get(t, server.URL+"/v1/log/consistency?firstSize=1&secondSize=1")
	var consistency ConsistencyProof
	decodeBody(t, response, &consistency)
	if err := VerifyConsistency(consistency); err != nil {
		t.Fatal(err)
	}

	tombstoneBody := strings.NewReader(`{"reason":"legal restriction","reference":"case-7"}`)
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v1/admin/tombstones/"+gene, tombstoneBody)
	request.Header.Set("Content-Type", "application/json")
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthorized admin status %d", response.StatusCode)
	}
	response.Body.Close()
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v1/admin/tombstones/"+gene, strings.NewReader(`{"reason":"legal restriction","reference":"case-7"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+adminToken)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("authorized tombstone: %d %s", response.StatusCode, readBody(response))
	}
	response.Body.Close()
	response = get(t, server.URL+"/v1/records/"+gene)
	if response.StatusCode != http.StatusGone {
		t.Fatalf("tombstoned payload status %d", response.StatusCode)
	}
	var tombstone Tombstone
	decodeBody(t, response, &tombstone)
	if tombstone.Digest != gene {
		t.Fatalf("unexpected tombstone: %+v", tombstone)
	}
	response = get(t, server.URL+"/v1/log/inclusion/0")
	if response.StatusCode != http.StatusOK {
		t.Fatal("tombstone removed inclusion proof")
	}
	response.Body.Close()

	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v1/admin/backups", nil)
	request.Header.Set("Authorization", "Bearer "+adminToken)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("online backup: %d %s", response.StatusCode, readBody(response))
	}
	var backupResult struct {
		File string `json:"file"`
	}
	decodeBody(t, response, &backupResult)
	backupStore := openTestStore(t, filepath.Join(service.backupDir, backupResult.File), false)
	defer backupStore.Close()
	if err := backupStore.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestExplorerDirectoryPaginationAndIdentityFilters(t *testing.T) {
	items := make([]RecordMetadata, 50)
	for index := range items {
		items[index] = RecordMetadata{
			Gene:         "gene-" + strconv.Itoa(index),
			IdentityName: "identity-" + strconv.Itoa(index),
			AgentID:      "urn:mgs:agent:gene-" + strconv.Itoa(index),
			AgentName:    "Ada Genome" + strconv.Itoa(index),
			SubjectType:  "agent",
			Capabilities: []string{"reasoning", "tool-use"},
			LogIndex:     uint64(index),
		}
	}
	items[49].SubjectType = "model"
	items[49].Foundation = true
	items[49].ModelProvider = "openai"
	items[49].ModelID = "gpt-test"

	page := prepareExplorerDirectory(explorerData{}, items, url.Values{"page": {"2"}})
	if page.Total != 50 || page.Page != 2 || page.PageCount != 3 || len(page.Items) != explorerPageSize ||
		page.PageStart != 25 || page.PageEnd != 48 || !page.HasPrevious || !page.HasNext ||
		page.Items[0].LogIndex != 25 || !strings.Contains(page.PreviousURL, "page=1") || !strings.Contains(page.NextURL, "page=3") {
		t.Fatalf("unexpected second directory page: %+v", page)
	}

	filtered := prepareExplorerDirectory(explorerData{}, items, url.Values{
		"q": {"gpt-test"}, "type": {"foundation"}, "provider": {"openai"}, "capability": {"reasoning"},
	})
	if filtered.Total != 1 || len(filtered.Items) != 1 || filtered.Items[0].ModelID != "gpt-test" {
		t.Fatalf("identity filters did not converge on the foundation model: %+v", filtered)
	}

	agent := prepareExplorerDirectory(explorerData{}, items, url.Values{"q": {"urn:mgs:agent:gene-7"}})
	if agent.Total != 1 || agent.Items[0].IdentityName != "identity-7" {
		t.Fatalf("Agent ID search did not find the identity: %+v", agent)
	}
}

func TestHTTPExplorerRendersPaginatedDirectory(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), false)
	defer store.Close()
	controller, did, method := testKey(t, 22)
	var oldestGene string
	for index := 0; index < explorerPageSize+1; index++ {
		name := "agent-" + string(rune('a'+index))
		submission, err := store.Submit(t.Context(), signedGenesis(t, controller, did, method, name, "example.org"))
		if err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			oldestGene = submission.Metadata.Gene
		}
	}
	checkpointKey, checkpointDID, checkpointMethod := testKey(t, 23)
	service, err := NewHTTPService(HTTPConfig{
		Store: store, AdminToken: strings.Repeat("a", 32), CheckpointKey: checkpointKey,
		CheckpointDID: checkpointDID, CheckpointMethod: checkpointMethod,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RatePerMinute: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()

	firstPage := readBody(get(t, server.URL+"/"))
	if !strings.Contains(firstPage, "Page 1 of 2") || !strings.Contains(firstPage, "Showing 1–24 of 25") ||
		!strings.Contains(firstPage, "rel=\"next\"") || strings.Contains(firstPage, "/genes/"+oldestGene) {
		t.Fatal("first rendered directory page did not expose the expected window")
	}
	secondPage := readBody(get(t, server.URL+"/?page=2"))
	if !strings.Contains(secondPage, "Page 2 of 2") || !strings.Contains(secondPage, "Showing 25–25 of 25") ||
		!strings.Contains(secondPage, "rel=\"prev\"") || !strings.Contains(secondPage, "/genes/"+oldestGene) {
		t.Fatal("second rendered directory page did not expose the remaining identity")
	}
}

func TestHTTPRejectsOversizeWrongMediaAndUnknownInput(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), false)
	defer store.Close()
	key, did, method := testKey(t, 30)
	service, err := NewHTTPService(HTTPConfig{
		Store: store, AdminToken: strings.Repeat("a", 32), CheckpointKey: key,
		CheckpointDID: did, CheckpointMethod: method,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RatePerMinute: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/v1/records", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	response, _ := http.DefaultClient.Do(request)
	if response.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("wrong media status %d", response.StatusCode)
	}
	response.Body.Close()
	request, _ = http.NewRequest(http.MethodPost, server.URL+"/v1/records", strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	request.Header.Set("Content-Type", recordMediaType)
	response, _ = http.DefaultClient.Do(request)
	if response.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversize status %d", response.StatusCode)
	}
	response.Body.Close()
}

func TestHTTPNamespaceChallenge(t *testing.T) {
	store := openTestStore(t, filepath.Join(t.TempDir(), "registry.db"), true)
	defer store.Close()
	key, did, method := testKey(t, 40)
	resolver := fakeTXTResolver{}
	service, err := NewHTTPService(HTTPConfig{
		Store: store, AdminToken: strings.Repeat("a", 32), CheckpointKey: key,
		CheckpointDID: did, CheckpointMethod: method, Resolver: resolver,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), RatePerMinute: 1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(service.Handler())
	defer server.Close()
	body, _ := json.Marshal(map[string]string{"namespace": "agents.example", "controllerDid": did})
	response, err := http.Post(server.URL+"/v1/namespace-challenges", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var challenge NamespaceChallenge
	decodeBody(t, response, &challenge)
	if challenge.ID == "" || challenge.RecordValue == "" {
		t.Fatalf("incomplete challenge: %+v", challenge)
	}
	token := strings.TrimPrefix(challenge.RecordValue, "mgs-registry=")
	resolver[challenge.RecordName] = []string{challenge.RecordValue}
	verifyBody, _ := json.Marshal(map[string]string{"token": token})
	response, err = http.Post(server.URL+"/v1/namespace-challenges/"+challenge.ID+"/verify", "application/json", bytes.NewReader(verifyBody))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("verify challenge: %d %s", response.StatusCode, readBody(response))
	}
	response.Body.Close()
}

func signedGenesisWithParent(t *testing.T, key ed25519.PrivateKey, did, method, name, namespace, parentGene string) []byte {
	t.Helper()
	record := mgs.GenesisRecord{
		MGS: mgs.Version, RecordType: mgs.RecordGenesis,
		Identity: mgs.Identity{
			Name: name, Namespace: namespace, SubjectType: "agent", Version: "1.0.0", DID: did,
			CreatedDate: "2026-07-20", AgentName: &mgs.AgentNameClaim{Given: name},
		},
		Parentage: []mgs.LineageEdge{{
			EdgeType: "model-instance-of", ParentGene: parentGene, Authorized: false,
		}},
		Controller: mgs.Controller{DID: did, GenesisKeyID: method},
		Genesis: mgs.GenesisMetadata{
			Timestamp: testTime.Format(time.RFC3339), ArtifactCommitments: []mgs.ArtifactCommitment{},
		},
		Capabilities: []mgs.CapabilityClaim{}, TrainingData: []mgs.TrainingDataDeclaration{},
	}
	unsigned, _ := json.Marshal(record)
	secured, err := mgs.SignRecord(unsigned, mgs.NewProofOptions(testTime, method), key)
	if err != nil {
		t.Fatal(err)
	}
	return secured
}

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func readBody(response *http.Response) string {
	return string(readBodyBytes(response))
}

func readBodyBytes(response *http.Response) []byte {
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return body
}

func decodeBody(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
