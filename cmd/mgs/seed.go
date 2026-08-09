package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
)

type foundationCatalog struct {
	Models map[string]foundationCatalogModel `json:"models"`
}

var foundationModelIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

type foundationCatalogModel struct {
	Providers   []string        `json:"providers"`
	ModelFamily string          `json:"model_family"`
	Features    map[string]bool `json:"features"`
	Modalities  struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
	TokenCosts struct {
		TrainingCutoff *string `json:"training_cutoff"`
		ContextWindow  uint64  `json:"context_window"`
	} `json:"token_costs"`
}

type foundationSeedManifest struct {
	Format        string                 `json:"format"`
	MGS           string                 `json:"mgs"`
	CatalogDigest string                 `json:"catalogDigest"`
	Observer      string                 `json:"observer"`
	ObservedAt    string                 `json:"observedAt"`
	Records       []foundationSeedRecord `json:"records"`
}

type foundationSeedRecord struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	File     string `json:"file"`
	Gene     string `json:"gene"`
}

func runSeedFoundations(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("seed-foundations", flag.ContinueOnError)
	flags.SetOutput(stderr)
	catalogPath := flags.String("catalog", "", "model catalog JSON")
	keyPath := flags.String("key", "", "observer private key file")
	outputDir := flags.String("out-dir", "", "new directory for signed model genesis records")
	timestamp := flags.String("timestamp", "", "observation timestamp; defaults to current UTC second")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *catalogPath == "" || *keyPath == "" || *outputDir == "" || flags.NArg() != 0 {
		return errors.New("seed-foundations requires --catalog, --key, and --out-dir")
	}
	raw, err := os.ReadFile(*catalogPath)
	if err != nil {
		return fmt.Errorf("read catalog: %w", err)
	}
	var catalog foundationCatalog
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	if err := decoder.Decode(&catalog); err != nil {
		return fmt.Errorf("decode catalog: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("catalog must contain exactly one JSON object")
	}
	if len(catalog.Models) == 0 {
		return errors.New("catalog.models must contain at least one model")
	}
	keyFile, privateKey, err := mgs.LoadPrivateKeyFile(*keyPath)
	if err != nil {
		return err
	}
	when := time.Now().UTC().Truncate(time.Second)
	if *timestamp != "" {
		when, err = time.Parse(time.RFC3339, *timestamp)
		if err != nil {
			return fmt.Errorf("timestamp: %w", err)
		}
		when = when.UTC().Truncate(time.Second)
	}
	if err := os.Mkdir(*outputDir, 0o755); err != nil {
		return fmt.Errorf("create output directory (must not already exist): %w", err)
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(*outputDir)
		}
	}()

	digest := mgs.ArtifactDigest(raw)
	manifest := foundationSeedManifest{
		Format: "mgs-foundation-seed-v1", MGS: mgs.Version, CatalogDigest: digest,
		Observer: keyFile.DID, ObservedAt: when.Format(time.RFC3339),
	}
	names := make([]string, 0, len(catalog.Models))
	for name := range catalog.Models {
		names = append(names, name)
	}
	sort.Strings(names)
	seenNames := make(map[string]bool, len(names))
	for _, name := range names {
		candidate := catalog.Models[name]
		if !foundationModelIDPattern.MatchString(name) {
			return fmt.Errorf("model id %q is not safe for a seed filename", name)
		}
		canonicalName := strings.ToLower(name)
		if seenNames[canonicalName] {
			return fmt.Errorf("model id %q duplicates another id case-insensitively", name)
		}
		seenNames[canonicalName] = true
		if len(candidate.Providers) == 0 || strings.TrimSpace(candidate.Providers[0]) == "" {
			return fmt.Errorf("model %q has no provider", name)
		}
		record := foundationGenesis(name, candidate, keyFile.DID, keyFile.VerificationMethod, when, digest, filepath.Base(*catalogPath))
		unsigned, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			return err
		}
		unsigned = append(unsigned, '\n')
		secured, err := mgs.SignRecord(unsigned, mgs.NewProofOptions(when, keyFile.VerificationMethod), privateKey)
		if err != nil {
			return fmt.Errorf("sign model %q: %w", name, err)
		}
		gene, err := mgs.VerifyGenesis(context.Background(), secured, mgs.DIDKeyResolver{})
		if err != nil {
			return fmt.Errorf("verify model %q: %w", name, err)
		}
		fileName := "model-" + name + ".json"
		if err := os.WriteFile(filepath.Join(*outputDir, fileName), secured, 0o644); err != nil {
			return err
		}
		manifest.Records = append(manifest.Records, foundationSeedRecord{
			Name: name, Provider: candidate.Providers[0], File: fileName, Gene: gene,
		})
	}
	manifestRaw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*outputDir, "seed-manifest.json"), append(manifestRaw, '\n'), 0o644); err != nil {
		return err
	}
	ok = true
	fmt.Fprintf(stdout, "wrote %d signed foundation records to %s (catalog %s)\n", len(manifest.Records), *outputDir, digest)
	return nil
}

