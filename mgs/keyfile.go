package mgs

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PrivateKeyFile struct {
	Format             string `json:"format"`
	Seed               string `json:"seed"`
	DID                string `json:"did"`
	VerificationMethod string `json:"verificationMethod"`
}

func GeneratePrivateKeyFile(path string) (*PrivateKeyFile, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 key: %w", err)
	}
	did, method, err := DIDKeyFromPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	file := &PrivateKeyFile{
		Format:             "mgs-ed25519-seed-v1",
		Seed:               base64.RawURLEncoding.EncodeToString(privateKey.Seed()),
		DID:                did,
		VerificationMethod: method,
	}
	encoded, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return nil, err
	}
	encoded = append(encoded, '\n')
	handle, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create private key file: %w", err)
	}
	if _, err := handle.Write(encoded); err != nil {
		handle.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("write private key file: %w", err)
	}
	if err := handle.Close(); err != nil {
		return nil, fmt.Errorf("close private key file: %w", err)
	}
	return file, nil
}

func LoadPrivateKeyFile(path string) (*PrivateKeyFile, ed25519.PrivateKey, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("stat private key file: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 && !isSystemdCredential(path) {
		return nil, nil, errors.New("private key file is accessible by group or other users")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read private key file: %w", err)
	}
	var file PrivateKeyFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, nil, fmt.Errorf("decode private key file: %w", err)
	}
	if file.Format != "mgs-ed25519-seed-v1" {
		return nil, nil, fmt.Errorf("unsupported private key format %q", file.Format)
	}
	seed, err := base64.RawURLEncoding.DecodeString(file.Seed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, nil, errors.New("private key file contains an invalid Ed25519 seed")
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	did, method, err := DIDKeyFromPublicKey(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		return nil, nil, err
	}
	if did != file.DID || method != file.VerificationMethod {
		return nil, nil, errors.New("private key metadata does not match its seed")
	}
	return &file, privateKey, nil
}

func isSystemdCredential(path string) bool {
	directory := filepath.Clean(os.Getenv("CREDENTIALS_DIRECTORY"))
	if directory == "." || !strings.HasPrefix(directory, "/run/credentials/") {
		return false
	}
	return filepath.Dir(filepath.Clean(path)) == directory
}
