package registry

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
	bolt "go.etcd.io/bbolt"
)

var (
	ErrNotFound          = errors.New("registry object not found")
	ErrConflict          = errors.New("registry conflict")
	ErrNamespaceRequired = errors.New("verified namespace control is required")
	ErrGone              = errors.New("registry object is unavailable")
)

var (
	bucketMeta          = []byte("meta")
	bucketObjects       = []byte("objects")
	bucketObjectMeta    = []byte("object_meta")
	bucketGenes         = []byte("genes")
	bucketIdentity      = []byte("identity")
	bucketAgentNames    = []byte("agent_names")
	bucketAmendments    = []byte("amendments")
	bucketAttestations  = []byte("attestations")
	bucketChildren      = []byte("children")
	bucketEquivocations = []byte("equivocations")
	bucketLog           = []byte("transparency_log")
	bucketAudit         = []byte("audit")
	bucketTombstones    = []byte("tombstones")
	bucketChallenges    = []byte("challenges")
	bucketGrants        = []byte("namespace_grants")
)

var allBuckets = [][]byte{
	bucketMeta, bucketObjects, bucketObjectMeta, bucketGenes, bucketIdentity, bucketAgentNames,
	bucketAmendments, bucketAttestations, bucketChildren, bucketEquivocations,
	bucketLog, bucketAudit, bucketTombstones, bucketChallenges, bucketGrants,
}

type Options struct {
	RequireNamespaceGrant bool
	Clock                 func() time.Time
}

type Store struct {
	db                    *bolt.DB
	requireNamespaceGrant bool
	now                   func() time.Time
	mu                    sync.Mutex
}

type RecordMetadata struct {
	Digest        string   `json:"digest"`
	RecordType    string   `json:"recordType"`
	Gene          string   `json:"gene,omitempty"`
	SubjectGene   string   `json:"subjectGene,omitempty"`
	Sequence      uint64   `json:"sequence,omitempty"`
	IdentityName  string   `json:"identityName,omitempty"`
	AgentID       string   `json:"agentId,omitempty"`
	AgentName     string   `json:"agentName,omitempty"`
	GivenName     string   `json:"givenName,omitempty"`
	FamilyName    string   `json:"familyName,omitempty"`
	CreatedDate   string   `json:"createdDate,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	Foundation    bool     `json:"foundation,omitempty"`
	ModelProvider string   `json:"modelProvider,omitempty"`
	ModelID       string   `json:"modelId,omitempty"`
	ModelFamily   string   `json:"modelFamily,omitempty"`
	ContextWindow uint64   `json:"contextWindow,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	SubjectType   string   `json:"subjectType,omitempty"`
	ControllerDID string   `json:"controllerDid,omitempty"`
	AuthorTime    string   `json:"authorTime"`
	InclusionTime string   `json:"inclusionTime"`
	LogIndex      uint64   `json:"logIndex"`
}

type Submission struct {
	Metadata   RecordMetadata `json:"metadata"`
	Created    bool           `json:"created"`
	Conflict   bool           `json:"conflict,omitempty"`
	ConflictID string         `json:"conflictId,omitempty"`
}

type GeneView struct {
	Genesis       RecordMetadata   `json:"genesis"`
	Amendments    []RecordMetadata `json:"amendments"`
	Attestations  []RecordMetadata `json:"attestations"`
	Children      []string         `json:"children"`
	Equivocations []string         `json:"equivocations"`
	Tombstoned    bool             `json:"tombstoned"`
}

type SearchResult struct {
	Items []RecordMetadata `json:"items"`
}

type Tombstone struct {
	Digest    string `json:"digest"`
	Reason    string `json:"reason"`
	Reference string `json:"reference,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type AuditEvent struct {
	Index    uint64         `json:"index"`
	Time     string         `json:"time"`
	Action   string         `json:"action"`
	Actor    string         `json:"actor"`
	Object   string         `json:"object,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
	Previous string         `json:"previous,omitempty"`
	Hash     string         `json:"hash"`
}

