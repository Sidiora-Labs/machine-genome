package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
	"github.com/paxlabs-inc/machine-genome/registry"
)

var (
	buildVersion = mgs.Version
	buildCommit  = "unknown"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "mgs:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stderr)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "keygen":
		return runKeygen(args[1:], stdout, stderr)
	case "init-genesis":
		return runInitGenesis(args[1:], stdout, stderr)
	case "sign":
		return runSign(args[1:], stdout, stderr)
	case "authorize-edge":
		return runAuthorize(args[1:], stdout, stderr)
	case "gene":
		return runGene(args[1:], stdout, stderr)
	case "digest":
		return runDigest(args[1:], stdout, stderr)
	case "artifact-digest":
		return runArtifactDigest(args[1:], stdout, stderr)
	case "verify":
		return runVerify(args[1:], stdout, stderr)
	case "verify-chain":
		return runVerifyChain(args[1:], stdout, stderr)
	case "canonicalize":
		return runCanonicalize(args[1:], stdout, stderr)
	case "registry-serve":
		return runRegistryServe(args[1:], stdout, stderr)
	case "registry-backup":
		return runRegistryBackup(args[1:], stdout, stderr)
	case "registry-verify":
		return runRegistryVerify(args[1:], stdout, stderr)
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		usage(stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, `Machine Genome reference CLI

Usage:
  mgs keygen --out private.key.json
  mgs init-genesis --key private.key.json --name NAME --namespace DNS_OR_DID \
    --subject-type agent --version 1.0.0 --out genesis.unsigned.json
  mgs sign --in unsigned.json --key private.key.json --out secured.json [--created RFC3339]
  mgs authorize-edge --in child.json --edge 0 --key parent.key.json --out child.json [--created RFC3339]
  mgs gene --in secured-genesis.json
  mgs digest --in canonicalizable-object.json
  mgs artifact-digest --in artifact.bin
  mgs verify --in secured-record.json
  mgs verify-chain secured-genesis.json amendment-1.json [amendment-2.json ...]
  mgs canonicalize --in record.json
  mgs registry-serve --data-dir DIR --checkpoint-key registry.key.json \
    --admin-token-file admin.token [--listen 127.0.0.1:8080]
  mgs registry-backup --data-dir DIR --out registry-backup.db
  mgs registry-verify --data-dir DIR
  mgs version [--json]`)
}

func runVersion(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	asJSON := flags.Bool("json", false, "emit machine-readable build metadata")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("version accepts only --json")
	}
	if *asJSON {
		return json.NewEncoder(stdout).Encode(map[string]string{
			"version": buildVersion,
			"commit":  buildCommit,
			"go":      runtime.Version(),
		})
	}
	fmt.Fprintf(stdout, "mgs %s (commit %s, %s)\n", buildVersion, buildCommit, runtime.Version())
	return nil
}

func runInitGenesis(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("init-genesis", flag.ContinueOnError)
	flags.SetOutput(stderr)
	keyPath := flags.String("key", "", "controller private key file")
	name := flags.String("name", "", "subject name")
	namespace := flags.String("namespace", "", "subject namespace")
	subjectType := flags.String("subject-type", "agent", "MGS subject type")
	version := flags.String("version", "", "subject version")
	description := flags.String("description", "", "optional description")
	timestamp := flags.String("timestamp", "", "genesis timestamp; defaults to current UTC second")
	output := flags.String("out", "-", "unsigned genesis output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *keyPath == "" || *name == "" || *namespace == "" || *version == "" || flags.NArg() != 0 {
		return errors.New("init-genesis requires --key, --name, --namespace, and --version")
	}
	keyFile, _, err := mgs.LoadPrivateKeyFile(*keyPath)
	if err != nil {
		return err
	}
	when := time.Now().UTC().Truncate(time.Second)
	if *timestamp != "" {
		when, err = time.Parse(time.RFC3339, *timestamp)
		if err != nil {
			return fmt.Errorf("timestamp: %w", err)
		}
	}
	record := mgs.GenesisRecord{
		MGS:        mgs.Version,
		RecordType: mgs.RecordGenesis,
		Identity: mgs.Identity{
			Name: *name, Namespace: *namespace, SubjectType: *subjectType,
			Version: *version, Description: *description, DID: keyFile.DID,
		},
		Parentage: []mgs.LineageEdge{},
		Controller: mgs.Controller{
			DID: keyFile.DID, GenesisKeyID: keyFile.VerificationMethod,
		},
		Genesis: mgs.GenesisMetadata{
			Timestamp:           when.UTC().Truncate(time.Second).Format(time.RFC3339),
			ArtifactCommitments: []mgs.ArtifactCommitment{},
		},
	}
	raw, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if _, err := mgs.ValidateGenesis(raw, false); err != nil {
		return err
	}
	return writeResult(*output, raw, stdout)
}

