package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/machine-genome/mgs"
	"github.com/Sidiora-Labs/machine-genome/registry"
)

func TestVersionHumanAndJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"version"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "mgs "+buildVersion) {
		t.Fatalf("unexpected version output: %s", stdout.String())
	}
	stdout.Reset()
	if err := run([]string{"version", "--json"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	var value map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if value["version"] != buildVersion || value["go"] == "" {
		t.Fatalf("incomplete version metadata: %+v", value)
	}
}

func TestCLIGenesisRoundTrip(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "controller.key.json")
	unsigned := filepath.Join(directory, "genesis.unsigned.json")
	secured := filepath.Join(directory, "genesis.json")
	var stdout, stderr bytes.Buffer

	if err := run([]string{"keygen", "--out", key}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(key)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode is %o", info.Mode().Perm())
	}
	if err := run([]string{
		"init-genesis", "--key", key, "--name", "cli-agent",
		"--namespace", "example.test", "--subject-type", "agent",
		"--version", "1.0.0", "--timestamp", "2026-07-20T14:00:00Z",
		"--out", unsigned,
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{
		"sign", "--in", unsigned, "--key", key, "--out", secured,
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := run([]string{"verify", "--in", secured}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "verified genesis zQm") {
		t.Fatalf("unexpected verify output: %s", stdout.String())
	}
	stdout.Reset()
	if err := run([]string{"gene", "--in", secured}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(stdout.String(), "zQm") {
		t.Fatalf("unexpected Gene: %s", stdout.String())
	}
}

func TestSeedFoundationsFromCatalog(t *testing.T) {
	directory := t.TempDir()
	key := filepath.Join(directory, "observer.key.json")
	catalog := filepath.Join(directory, "models.json")
	output := filepath.Join(directory, "seed")
	if err := os.WriteFile(catalog, []byte(`{"models":{"example-pro":{"providers":["example"],"model_family":"example","features":{"vision":true,"tools":true},"modalities":{"input":["text"],"output":["text"]},"token_costs":{"training_cutoff":"2026-01","context_window":128000}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := run([]string{"keygen", "--out", key}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := run([]string{
		"seed-foundations", "--catalog", catalog, "--key", key, "--out-dir", output,
		"--timestamp", "2026-08-09T18:00:00Z",
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "wrote 1 signed foundation records") {
		t.Fatalf("unexpected seed output: %s", stdout.String())
	}
	raw, err := os.ReadFile(filepath.Join(output, "model-example-pro.json"))
	if err != nil {
		t.Fatal(err)
	}
	record, err := mgs.ValidateGenesis(raw, true)
	if err != nil {
		t.Fatal(err)
	}
	if record.Model == nil || !record.Model.Foundation || record.Model.Family != "example" ||
		record.Model.ContextWindow != 128000 || len(record.Capabilities) != 3 {
		t.Fatalf("unexpected foundation record: %+v", record)
	}
	if len(record.TrainingData) != 1 || record.TrainingData[0].DataCutoff != "2026-01-01" {
		t.Fatalf("unexpected training disclosure: %+v", record.TrainingData)
	}
}

func TestRepositoryFoundationCatalogGeneratesAllRecords(t *testing.T) {
	catalog := filepath.Join("..", "..", "records", "foundation", "models.json")
	if _, err := os.Stat(catalog); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	key := filepath.Join(directory, "observer.key.json")
	output := filepath.Join(directory, "seed")
	var stdout, stderr bytes.Buffer
	if err := run([]string{"keygen", "--out", key}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if err := run([]string{
		"seed-foundations", "--catalog", catalog, "--key", key, "--out-dir", output,
		"--timestamp", "2026-08-09T18:00:00Z",
	}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(output, "seed-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest foundationSeedManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Records) != 160 {
		t.Fatalf("generated %d foundation records, want 160", len(manifest.Records))
	}
	store, err := registry.Open(filepath.Join(directory, "registry.db"), registry.Options{RequireNamespaceGrant: true})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, item := range manifest.Records {
		recordRaw, err := os.ReadFile(filepath.Join(output, item.File))
		if err != nil {
			t.Fatal(err)
		}
		gene, err := mgs.VerifyGenesis(context.Background(), recordRaw, mgs.DIDKeyResolver{})
		if err != nil || gene != item.Gene {
			t.Fatalf("verify %s: gene=%s want=%s err=%v", item.Name, gene, item.Gene, err)
		}
		if submission, err := store.Submit(context.Background(), recordRaw); err != nil || !submission.Created {
			t.Fatalf("submit %s: submission=%+v err=%v", item.Name, submission, err)
		}
	}
	_, genes, treeSize, err := store.Stats()
	if err != nil || genes != 160 || treeSize != 160 {
		t.Fatalf("foundation registry stats: genes=%d tree=%d err=%v", genes, treeSize, err)
	}
	result, err := store.Search("", 250)
	if err != nil || len(result.Items) != 160 {
		t.Fatalf("foundation directory: items=%d err=%v", len(result.Items), err)
	}
}

func TestCLIRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"not-a-command"}, &stdout, &stderr); err == nil {
		t.Fatal("unknown command succeeded")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatal("unknown command did not print usage")
	}
}