func Open(path string, options Options) (*Store, error) {
	if path == "" {
		return nil, errors.New("registry database path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create registry data directory: %w", err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("protect registry data directory: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open registry database: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		db.Close()
		return nil, fmt.Errorf("protect registry database: %w", err)
	}
	now := options.Clock
	if now == nil {
		now = time.Now
	}
	store := &Store{db: db, requireNamespaceGrant: options.RequireNamespaceGrant, now: now}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, name := range allBuckets {
			if _, err := tx.CreateBucketIfNotExists(name); err != nil {
				return err
			}
		}
		return tx.Bucket(bucketMeta).Put([]byte("schema"), []byte("1"))
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize registry database: %w", err)
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Submit(ctx context.Context, raw []byte) (Submission, error) {
	if err := ctx.Err(); err != nil {
		return Submission{}, err
	}
	if len(raw) == 0 || len(raw) > 1<<20 {
		return Submission{}, errors.New("record must be between 1 byte and 1 MiB")
	}
	canonical, err := mgs.Canonicalize(raw)
	if err != nil {
		return Submission{}, err
	}
	var header struct {
		RecordType string `json:"recordType"`
	}
	if err := json.Unmarshal(canonical, &header); err != nil {
		return Submission{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	switch header.RecordType {
	case mgs.RecordGenesis:
		return s.submitGenesis(ctx, canonical)
	case mgs.RecordAmendment:
		return s.submitAmendment(ctx, canonical)
	case mgs.RecordAttestation:
		return s.submitAttestation(ctx, canonical)
	default:
		return Submission{}, fmt.Errorf("unsupported recordType %q", header.RecordType)
	}
}

func (s *Store) submitGenesis(ctx context.Context, canonical []byte) (Submission, error) {
	record, err := mgs.ValidateGenesis(canonical, true)
	if err != nil {
		return Submission{}, err
	}
	gene, err := mgs.VerifyGenesis(ctx, canonical, mgs.DIDKeyResolver{})
	if err != nil {
		return Submission{}, err
	}
	if err := mgs.VerifyLineage(ctx, canonical, mgs.DIDKeyResolver{}, s); err != nil {
		return Submission{}, err
	}
	if err := s.namespaceAllowed(record.Identity.Namespace, record.Controller.DID); err != nil {
		return Submission{}, err
	}
	meta := RecordMetadata{
		Digest: gene, RecordType: mgs.RecordGenesis, Gene: gene,
		IdentityName: record.Identity.Name, Namespace: record.Identity.Namespace,
		SubjectType: record.Identity.SubjectType, ControllerDID: record.Controller.DID,
		AuthorTime:  record.Genesis.Timestamp,
		CreatedDate: record.Identity.CreatedDate,
	}
	for _, capability := range record.Capabilities {
		meta.Capabilities = append(meta.Capabilities, capability.ID)
	}
	if record.Model != nil {
		meta.Foundation = record.Model.Foundation
		meta.ModelProvider = record.Model.Provider
		meta.ModelID = record.Model.ModelID
		meta.ModelFamily = record.Model.Family
		meta.ContextWindow = record.Model.ContextWindow
	}
	if record.Identity.AgentName != nil {
		agent, deriveErr := mgs.DeriveAgentIdentity(record, gene)
		if deriveErr != nil {
			return Submission{}, deriveErr
		}
		meta.AgentID = agent.ID
		meta.AgentName = agent.Name
		meta.GivenName = agent.GivenName
		meta.FamilyName = agent.FamilyName
	}
	coordinate := identityKey(record.Identity.Namespace, record.Identity.Name, record.Identity.Version)
	return s.insert(canonical, meta, func(tx *bolt.Tx) error {
		if existing := tx.Bucket(bucketIdentity).Get(coordinate); existing != nil && string(existing) != gene {
			return fmt.Errorf("%w: identity coordinate is already registered as %s", ErrConflict, existing)
		}
		if err := tx.Bucket(bucketGenes).Put([]byte(gene), []byte(gene)); err != nil {
			return err
		}
		if err := tx.Bucket(bucketIdentity).Put(coordinate, []byte(gene)); err != nil {
			return err
		}
		if meta.AgentName != "" {
			nameKey := []byte(strings.ToLower(meta.AgentName))
			if existing := tx.Bucket(bucketAgentNames).Get(nameKey); existing != nil && string(existing) != gene {
				return fmt.Errorf("%w: agent name %q is already claimed by %s", ErrConflict, meta.AgentName, existing)
			}
			if err := tx.Bucket(bucketAgentNames).Put(nameKey, []byte(gene)); err != nil {
				return err
			}
		}
		for _, edge := range record.Parentage {
			if !edge.Unresolved {
				if err := tx.Bucket(bucketChildren).Put(joinKey(edge.ParentGene, gene), []byte(gene)); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (s *Store) submitAmendment(ctx context.Context, canonical []byte) (Submission, error) {
	record, err := mgs.ValidateAmendment(canonical, true)
	if err != nil {
		return Submission{}, err
	}
	digest, err := mgs.VerifyAmendment(ctx, canonical, mgs.DIDKeyResolver{})
	if err != nil {
		return Submission{}, err
	}
	genesis, err := s.Get(record.Gene, true)
	if err != nil {
		return Submission{}, fmt.Errorf("resolve amendment genesis: %w", err)
	}
	accepted, err := s.amendmentBytes(record.Gene)
	if err != nil {
		return Submission{}, err
	}
	expected := uint64(len(accepted) + 1)
	if record.Seq < expected {
		prefix := accepted[:record.Seq-1]
		candidate := append(append(make([][]byte, 0, len(prefix)+1), prefix...), canonical)
		if _, verifyErr := mgs.VerifyAmendmentChain(ctx, genesis, candidate, mgs.DIDKeyResolver{}); verifyErr != nil {
			return Submission{}, verifyErr
		}
		var acceptedRecord mgs.AmendmentRecord
		if err := json.Unmarshal(accepted[record.Seq-1], &acceptedRecord); err != nil {
			return Submission{}, err
		}
		acceptedDigest, _ := mgs.Digest(accepted[record.Seq-1])
		if acceptedDigest == digest {
			meta, err := s.Metadata(digest)
			return Submission{Metadata: meta}, err
		}
		if acceptedRecord.Prev != record.Prev {
			return Submission{}, fmt.Errorf("%w: amendment sequence %d is already committed", ErrConflict, record.Seq)
		}
		return s.recordEquivocation(canonical, record, digest, acceptedDigest)
	}
	if record.Seq > expected {
		return Submission{}, fmt.Errorf("%w: amendment sequence %d has a gap; want %d", ErrConflict, record.Seq, expected)
	}
	if _, err := mgs.VerifyAmendmentChain(ctx, genesis, append(accepted, canonical), mgs.DIDKeyResolver{}); err != nil {
		return Submission{}, err
	}
	meta := RecordMetadata{
		Digest: digest, RecordType: mgs.RecordAmendment, Gene: record.Gene,
		Sequence: record.Seq, AuthorTime: record.Timestamp,
	}
	return s.insert(canonical, meta, func(tx *bolt.Tx) error {
		return tx.Bucket(bucketAmendments).Put(sequenceKey(record.Gene, record.Seq), []byte(digest))
	})
}

func (s *Store) recordEquivocation(canonical []byte, record *mgs.AmendmentRecord, digest, accepted string) (Submission, error) {
	now := s.now().UTC().Truncate(time.Second).Format(time.RFC3339)
	meta := RecordMetadata{
		Digest: digest, RecordType: mgs.RecordAmendment, Gene: record.Gene,
		Sequence: record.Seq, AuthorTime: record.Timestamp, InclusionTime: now,
	}
	conflictID := fmt.Sprintf("%s/%d/%s", record.Gene, record.Seq, record.Prev)
	err := s.db.Update(func(tx *bolt.Tx) error {
		value, _ := json.Marshal(struct {
			Record   json.RawMessage `json:"record"`
			Metadata RecordMetadata  `json:"metadata"`
			Accepted string          `json:"accepted"`
		}{canonical, meta, accepted})
		if err := tx.Bucket(bucketEquivocations).Put(joinKey(conflictID, digest), value); err != nil {
			return err
		}
		return s.appendAudit(tx, "equivocation.observed", "public", digest, map[string]any{
			"gene": record.Gene, "sequence": record.Seq, "accepted": accepted,
		})
	})
	if err != nil {
		return Submission{}, err
	}
	return Submission{Metadata: meta, Conflict: true, ConflictID: conflictID}, nil
}

func (s *Store) submitAttestation(ctx context.Context, canonical []byte) (Submission, error) {
	record, err := mgs.ValidateAttestation(canonical, true)
	if err != nil {
		return Submission{}, err
	}
	digest, err := mgs.VerifyAttestation(ctx, canonical, mgs.DIDKeyResolver{})
	if err != nil {
		return Submission{}, err
	}
	if _, err := s.Metadata(record.SubjectGene); err != nil {
		return Submission{}, fmt.Errorf("resolve attestation subject: %w", err)
	}
	meta := RecordMetadata{
		Digest: digest, RecordType: mgs.RecordAttestation,
		SubjectGene: record.SubjectGene, AuthorTime: record.Timestamp,
	}
	return s.insert(canonical, meta, func(tx *bolt.Tx) error {
		return tx.Bucket(bucketAttestations).Put(joinKey(record.SubjectGene, digest), []byte(digest))
	})
}

func (s *Store) insert(canonical []byte, meta RecordMetadata, index func(*bolt.Tx) error) (Submission, error) {
	var submission Submission
	err := s.db.Update(func(tx *bolt.Tx) error {
		if existing := tx.Bucket(bucketObjects).Get([]byte(meta.Digest)); existing != nil {
			stored, err := decodeMetadata(tx.Bucket(bucketObjectMeta).Get([]byte(meta.Digest)))
			submission = Submission{Metadata: stored}
			return err
		}
		if err := index(tx); err != nil {
			return err
		}
		now := s.now().UTC().Truncate(time.Second).Format(time.RFC3339)
		logIndex, err := tx.Bucket(bucketLog).NextSequence()
		if err != nil {
			return err
		}
		meta.InclusionTime = now
		meta.LogIndex = logIndex - 1
		if err := tx.Bucket(bucketObjects).Put([]byte(meta.Digest), canonical); err != nil {
			return err
		}
		encoded, _ := json.Marshal(meta)
		if err := tx.Bucket(bucketObjectMeta).Put([]byte(meta.Digest), encoded); err != nil {
			return err
		}
		if err := tx.Bucket(bucketLog).Put(uint64Key(meta.LogIndex), []byte(meta.Digest)); err != nil {
			return err
		}
		if err := s.appendAudit(tx, "record.accepted", "public", meta.Digest, map[string]any{
			"recordType": meta.RecordType, "logIndex": meta.LogIndex,
		}); err != nil {
			return err
		}
		submission = Submission{Metadata: meta, Created: true}
		return nil
	})
	return submission, err
}

func (s *Store) ResolveGenesis(_ context.Context, gene string) ([]byte, error) {
	return s.Get(gene, true)
}

func (s *Store) Get(digest string, includeGone bool) ([]byte, error) {
	var result []byte
	err := s.db.View(func(tx *bolt.Tx) error {
		if !includeGone && tx.Bucket(bucketTombstones).Get([]byte(digest)) != nil {
			return ErrGone
		}
		value := tx.Bucket(bucketObjects).Get([]byte(digest))
		if value == nil {
			return ErrNotFound
		}
		result = append([]byte(nil), value...)
		return nil
	})
	return result, err
}

func (s *Store) Metadata(digest string) (RecordMetadata, error) {
	var result RecordMetadata
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketObjectMeta).Get([]byte(digest))
		if value == nil {
			return ErrNotFound
		}
		var err error
		result, err = decodeMetadata(value)
		return err
	})
	return result, err
}

func (s *Store) Gene(gene string) (GeneView, error) {
	genesis, err := s.Metadata(gene)
	if err != nil {
		return GeneView{}, err
	}
	result := GeneView{Genesis: genesis}
	err = s.db.View(func(tx *bolt.Tx) error {
		result.Tombstoned = tx.Bucket(bucketTombstones).Get([]byte(gene)) != nil
		if err := collectMetadata(tx, bucketAmendments, []byte(gene+"\x00"), &result.Amendments); err != nil {
			return err
		}
		if err := collectMetadata(tx, bucketAttestations, []byte(gene+"\x00"), &result.Attestations); err != nil {
			return err
		}
		cursor := tx.Bucket(bucketChildren).Cursor()
		for key, value := cursor.Seek([]byte(gene + "\x00")); key != nil && bytes.HasPrefix(key, []byte(gene+"\x00")); key, value = cursor.Next() {
			result.Children = append(result.Children, string(value))
		}
		cursor = tx.Bucket(bucketEquivocations).Cursor()
		for key, _ := cursor.Seek([]byte(gene + "/")); key != nil && bytes.HasPrefix(key, []byte(gene+"/")); key, _ = cursor.Next() {
			result.Equivocations = append(result.Equivocations, string(key))
		}
		return nil
	})
	return result, err
}

func (s *Store) Search(query string, limit int) (SearchResult, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if limit <= 0 || limit > 250 {
		limit = 25
	}
	result := SearchResult{Items: []RecordMetadata{}}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketObjectMeta).ForEach(func(_, value []byte) error {
			meta, err := decodeMetadata(value)
			if err != nil {
				return err
			}
			if meta.RecordType != mgs.RecordGenesis {
				return nil
			}
			classification := ""
			if meta.Foundation {
				classification = "foundation"
			}
			haystack := strings.ToLower(strings.Join([]string{
				meta.Digest, meta.IdentityName, meta.AgentID, meta.AgentName, meta.Namespace,
				meta.SubjectType, meta.ControllerDID, meta.ModelProvider, meta.ModelID,
				meta.ModelFamily, classification, strings.Join(meta.Capabilities, " "),
			}, " "))
			if query == "" || strings.Contains(haystack, query) {
				result.Items = append(result.Items, meta)
				if len(result.Items) > limit {
					sort.Slice(result.Items, func(i, j int) bool {
						return result.Items[i].InclusionTime > result.Items[j].InclusionTime
					})
					result.Items = result.Items[:limit]
				}
			}
			return nil
		})
	})
	sort.Slice(result.Items, func(i, j int) bool {
		return result.Items[i].InclusionTime > result.Items[j].InclusionTime
	})
	if len(result.Items) > limit {
		result.Items = result.Items[:limit]
	}
	return result, err
}

// ListGenes returns every genesis metadata entry in newest-first log order.
// The explorer applies its own filters and slices this result into pages so
// API search limits cannot silently hide identities from the directory.
func (s *Store) ListGenes() ([]RecordMetadata, error) {
	items := []RecordMetadata{}
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketObjectMeta).ForEach(func(_, value []byte) error {
			meta, err := decodeMetadata(value)
			if err != nil {
				return err
			}
			if meta.RecordType == mgs.RecordGenesis {
				items = append(items, meta)
			}
			return nil
		})
	})
	sort.Slice(items, func(i, j int) bool {
		return items[i].LogIndex > items[j].LogIndex
	})
	return items, err
}

func (s *Store) Tombstone(digest, reason, reference, actor string) (Tombstone, error) {
	if strings.TrimSpace(reason) == "" {
		return Tombstone{}, errors.New("tombstone reason is required")
	}
	if _, err := s.Metadata(digest); err != nil {
		return Tombstone{}, err
	}
	record := Tombstone{
		Digest: digest, Reason: strings.TrimSpace(reason), Reference: strings.TrimSpace(reference),
		CreatedAt: s.now().UTC().Truncate(time.Second).Format(time.RFC3339),
	}
	encoded, _ := json.Marshal(record)
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketTombstones).Put([]byte(digest), encoded); err != nil {
			return err
		}
		return s.appendAudit(tx, "record.tombstoned", actor, digest, map[string]any{
			"reason": record.Reason, "reference": record.Reference,
		})
	})
	return record, err
}