func runKeygen(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("keygen", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("out", "", "new private key file")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *output == "" || flags.NArg() != 0 {
		return errors.New("keygen requires exactly --out")
	}
	key, err := mgs.GeneratePrivateKeyFile(*output)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created %s with mode 0600\nDID: %s\nverification method: %s\n", *output, key.DID, key.VerificationMethod)
	return nil
}

func runSign(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("sign", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "unsigned MGS record")
	keyPath := flags.String("key", "", "private key file")
	output := flags.String("out", "-", "secured record, or - for stdout")
	created := flags.String("created", "", "proof creation timestamp; defaults to record timestamp")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || *keyPath == "" || flags.NArg() != 0 {
		return errors.New("sign requires --in and --key")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	keyFile, privateKey, err := mgs.LoadPrivateKeyFile(*keyPath)
	if err != nil {
		return err
	}
	timestamp := *created
	if timestamp == "" {
		timestamp, err = recordTimestamp(raw)
		if err != nil {
			return err
		}
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return fmt.Errorf("created: %w", err)
	}
	secured, err := mgs.SignRecord(raw, mgs.NewProofOptions(parsed, keyFile.VerificationMethod), privateKey)
	if err != nil {
		return err
	}
	if err := writeResult(*output, secured, stdout); err != nil {
		return err
	}
	return nil
}

func runAuthorize(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("authorize-edge", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "unsigned child genesis")
	keyPath := flags.String("key", "", "parent private key file")
	output := flags.String("out", "-", "updated child genesis")
	edge := flags.Int("edge", -1, "zero-based parentage edge index")
	created := flags.String("created", "", "proof creation timestamp; defaults to genesis timestamp")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || *keyPath == "" || *edge < 0 || flags.NArg() != 0 {
		return errors.New("authorize-edge requires --in, --edge, and --key")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	keyFile, privateKey, err := mgs.LoadPrivateKeyFile(*keyPath)
	if err != nil {
		return err
	}
	timestamp := *created
	if timestamp == "" {
		timestamp, err = recordTimestamp(raw)
		if err != nil {
			return err
		}
	}
	parsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return err
	}
	updated, err := mgs.AuthorizeEdge(raw, *edge, mgs.NewProofOptions(parsed, keyFile.VerificationMethod), privateKey)
	if err != nil {
		return err
	}
	return writeResult(*output, updated, stdout)
}

func runGene(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("gene", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "secured genesis")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("gene requires --in")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	gene, err := mgs.Gene(raw)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, gene)
	return nil
}

func runDigest(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("digest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "JSON object to canonicalize and digest")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("digest requires --in")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	digest, err := mgs.Digest(raw)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, digest)
	return nil
}

func runArtifactDigest(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("artifact-digest", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "artifact bytes, or - for stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("artifact-digest requires --in")
	}
	var raw []byte
	var err error
	if *input == "-" {
		raw, err = io.ReadAll(io.LimitReader(os.Stdin, 1<<30))
	} else {
		raw, err = os.ReadFile(*input)
	}
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, mgs.ArtifactDigest(raw))
	return nil
}

func runVerify(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "secured MGS record")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("verify requires --in")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	var header struct {
		RecordType string `json:"recordType"`
	}
	if err := strictDecode(raw, &header); err != nil {
		return err
	}
	ctx := context.Background()
	var identifier string
	switch header.RecordType {
	case mgs.RecordGenesis:
		identifier, err = mgs.VerifyGenesis(ctx, raw, mgs.DIDKeyResolver{})
	case mgs.RecordAttestation:
		identifier, err = mgs.VerifyAttestation(ctx, raw, mgs.DIDKeyResolver{})
	case mgs.RecordAmendment:
		identifier, err = mgs.VerifyAmendment(ctx, raw, mgs.DIDKeyResolver{})
	default:
		err = fmt.Errorf("unsupported recordType %q", header.RecordType)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "verified %s %s\n", header.RecordType, identifier)
	return nil
}

