package mgs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrivateKeyFilePermissionsAndIntegrity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "controller.key.json")
	generated, err := GeneratePrivateKeyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode is %o, want 600", info.Mode().Perm())
	}
	loaded, _, err := LoadPrivateKeyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DID != generated.DID || loaded.VerificationMethod != generated.VerificationMethod {
		t.Fatal("loaded key metadata changed")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadPrivateKeyFile(path); err == nil {
		t.Fatal("world-readable private key was accepted")
	}
}