func (s *Store) TombstoneFor(digest string) (Tombstone, error) {
	var result Tombstone
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketTombstones).Get([]byte(digest))
		if value == nil {
			return ErrNotFound
		}
		return json.Unmarshal(value, &result)
	})
	return result, err
}

func (s *Store) Audit(limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	var result []AuditEvent
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketAudit).Cursor()
		for key, value := cursor.Last(); key != nil && len(result) < limit; key, value = cursor.Prev() {
			var event AuditEvent
			if err := json.Unmarshal(value, &event); err != nil {
				return err
			}
			result = append(result, event)
		}
		return nil
	})
	return result, err
}

func (s *Store) appendAudit(tx *bolt.Tx, action, actor, object string, details map[string]any) error {
	bucket := tx.Bucket(bucketAudit)
	sequence, err := bucket.NextSequence()
	if err != nil {
		return err
	}
	event := AuditEvent{
		Index: sequence - 1, Time: s.now().UTC().Truncate(time.Second).Format(time.RFC3339),
		Action: action, Actor: actor, Object: object, Details: details,
	}
	if sequence > 1 {
		var previous AuditEvent
		if err := json.Unmarshal(bucket.Get(uint64Key(sequence-2)), &previous); err != nil {
			return err
		}
		event.Previous = previous.Hash
	}
	unsigned, _ := json.Marshal(event)
	hash, err := mgs.Digest(unsigned)
	if err != nil {
		return err
	}
	event.Hash = hash
	encoded, _ := json.Marshal(event)
	return bucket.Put(uint64Key(event.Index), encoded)
}