func foundationGenesis(name string, source foundationCatalogModel, did, method string, when time.Time, catalogDigest, catalogName string) mgs.GenesisRecord {
	provider := source.Providers[0]
	capabilities := make([]mgs.CapabilityClaim, 0)
	featureNames := make([]string, 0, len(source.Features))
	for feature, supported := range source.Features {
		if supported {
			featureNames = append(featureNames, feature)
		}
	}
	sort.Strings(featureNames)
	for _, feature := range featureNames {
		label := strings.ReplaceAll(feature, "_", " ")
		capabilities = append(capabilities, mgs.CapabilityClaim{
			ID: "feature." + strings.ReplaceAll(feature, "_", "-"), Name: label,
			Category: "model-feature", Level: "working",
			Description: "Catalog reports support; this is not a comparative strength score.",
		})
	}
	for _, modality := range append(append([]string{}, source.Modalities.Input...), source.Modalities.Output...) {
		id := "modality." + strings.ToLower(strings.ReplaceAll(modality, "_", "-"))
		already := false
		for _, capability := range capabilities {
			if capability.ID == id {
				already = true
			}
		}
		if !already {
			capabilities = append(capabilities, mgs.CapabilityClaim{
				ID: id, Name: modality, Category: "modality", Level: "working",
			})
		}
	}
	training := mgs.TrainingDataDeclaration{
		Stage: "pretraining", Name: "Provider training corpus", Disclosure: "unknown",
		Description: "The seed catalog does not enumerate the underlying training datasets.",
	}
	if source.TokenCosts.TrainingCutoff != nil && *source.TokenCosts.TrainingCutoff != "" {
		cutoff := *source.TokenCosts.TrainingCutoff
		if len(cutoff) == 7 {
			cutoff += "-01"
		}
		training.DataCutoff = cutoff
		training.Disclosure = "summary"
	}
	return mgs.GenesisRecord{
		MGS: mgs.Version, RecordType: mgs.RecordGenesis,
		Identity: mgs.Identity{
			Name: name, Namespace: did, SubjectType: "model", Version: name,
			Description: "Third-party foundation-model observation generated from the supplied catalog; not a vendor endorsement.",
			CreatedDate: when.Format("2006-01-02"),
		},
		Parentage:  []mgs.LineageEdge{},
		Controller: mgs.Controller{DID: did, GenesisKeyID: method},
		Genesis: mgs.GenesisMetadata{
			Timestamp: when.Format(time.RFC3339),
			ArtifactCommitments: []mgs.ArtifactCommitment{{
				Kind: "foundation-model-catalog", Hash: catalogDigest, Present: false,
				Note: "Raw-byte commitment to " + catalogName + "; retained outside this signed record.",
			}},
		},
		Capabilities: capabilities,
		TrainingData: []mgs.TrainingDataDeclaration{training},
		Model: &mgs.ModelProfile{
			Provider: provider, Providers: append([]string(nil), source.Providers...),
			ModelID: name, Family: source.ModelFamily, ContextWindow: source.TokenCosts.ContextWindow,
			Foundation: true,
		},
	}
}