func runVerifyChain(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("verify-chain", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() < 1 {
		return errors.New("verify-chain requires genesis followed by zero or more amendments")
	}
	genesis, err := os.ReadFile(flags.Arg(0))
	if err != nil {
		return err
	}
	amendments := make([][]byte, 0, flags.NArg()-1)
	for _, path := range flags.Args()[1:] {
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		amendments = append(amendments, raw)
	}
	state, err := mgs.VerifyAmendmentChain(context.Background(), genesis, amendments, mgs.DIDKeyResolver{})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "verified gene %s through seq %d; active controller %s\n", state.Gene, state.LastSequence, state.Controller)
	return nil
}

func runCanonicalize(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("canonicalize", flag.ContinueOnError)
	flags.SetOutput(stderr)
	input := flags.String("in", "", "JSON record")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *input == "" || flags.NArg() != 0 {
		return errors.New("canonicalize requires --in")
	}
	raw, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	canonical, err := mgs.Canonicalize(raw)
	if err != nil {
		return err
	}
	_, err = stdout.Write(append(canonical, '\n'))
	return err
}

func writeResult(path string, data []byte, stdout io.Writer) error {
	if path == "-" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func recordTimestamp(raw []byte) (string, error) {
	var record struct {
		RecordType string `json:"recordType"`
		Timestamp  string `json:"timestamp"`
		Genesis    struct {
			Timestamp string `json:"timestamp"`
		} `json:"genesis"`
	}
	if err := strictDecode(raw, &record); err != nil {
		return "", err
	}
	if record.RecordType == mgs.RecordGenesis {
		return record.Genesis.Timestamp, nil
	}
	return record.Timestamp, nil
}

func strictDecode(raw []byte, target any) error {
	if _, err := mgs.Canonicalize(raw); err != nil {
		return err
	}
	// Canonicalize already performed the security-sensitive parse.
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("decode record: %w", err)
	}
	return nil
}