func (s *Store) amendmentBytes(gene string) ([][]byte, error) {
	var result [][]byte
	err := s.db.View(func(tx *bolt.Tx) error {
		cursor := tx.Bucket(bucketAmendments).Cursor()
		prefix := []byte(gene + "\x00")
		for key, digest := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, digest = cursor.Next() {
			raw := tx.Bucket(bucketObjects).Get(digest)
			if raw == nil {
				return errors.New("registry amendment index references missing object")
			}
			result = append(result, append([]byte(nil), raw...))
		}
		return nil
	})
	return result, err
}

func (s *Store) namespaceAllowed(namespace, controller string) error {
	if strings.HasPrefix(namespace, "did:") {
		if namespace == controller {
			return nil
		}
		return errors.New("DID namespace must equal the genesis controller DID")
	}
	if !s.requireNamespaceGrant {
		return nil
	}
	namespace = strings.ToLower(strings.TrimSuffix(namespace, "."))
	var grant NamespaceGrant
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketGrants).Get(joinKey(namespace, controller))
		if value == nil {
			return ErrNamespaceRequired
		}
		return json.Unmarshal(value, &grant)
	})
	if err != nil {
		return err
	}
	expires, err := time.Parse(time.RFC3339, grant.ExpiresAt)
	if err != nil || !s.now().Before(expires) {
		return ErrNamespaceRequired
	}
	return nil
}

