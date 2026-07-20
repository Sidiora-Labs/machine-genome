package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestCLIRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"not-a-command"}, &stdout, &stderr); err == nil {
		t.Fatal("unknown command succeeded")
	}
	if !strings.Contains(stderr.String(), "Usage:") {
		t.Fatal("unknown command did not print usage")
	}
}