func runRegistryServe(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("registry-serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "private registry data directory")
	listen := flags.String("listen", "127.0.0.1:8080", "HTTP listen address")
	checkpointKey := flags.String("checkpoint-key", "", "registry checkpoint Ed25519 key file")
	adminTokenFile := flags.String("admin-token-file", "", "file containing a 32-byte-or-longer administrator token")
	publicBaseURL := flags.String("public-base-url", "", "external registry base URL")
	backupDir := flags.String("backup-dir", "", "private online-backup directory; defaults below data-dir")
	tlsCert := flags.String("tls-cert", "", "TLS certificate chain")
	tlsKey := flags.String("tls-key", "", "TLS private key")
	allowInsecurePublic := flags.Bool("allow-insecure-public", false, "allow cleartext HTTP on a non-loopback address")
	allowUnverifiedNamespaces := flags.Bool("allow-unverified-namespaces", false, "development only: accept DNS namespaces without a challenge")
	rate := flags.Int("rate-per-minute", 120, "per-client request limit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dataDir == "" || *checkpointKey == "" || *adminTokenFile == "" || flags.NArg() != 0 {
		return errors.New("registry-serve requires --data-dir, --checkpoint-key, and --admin-token-file")
	}
	if (*tlsCert == "") != (*tlsKey == "") {
		return errors.New("--tls-cert and --tls-key must be provided together")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil {
		return fmt.Errorf("listen address: %w", err)
	}
	if *tlsCert == "" && !*allowInsecurePublic && !isLoopbackHost(host) {
		return errors.New("refusing cleartext registry on a non-loopback address; configure TLS or explicitly use --allow-insecure-public behind a trusted TLS proxy")
	}
	token, err := readSecretFile(*adminTokenFile)
	if err != nil {
		return fmt.Errorf("admin token: %w", err)
	}
	keyFile, privateKey, err := mgs.LoadPrivateKeyFile(*checkpointKey)
	if err != nil {
		return fmt.Errorf("checkpoint key: %w", err)
	}
	store, err := registry.Open(filepath.Join(*dataDir, "registry.db"), registry.Options{
		RequireNamespaceGrant: !*allowUnverifiedNamespaces,
	})
	if err != nil {
		return err
	}
	defer store.Close()
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	service, err := registry.NewHTTPService(registry.HTTPConfig{
		Store: store, AdminToken: token, CheckpointKey: privateKey,
		CheckpointDID: keyFile.DID, CheckpointMethod: keyFile.VerificationMethod,
		Logger: logger, PublicBaseURL: *publicBaseURL, RatePerMinute: *rate,
		BackupDir: func() string {
			if *backupDir != "" {
				return *backupDir
			}
			return filepath.Join(*dataDir, "backups")
		}(),
	})
	if err != nil {
		return err
	}
	server := &http.Server{
		Addr: *listen, Handler: service.Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		MaxHeaderBytes: 32 << 10,
	}
	signals, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serverError := make(chan error, 1)
	go func() {
		if *tlsCert != "" {
			serverError <- server.ListenAndServeTLS(*tlsCert, *tlsKey)
		} else {
			serverError <- server.ListenAndServe()
		}
	}()
	fmt.Fprintf(stdout, "Machine Genome registry listening on %s\n", *listen)
	select {
	case err := <-serverError:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-signals.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		if err := <-serverError; !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	return nil
}

func runRegistryBackup(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("registry-backup", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "registry data directory")
	output := flags.String("out", "", "new backup database path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dataDir == "" || *output == "" || flags.NArg() != 0 {
		return errors.New("registry-backup requires --data-dir and --out")
	}
	store, err := registry.Open(filepath.Join(*dataDir, "registry.db"), registry.Options{})
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Backup(*output); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created consistent registry backup %s\n", *output)
	return nil
}

func runRegistryVerify(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("registry-verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dataDir := flags.String("data-dir", "", "registry data directory")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *dataDir == "" || flags.NArg() != 0 {
		return errors.New("registry-verify requires --data-dir")
	}
	store, err := registry.Open(filepath.Join(*dataDir, "registry.db"), registry.Options{})
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Verify(); err != nil {
		return err
	}
	objects, genes, size, err := store.Stats()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "verified registry: %d objects, %d Genes, transparency tree size %d\n", objects, genes, size)
	return nil
}

func readSecretFile(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Mode().Perm()&0o077 != 0 && !isSystemdCredential(path) {
		return "", errors.New("secret file is accessible by group or other users")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if len(value) < 32 {
		return "", errors.New("secret must contain at least 32 bytes")
	}
	return value, nil
}

func isSystemdCredential(path string) bool {
	directory := filepath.Clean(os.Getenv("CREDENTIALS_DIRECTORY"))
	if directory == "." || !strings.HasPrefix(directory, "/run/credentials/") {
		return false
	}
	return filepath.Dir(filepath.Clean(path)) == directory
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