func (s *Store) Backup(output string) error {
	if output == "" {
		return errors.New("backup output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o700); err != nil {
		return err
	}
	handle, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	ok := false
	defer func() {
		if !ok {
			handle.Close()
			_ = os.Remove(output)
		}
	}()
	err = s.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(handle)
		return err
	})
	if err != nil {
		return err
	}
	if err := handle.Sync(); err != nil {
		return err
	}
	if err := handle.Close(); err != nil {
		return err
	}
	ok = true
	return s.db.Update(func(tx *bolt.Tx) error {
		return s.appendAudit(tx, "registry.backup.created", "operator", "", map[string]any{
			"file": filepath.Base(output),
		})
	})
}

func (s *Store) Stats() (objects, genes, logSize uint64, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		objects = uint64(tx.Bucket(bucketObjects).Stats().KeyN)
		genes = uint64(tx.Bucket(bucketGenes).Stats().KeyN)
		logSize = tx.Bucket(bucketLog).Sequence()
		return nil
	})
	return
}

func (s *Store) LogLeaves() ([]string, error) {
	var leaves []string
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketLog).ForEach(func(_, value []byte) error {
			leaves = append(leaves, string(value))
			return nil
		})
	})
	return leaves, err
}

func (s *Store) Verify() error {
	return s.db.View(func(tx *bolt.Tx) error {
		for _, name := range allBuckets {
			if tx.Bucket(name) == nil {
				return fmt.Errorf("missing bucket %q", name)
			}
		}
		var previous string
		err := tx.Bucket(bucketAudit).ForEach(func(_, value []byte) error {
			var event AuditEvent
			if err := json.Unmarshal(value, &event); err != nil {
				return err
			}
			hash := event.Hash
			event.Hash = ""
			if event.Previous != previous {
				return fmt.Errorf("audit chain mismatch at %d", event.Index)
			}
			unsigned, _ := json.Marshal(event)
			actual, err := mgs.Digest(unsigned)
			if err != nil || actual != hash {
				return fmt.Errorf("audit hash mismatch at %d", event.Index)
			}
			previous = hash
			return nil
		})
		if err != nil {
			return err
		}
		return tx.Bucket(bucketObjectMeta).ForEach(func(key, value []byte) error {
			raw := tx.Bucket(bucketObjects).Get(key)
			if raw == nil {
				return fmt.Errorf("metadata %s has no object", key)
			}
			meta, err := decodeMetadata(value)
			if err != nil {
				return err
			}
			if err := mgs.DigestMatches(raw, meta.Digest); err != nil {
				return err
			}
			return nil
		})
	})
}

func collectMetadata(tx *bolt.Tx, bucketName, prefix []byte, target *[]RecordMetadata) error {
	cursor := tx.Bucket(bucketName).Cursor()
	for key, digest := cursor.Seek(prefix); key != nil && bytes.HasPrefix(key, prefix); key, digest = cursor.Next() {
		meta, err := decodeMetadata(tx.Bucket(bucketObjectMeta).Get(digest))
		if err != nil {
			return err
		}
		*target = append(*target, meta)
	}
	return nil
}

func decodeMetadata(raw []byte) (RecordMetadata, error) {
	if raw == nil {
		return RecordMetadata{}, ErrNotFound
	}
	var meta RecordMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return RecordMetadata{}, err
	}
	return meta, nil
}

func uint64Key(value uint64) []byte {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, value)
	return key
}

func sequenceKey(gene string, sequence uint64) []byte {
	return append(append([]byte(gene), 0), uint64Key(sequence)...)
}

func identityKey(namespace, name, version string) []byte {
	return []byte(strings.Join([]string{strings.ToLower(namespace), strings.ToLower(name), version}, "\x00"))
}

func joinKey(parts ...string) []byte {
	return []byte(strings.Join(parts, "\x00"))
}

func Copy(dst io.Writer, src []byte) error {
	_, err := dst.Write(src)
	return err
}
