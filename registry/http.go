package registry

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Sidiora-Labs/machine-genome/mgs"
)

const recordMediaType = "application/mgs+json"

type HTTPConfig struct {
	Store            *Store
	AdminToken       string
	CheckpointKey    ed25519.PrivateKey
	CheckpointDID    string
	CheckpointMethod string
	Resolver         TXTResolver
	Logger           *slog.Logger
	PublicBaseURL    string
	RatePerMinute    int
	BackupDir        string
}

type HTTPService struct {
	store            *Store
	adminHash        [32]byte
	checkpointKey    ed25519.PrivateKey
	checkpointDID    string
	checkpointMethod string
	resolver         TXTResolver
	logger           *slog.Logger
	publicBaseURL    string
	backupDir        string
	rate             *ipLimiter
	requests         atomic.Uint64
	rejected         atomic.Uint64
}

type Checkpoint struct {
	Format             string `json:"format"`
	TreeSize           uint64 `json:"treeSize"`
	RootHash           string `json:"rootHash"`
	Timestamp          string `json:"timestamp"`
	RegistryDID        string `json:"registryDid"`
	VerificationMethod string `json:"verificationMethod"`
	SignatureAlgorithm string `json:"signatureAlgorithm"`
	Signature          string `json:"signature"`
}

func NewHTTPService(config HTTPConfig) (*HTTPService, error) {
	if config.Store == nil {
		return nil, errors.New("registry store is required")
	}
	if len(config.AdminToken) < 32 {
		return nil, errors.New("admin token must contain at least 32 bytes")
	}
	if len(config.CheckpointKey) != ed25519.PrivateKeySize {
		return nil, errors.New("checkpoint Ed25519 key is required")
	}
	if config.CheckpointDID == "" || config.CheckpointMethod == "" {
		return nil, errors.New("checkpoint DID and verification method are required")
	}
	derivedDID, derivedMethod, err := mgs.DIDKeyFromPublicKey(config.CheckpointKey.Public().(ed25519.PublicKey))
	if err != nil || derivedDID != config.CheckpointDID || derivedMethod != config.CheckpointMethod {
		return nil, errors.New("checkpoint key does not match its configured DID and verification method")
	}
	if config.PublicBaseURL != "" {
		base, err := url.Parse(config.PublicBaseURL)
		if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" ||
			base.User != nil || base.RawQuery != "" || base.Fragment != "" {
			return nil, errors.New("public base URL must be an absolute HTTP(S) origin without credentials, query, or fragment")
		}
	}
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	rate := config.RatePerMinute
	if rate <= 0 {
		rate = 120
	}
	return &HTTPService{
		store: config.Store, adminHash: sha256.Sum256([]byte(config.AdminToken)),
		checkpointKey: config.CheckpointKey, checkpointDID: config.CheckpointDID,
		checkpointMethod: config.CheckpointMethod, resolver: config.Resolver,
		logger: logger, publicBaseURL: strings.TrimSuffix(config.PublicBaseURL, "/"),
		backupDir: config.BackupDir, rate: newIPLimiter(rate),
	}, nil
}

func (s *HTTPService) Handler() http.Handler {
	return s.middleware(http.HandlerFunc(s.route))
}

func (s *HTTPService) route(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz/live":
		writeJSON(w, http.StatusOK, map[string]any{"status": "live"})
	case r.Method == http.MethodGet && r.URL.Path == "/healthz/ready":
		objects, genes, logSize, err := s.store.Stats()
		if err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "not_ready", "Registry storage is unavailable.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ready", "objects": objects, "genes": genes, "treeSize": logSize})
	case r.Method == http.MethodGet && r.URL.Path == "/metrics":
		s.metrics(w)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/records":
		s.submit(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/records/"):
		s.record(w, r, strings.TrimPrefix(r.URL.Path, "/v1/records/"))
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/genes/"):
		s.gene(w, r, strings.TrimPrefix(r.URL.Path, "/v1/genes/"))
	case r.Method == http.MethodGet && r.URL.Path == "/v1/search":
		s.search(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/namespace-challenges":
		s.createChallenge(w, r)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/namespace-challenges/") && strings.HasSuffix(r.URL.Path, "/verify"):
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/namespace-challenges/"), "/verify")
		s.verifyChallenge(w, r, id)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/tombstones/"):
		s.tombstone(w, r, strings.TrimPrefix(r.URL.Path, "/v1/admin/tombstones/"))
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/audit":
		s.audit(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/backups":
		s.backup(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/log/checkpoint":
		s.checkpoint(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/log/inclusion/"):
		s.inclusion(w, r, strings.TrimPrefix(r.URL.Path, "/v1/log/inclusion/"))
	case r.Method == http.MethodGet && r.URL.Path == "/v1/log/consistency":
		s.consistency(w, r)
	case r.Method == http.MethodGet && (r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/genes/")):
		s.explorer(w, r)
	default:
		writeProblem(w, http.StatusNotFound, "not_found", "The requested registry resource does not exist.")
	}
}

func (s *HTTPService) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.requests.Add(1)
		requestID := requestID()
		w.Header().Set("X-Request-ID", requestID)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		if !s.rate.allow(clientIP(r), time.Now()) {
			s.rejected.Add(1)
			w.Header().Set("Retry-After", "60")
			writeProblem(w, http.StatusTooManyRequests, "rate_limited", "Request rate limit exceeded.")
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("registry request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds())
	})
}

func (s *HTTPService) submit(w http.ResponseWriter, r *http.Request) {
	if media := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0])); media != recordMediaType && media != "application/json" {
		writeProblem(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Use application/mgs+json.")
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		writeProblem(w, http.StatusRequestEntityTooLarge, "record_too_large", "Records are limited to 1 MiB.")
		return
	}
	result, err := s.store.Submit(r.Context(), body)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	if result.Conflict {
		writeJSON(w, http.StatusConflict, result)
		return
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	w.Header().Set("Location", s.publicBaseURL+"/v1/records/"+result.Metadata.Digest)
	writeJSON(w, status, result)
}

func (s *HTTPService) record(w http.ResponseWriter, _ *http.Request, digest string) {
	if err := mgs.ValidateDigest(digest); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_digest", "The record identifier is not an MGS SHA-256 digest.")
		return
	}
	raw, err := s.store.Get(digest, false)
	if errors.Is(err, ErrGone) {
		tombstone, lookupErr := s.store.TombstoneFor(digest)
		if lookupErr != nil {
			s.writeStoreError(w, lookupErr)
			return
		}
		writeJSON(w, http.StatusGone, tombstone)
		return
	}
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", recordMediaType)
	w.Header().Set("ETag", `"`+digest+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}

func (s *HTTPService) gene(w http.ResponseWriter, _ *http.Request, gene string) {
	if strings.Contains(gene, "/") {
		writeProblem(w, http.StatusNotFound, "not_found", "The requested gene view does not exist.")
		return
	}
	view, err := s.store.Gene(gene)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *HTTPService) search(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := s.store.Search(r.URL.Query().Get("q"), limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *HTTPService) createChallenge(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Namespace     string `json:"namespace"`
		ControllerDID string `json:"controllerDid"`
	}
	if err := decodeRequest(w, r, &input, 16<<10); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	challenge, _, err := s.store.CreateNamespaceChallenge(input.Namespace, input.ControllerDID)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_challenge", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, challenge)
}

func (s *HTTPService) verifyChallenge(w http.ResponseWriter, r *http.Request, id string) {
	var input struct {
		Token string `json:"token"`
	}
	if err := decodeRequest(w, r, &input, 16<<10); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	grant, err := s.store.VerifyNamespaceChallenge(r.Context(), id, input.Token, s.resolver)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grant)
}

func (s *HTTPService) tombstone(w http.ResponseWriter, r *http.Request, digest string) {
	if !s.authorized(r) {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "A valid registry administrator token is required.")
		return
	}
	var input struct {
		Reason    string `json:"reason"`
		Reference string `json:"reference"`
	}
	if err := decodeRequest(w, r, &input, 16<<10); err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	result, err := s.store.Tombstone(digest, input.Reason, input.Reference, "administrator")
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (s *HTTPService) audit(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "A valid registry administrator token is required.")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := s.store.Audit(limit)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func (s *HTTPService) backup(w http.ResponseWriter, r *http.Request) {
	if !s.authorized(r) {
		writeProblem(w, http.StatusUnauthorized, "unauthorized", "A valid registry administrator token is required.")
		return
	}
	if s.backupDir == "" {
		writeProblem(w, http.StatusServiceUnavailable, "backup_unavailable", "Online backups are not configured.")
		return
	}
	name := "registry-" + time.Now().UTC().Format("20060102T150405.000000000Z") + ".backup.db"
	if err := s.store.Backup(filepath.Join(s.backupDir, name)); err != nil {
		s.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"file": name, "created": true})
}

func (s *HTTPService) checkpoint(w http.ResponseWriter, _ *http.Request) {
	leaves, err := s.store.LogLeaves()
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	checkpoint := Checkpoint{
		Format: "mgs-checkpoint-v1", TreeSize: uint64(len(leaves)), RootHash: TreeRoot(leaves),
		Timestamp:   time.Now().UTC().Truncate(time.Second).Format(time.RFC3339),
		RegistryDID: s.checkpointDID, VerificationMethod: s.checkpointMethod,
		SignatureAlgorithm: "Ed25519-JCS",
	}
	unsigned, _ := json.Marshal(checkpoint)
	canonical, err := mgs.Canonicalize(unsigned)
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	checkpoint.Signature = base64.RawURLEncoding.EncodeToString(ed25519.Sign(s.checkpointKey, canonical))
	writeJSON(w, http.StatusOK, checkpoint)
}

func VerifyCheckpoint(checkpoint Checkpoint, publicKey ed25519.PublicKey) error {
	if checkpoint.Format != "mgs-checkpoint-v1" || checkpoint.SignatureAlgorithm != "Ed25519-JCS" {
		return errors.New("unsupported checkpoint format or signature algorithm")
	}
	derivedDID, derivedMethod, err := mgs.DIDKeyFromPublicKey(publicKey)
	if err != nil || checkpoint.RegistryDID != derivedDID || checkpoint.VerificationMethod != derivedMethod {
		return errors.New("checkpoint signer metadata does not match the public key")
	}
	root, err := base64.RawURLEncoding.DecodeString(checkpoint.RootHash)
	if err != nil || len(root) != sha256.Size {
		return errors.New("invalid checkpoint root hash")
	}
	parsedTime, err := time.Parse(time.RFC3339, checkpoint.Timestamp)
	if err != nil || parsedTime.UTC().Truncate(time.Second).Format(time.RFC3339) != checkpoint.Timestamp {
		return errors.New("invalid checkpoint timestamp")
	}
	signature, err := base64.RawURLEncoding.DecodeString(checkpoint.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid checkpoint signature encoding")
	}
	checkpoint.Signature = ""
	unsigned, _ := json.Marshal(checkpoint)
	canonical, err := mgs.Canonicalize(unsigned)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, canonical, signature) {
		return errors.New("checkpoint signature mismatch")
	}
	return nil
}

func (s *HTTPService) inclusion(w http.ResponseWriter, r *http.Request, indexText string) {
	index, err := strconv.ParseUint(indexText, 10, 64)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_index", "Log index must be an unsigned integer.")
		return
	}
	size, _ := strconv.ParseUint(r.URL.Query().Get("treeSize"), 10, 64)
	proof, err := s.store.InclusionProof(index, size)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_proof_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proof)
}

func (s *HTTPService) consistency(w http.ResponseWriter, r *http.Request) {
	first, firstErr := strconv.ParseUint(r.URL.Query().Get("firstSize"), 10, 64)
	second, secondErr := strconv.ParseUint(r.URL.Query().Get("secondSize"), 10, 64)
	if firstErr != nil || secondErr != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_tree_size", "firstSize and secondSize must be unsigned integers.")
		return
	}
	proof, err := s.store.ConsistencyProof(first, second)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "invalid_proof_request", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, proof)
}

func (s *HTTPService) explorer(w http.ResponseWriter, r *http.Request) {
	data := explorerData{BaseURL: s.publicBaseURL}
	objects, genes, treeSize, err := s.store.Stats()
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	leaves, err := s.store.LogLeaves()
	if err != nil {
		s.writeStoreError(w, err)
		return
	}
	data.Stats = explorerStats{
		Objects: objects, Genes: genes, TreeSize: treeSize, RootHash: TreeRoot(leaves),
	}
	if r.URL.Path == "/" {
		items, err := s.store.ListGenes()
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		data = prepareExplorerDirectory(data, items, r.URL.Query())
	} else {
		gene := strings.TrimPrefix(r.URL.Path, "/genes/")
		view, err := s.store.Gene(gene)
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		raw, err := s.store.Get(gene, true)
		if err != nil {
			s.writeStoreError(w, err)
			return
		}
		var genesis mgs.GenesisRecord
		if err := json.Unmarshal(raw, &genesis); err != nil {
			s.writeStoreError(w, err)
			return
		}
		detail := explorerGene{Registry: view, Record: genesis}
		for _, edge := range genesis.Parentage {
			parent := explorerParent{
				Gene: edge.ParentGene, EdgeType: edge.EdgeType, Authorized: edge.Authorized,
				Unresolved: edge.Unresolved, ExternalRef: edge.ExternalRef,
			}
			if edge.ParentGene != "" {
				metadata, metadataErr := s.store.Metadata(edge.ParentGene)
				if metadataErr != nil && !errors.Is(metadataErr, ErrNotFound) {
					s.writeStoreError(w, metadataErr)
					return
				}
				if metadataErr == nil {
					parent.Name = metadata.IdentityName
					parent.Namespace = metadata.Namespace
					parent.SubjectType = metadata.SubjectType
				}
			}
			detail.Parents = append(detail.Parents, parent)
		}
		data.View = &detail
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := explorerTemplate.Execute(w, data); err != nil {
		s.logger.Error("render explorer", "error", err)
	}
}

func (s *HTTPService) metrics(w http.ResponseWriter) {
	objects, genes, size, err := s.store.Stats()
	if err != nil {
		writeProblem(w, http.StatusServiceUnavailable, "not_ready", "Registry storage is unavailable.")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "mgs_registry_requests_total %d\nmgs_registry_rejected_total %d\nmgs_registry_objects %d\nmgs_registry_genes %d\nmgs_registry_transparency_tree_size %d\n",
		s.requests.Load(), s.rejected.Load(), objects, genes, size)
}

func (s *HTTPService) authorized(r *http.Request) bool {
	const prefix = "Bearer "
	value := r.Header.Get("Authorization")
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	hash := sha256.Sum256([]byte(strings.TrimPrefix(value, prefix)))
	return subtle.ConstantTimeCompare(hash[:], s.adminHash[:]) == 1
}

func (s *HTTPService) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "not_found", "The requested registry object does not exist.")
	case errors.Is(err, ErrNamespaceRequired):
		writeProblem(w, http.StatusForbidden, "namespace_control_required", "Complete a namespace challenge for this controller before submitting the genesis.")
	case errors.Is(err, ErrConflict):
		writeProblem(w, http.StatusConflict, "conflict", err.Error())
	default:
		writeProblem(w, http.StatusUnprocessableEntity, "invalid_record", err.Error())
	}
}

func decodeRequest(w http.ResponseWriter, r *http.Request, target any, limit int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request must contain exactly one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	writeJSON(w, status, map[string]any{
		"type": "urn:mgs:problem:" + code, "title": http.StatusText(status),
		"status": status, "code": code, "detail": detail,
	})
}

func requestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return base64.RawURLEncoding.EncodeToString(value)
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

type ipLimiter struct {
	mu      sync.Mutex
	limit   int
	entries map[string]*rateEntry
	lastGC  time.Time
}

type rateEntry struct {
	window time.Time
	count  int
}

func newIPLimiter(limit int) *ipLimiter {
	return &ipLimiter{limit: limit, entries: make(map[string]*rateEntry)}
}

func (l *ipLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.lastGC.IsZero() || now.Sub(l.lastGC) >= time.Minute {
		for address, candidate := range l.entries {
			if now.Sub(candidate.window) >= time.Minute {
				delete(l.entries, address)
			}
		}
		l.lastGC = now
	}
	entry := l.entries[ip]
	if entry == nil || now.Sub(entry.window) >= time.Minute {
		if len(l.entries) >= 10_000 {
			return false
		}
		l.entries[ip] = &rateEntry{window: now, count: 1}
		return true
	}
	entry.count++
	return entry.count <= l.limit
}

type explorerData struct {
	BaseURL      string
	Query        string
	SubjectType  string
	Provider     string
	Capability   string
	Providers    []string
	Capabilities []string
	Items        []RecordMetadata
	Total        int
	Page         int
	PageCount    int
	PageStart    int
	PageEnd      int
	HasPrevious  bool
	HasNext      bool
	PreviousURL  string
	NextURL      string
	Stats        explorerStats
	View         *explorerGene
}

type explorerStats struct {
	Objects  uint64
	Genes    uint64
	TreeSize uint64
	RootHash string
}

type explorerGene struct {
	Registry GeneView
	Record   mgs.GenesisRecord
	Parents  []explorerParent
}

type explorerParent struct {
	Gene        string
	Name        string
	Namespace   string
	SubjectType string
	EdgeType    string
	Authorized  bool
	Unresolved  bool
	ExternalRef string
}

const explorerPageSize = 24

func prepareExplorerDirectory(data explorerData, items []RecordMetadata, values url.Values) explorerData {
	data.Query = strings.TrimSpace(values.Get("q"))
	data.SubjectType = strings.ToLower(strings.TrimSpace(values.Get("type")))
	data.Provider = strings.TrimSpace(values.Get("provider"))
	data.Capability = strings.TrimSpace(values.Get("capability"))

	providerSet := make(map[string]bool)
	capabilitySet := make(map[string]bool)
	filtered := make([]RecordMetadata, 0, len(items))
	for _, item := range items {
		if item.ModelProvider != "" {
			providerSet[item.ModelProvider] = true
		}
		for _, capability := range item.Capabilities {
			capabilitySet[capability] = true
		}
		if explorerRecordMatches(item, data.Query, data.SubjectType, data.Provider, data.Capability) {
			filtered = append(filtered, item)
		}
	}
	for provider := range providerSet {
		data.Providers = append(data.Providers, provider)
	}
	for capability := range capabilitySet {
		data.Capabilities = append(data.Capabilities, capability)
	}
	sort.Strings(data.Providers)
	sort.Strings(data.Capabilities)
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].LogIndex > filtered[j].LogIndex })

	data.Total = len(filtered)
	data.PageCount = (data.Total + explorerPageSize - 1) / explorerPageSize
	if data.PageCount == 0 {
		data.PageCount = 1
	}
	data.Page, _ = strconv.Atoi(values.Get("page"))
	if data.Page < 1 {
		data.Page = 1
	}
	if data.Page > data.PageCount {
		data.Page = data.PageCount
	}
	start := (data.Page - 1) * explorerPageSize
	end := start + explorerPageSize
	if end > data.Total {
		end = data.Total
	}
	if start < data.Total {
		data.Items = filtered[start:end]
		data.PageStart = start + 1
		data.PageEnd = end
	} else {
		data.Items = []RecordMetadata{}
	}
	data.HasPrevious = data.Page > 1
	data.HasNext = data.Page < data.PageCount
	if data.HasPrevious {
		data.PreviousURL = explorerPageURL(values, data.Page-1)
	}
	if data.HasNext {
		data.NextURL = explorerPageURL(values, data.Page+1)
	}
	return data
}

func explorerRecordMatches(item RecordMetadata, query, subjectType, provider, capability string) bool {
	if subjectType != "" && subjectType != "all" {
		if subjectType == "agent" {
			if item.SubjectType != "agent" && item.SubjectType != "agent-class" && item.SubjectType != "agent-instance" {
				return false
			}
		} else if subjectType == "foundation" {
			if !item.Foundation {
				return false
			}
		} else if !strings.EqualFold(item.SubjectType, subjectType) {
			return false
		}
	}
	if provider != "" && !strings.EqualFold(item.ModelProvider, provider) {
		return false
	}
	if capability != "" && !containsFold(item.Capabilities, capability) {
		return false
	}
	if query == "" {
		return true
	}
	classification := ""
	if item.Foundation {
		classification = "foundation"
	}
	haystack := strings.ToLower(strings.Join([]string{
		item.Gene, item.IdentityName, item.AgentID, item.AgentName, item.GivenName,
		item.FamilyName, item.Namespace, item.SubjectType, item.ModelProvider,
		item.ModelID, item.ModelFamily, classification, strings.Join(item.Capabilities, " "),
	}, " "))
	return strings.Contains(haystack, strings.ToLower(query))
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(value, wanted) {
			return true
		}
	}
	return false
}

func explorerPageURL(values url.Values, page int) string {
	copyValues := make(url.Values, len(values))
	for key, value := range values {
		copyValues[key] = append([]string(nil), value...)
	}
	copyValues.Set("page", strconv.Itoa(page))
	return "/?" + copyValues.Encode() + "#identities"
}

var explorerTemplate = template.Must(template.New("explorer").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="A public registry for verifiable machine identity and lineage.">
<title>{{if .View}}{{.View.Record.Identity.Name}} · {{end}}Machine Genome Registry</title><style>
:root{color-scheme:light;--paper:#f3f1ea;--paper-2:#e9e7de;--panel:#fbfaf5;--ink:#15211f;--muted:#65706c;--line:#d5d5cb;--accent:#0d6c5d;--accent-2:#bce8d9;--night:#102824;--warn:#a75a16;--warn-bg:#fff1df;--shadow:0 18px 50px rgba(22,39,35,.08)}
*{box-sizing:border-box}html{scroll-behavior:smooth}body{margin:0;background:var(--paper);color:var(--ink);font:16px/1.55 Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;-webkit-font-smoothing:antialiased}
a{color:inherit;text-decoration-color:#8b9c97;text-underline-offset:.18em}a:hover{color:var(--accent)}
.shell{width:min(1180px,calc(100% - 40px));margin:auto}.site-head{border-bottom:1px solid rgba(21,33,31,.12);background:rgba(243,241,234,.94)}
.nav{min-height:76px;display:flex;align-items:center;justify-content:space-between;gap:24px}.brand{display:flex;align-items:center;gap:11px;text-decoration:none;font-weight:760;letter-spacing:-.02em}
.mark{width:34px;height:34px;display:grid;place-items:center;border-radius:50%;background:var(--night);color:#d6fff0}.mark svg{width:22px;height:22px}
.nav-links{display:flex;align-items:center;gap:22px;font-size:.9rem;color:var(--muted)}.live{display:inline-flex;align-items:center;gap:8px;font-weight:680;color:var(--accent)}.live:before{content:"";width:7px;height:7px;border-radius:50%;background:#24a983;box-shadow:0 0 0 4px rgba(36,169,131,.12)}
main{min-height:70vh}.hero{padding:82px 0 58px;display:grid;grid-template-columns:minmax(0,1.5fr) minmax(280px,.72fr);gap:60px;align-items:end}
.eyebrow,.label{font-size:.73rem;font-weight:780;letter-spacing:.13em;text-transform:uppercase;color:var(--accent)}h1,h2,h3,p{margin-top:0}
h1{font-family:Georgia,"Times New Roman",serif;font-size:clamp(3rem,7.2vw,6.3rem);line-height:.92;letter-spacing:-.055em;font-weight:500;margin:.28em 0 .28em;max-width:880px}
.lede{font-size:clamp(1.05rem,1.8vw,1.3rem);color:var(--muted);max-width:690px}.hero-note{border-left:1px solid var(--line);padding-left:28px;color:var(--muted);font-size:.92rem}.hero-note strong{display:block;color:var(--ink);font-size:1rem;margin-bottom:8px}
.directory-tools{display:grid;grid-template-columns:minmax(230px,1.5fr) repeat(3,minmax(145px,.65fr)) auto;gap:10px;padding:12px;background:var(--panel);border:1px solid var(--line);border-radius:16px;box-shadow:var(--shadow);margin-bottom:64px}
.field{display:grid;gap:3px;padding:2px 10px;border-right:1px solid var(--line);min-width:0}.field label{font-size:.67rem;font-weight:780;letter-spacing:.09em;text-transform:uppercase;color:var(--muted)}.field input,.field select{width:100%;min-width:0;background:transparent;color:var(--ink);border:0;padding:5px 0;font:inherit;outline:none}.field input::placeholder{color:#89928e}.directory-actions{display:flex;align-items:center;gap:9px}.reset{font-size:.8rem;color:var(--muted)}
.button{display:inline-flex;align-items:center;justify-content:center;border:0;border-radius:10px;padding:12px 20px;background:var(--night);color:white;font:inherit;font-weight:730;cursor:pointer;text-decoration:none}.button:hover{background:var(--accent);color:white}
.registry-bar{display:grid;grid-template-columns:repeat(4,1fr);background:var(--night);color:white;border-radius:18px;overflow:hidden;margin-bottom:72px}
.stat{padding:22px 24px;border-right:1px solid rgba(255,255,255,.13)}.stat:last-child{border:0}.stat strong{display:block;font-family:Georgia,serif;font-size:1.65rem;font-weight:500}.stat span{color:#a9c0ba;font-size:.76rem;text-transform:uppercase;letter-spacing:.1em}.stat.root strong{font:600 .78rem/1.5 ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}
.section-head{display:flex;align-items:end;justify-content:space-between;gap:20px;border-bottom:1px solid var(--line);padding-bottom:18px;margin-bottom:0}.section-head h2{font-family:Georgia,serif;font-size:clamp(2rem,4vw,3.2rem);font-weight:500;letter-spacing:-.035em;margin:0}.section-head p{color:var(--muted);margin:0}
.record-list{margin-bottom:28px}.record{display:grid;grid-template-columns:54px minmax(210px,.9fr) minmax(210px,1fr) minmax(170px,.8fr) 150px;gap:24px;align-items:center;padding:23px 8px;border-bottom:1px solid var(--line)}.record:hover{background:rgba(255,255,255,.38)}.record-index{font:600 .77rem ui-monospace,monospace;color:var(--muted)}
.record h3{font-size:1.05rem;margin:0}.record h3 a{text-decoration:none}.record-coordinate{display:grid;gap:3px}.type{display:inline-flex;padding:5px 9px;border:1px solid var(--line);border-radius:999px;color:var(--muted);font-size:.72rem;text-transform:uppercase;letter-spacing:.08em;width:max-content}.digest{font:600 .76rem/1.55 ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere;color:var(--muted)}.record-time{font-size:.8rem;color:var(--muted);text-align:right}.capability-links{display:flex;gap:5px;flex-wrap:wrap}.chip{display:inline-flex;padding:3px 7px;border-radius:999px;background:var(--paper-2);color:var(--muted);font-size:.68rem;text-decoration:none}.chip:hover{background:var(--accent-2);color:var(--accent)}
.empty{padding:38px 8px;color:var(--muted);border-bottom:1px solid var(--line)}
.pagination{display:flex;align-items:center;justify-content:space-between;gap:20px;margin:0 0 80px}.page-status{text-align:center;color:var(--muted);font-size:.84rem}.page-status strong{display:block;color:var(--ink);font-size:.92rem}.page-link{min-width:110px;padding:10px 14px;border:1px solid var(--line);border-radius:10px;text-decoration:none;text-align:center;font-weight:700;font-size:.84rem}.page-link:hover{border-color:var(--accent)}.page-spacer{min-width:110px}
.detail-hero{padding:56px 0 44px}.breadcrumb{font-size:.84rem;color:var(--muted);margin-bottom:44px;overflow-wrap:anywhere}.breadcrumb a{text-decoration:none}.detail-title{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:30px;align-items:end}.detail-title h1{font-size:clamp(3.1rem,7vw,5.8rem);margin:.15em 0}.description{font-size:1.1rem;color:var(--muted);max-width:720px}.verified{display:inline-flex;align-items:center;gap:9px;padding:10px 14px;background:var(--accent-2);color:#075346;border-radius:999px;font-size:.82rem;font-weight:760;white-space:nowrap}.verified:before{content:"✓";display:grid;place-items:center;width:19px;height:19px;border-radius:50%;background:var(--accent);color:white}
.identity{display:grid;grid-template-columns:1.2fr .8fr;background:var(--panel);border:1px solid var(--line);border-radius:20px;overflow:hidden;box-shadow:var(--shadow);margin-bottom:58px}.identity-main{padding:32px}.identity-side{padding:32px;background:var(--paper-2);border-left:1px solid var(--line)}
.gene{font:600 .86rem/1.7 ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere;margin:12px 0 28px}.facts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:22px}.fact{min-width:0}.facts>.fact:first-child{grid-column:1/-1}.fact strong{display:block;max-width:100%;font-size:.93rem;margin-top:4px;overflow-wrap:anywhere;word-break:break-word}.fact .digest{font-size:.7rem}
.identity-genome{margin:-24px 0 58px}.genome-head{display:flex;align-items:end;justify-content:space-between;gap:20px;margin-bottom:18px}.genome-head h2{font-family:Georgia,serif;font-size:2rem;font-weight:500;letter-spacing:-.03em;margin:0}.genome-head p{color:var(--muted);font-size:.85rem;margin:0}.genome-track{display:grid;grid-template-columns:repeat(4,1fr);border:1px solid var(--line);border-radius:16px;overflow:hidden;background:var(--panel)}.genome-node{position:relative;padding:22px;border-right:1px solid var(--line);min-height:142px}.genome-node:last-child{border:0}.genome-node:not(:last-child):after{content:"→";position:absolute;right:-12px;top:24px;z-index:1;display:grid;place-items:center;width:24px;height:24px;border-radius:50%;background:var(--night);color:white;font-size:.75rem}.genome-node strong{display:block;margin:7px 0 5px;overflow-wrap:anywhere}.genome-node p{color:var(--muted);font-size:.79rem;margin:0}.genome-number{font-family:Georgia,serif;font-size:1.8rem;font-weight:500}
.layout{display:grid;grid-template-columns:minmax(0,1.55fr) minmax(280px,.75fr);gap:48px;margin-bottom:76px}.block{margin-bottom:52px}.block-title{display:flex;align-items:end;justify-content:space-between;border-bottom:1px solid var(--line);padding-bottom:14px;margin-bottom:18px}.block-title h2{font-family:Georgia,serif;font-size:2rem;font-weight:500;letter-spacing:-.03em;margin:0}.count{color:var(--muted);font-size:.84rem}
.lineage{display:grid;gap:12px}.parent{position:relative;display:grid;grid-template-columns:44px minmax(0,1fr) auto;gap:14px;align-items:center;background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:17px;text-decoration:none}.parent:hover{border-color:#9aaba5;color:inherit}.parent-icon{width:42px;height:42px;border-radius:12px;background:var(--paper-2);display:grid;place-items:center;font-family:Georgia,serif;font-size:1.25rem}.parent h3{margin:0 0 2px;font-size:1rem}.relation{color:var(--muted);font-size:.78rem}.auth{font-size:.7rem;font-weight:780;text-transform:uppercase;letter-spacing:.07em;color:var(--accent)}.auth.observed{color:var(--warn)}
.artifact{padding:19px 0;border-bottom:1px solid var(--line)}.artifact:first-of-type{padding-top:2px}.artifact-top{display:flex;justify-content:space-between;gap:20px}.artifact h3{font-size:.96rem;margin:0 0 5px}.artifact p{color:var(--muted);font-size:.84rem;margin:6px 0}.artifact .digest{font-size:.68rem}.present{color:var(--accent);font-size:.72rem;font-weight:760}.side-card{border:1px solid var(--line);border-radius:16px;padding:23px;margin-bottom:16px;background:rgba(251,250,245,.6)}.side-card h3{font-size:1rem;margin-bottom:18px}.metric{display:flex;justify-content:space-between;padding:11px 0;border-top:1px solid var(--line);font-size:.87rem}.metric span{color:var(--muted)}.api-links{display:grid;gap:9px}.api-links a{display:flex;justify-content:space-between;gap:14px;padding:11px 0;border-top:1px solid var(--line);font-size:.86rem;text-decoration:none}.api-links a:after{content:"↗";color:var(--muted)}
.notice{background:var(--warn-bg);border:1px solid #f0d0a7;border-radius:12px;padding:16px;color:#744218;font-size:.88rem}.empty-lineage{padding:25px 0;color:var(--muted)}
footer{background:var(--night);color:white;padding:42px 0}.foot{display:flex;justify-content:space-between;gap:40px;align-items:end}.foot p{color:#a9c0ba;max-width:560px;margin:7px 0 0}.foot-links{display:flex;gap:20px;font-size:.84rem;color:#cde3dd}
@media(max-width:980px){.directory-tools{grid-template-columns:1fr 1fr}.field{border-right:0;border-bottom:1px solid var(--line)}.directory-actions{justify-content:flex-end}.record{grid-template-columns:42px minmax(200px,1fr) minmax(170px,.8fr) 130px}.record-capabilities{display:none}.genome-track{grid-template-columns:1fr 1fr}.genome-node:nth-child(2){border-right:0}.genome-node:nth-child(-n+2){border-bottom:1px solid var(--line)}}
@media(max-width:800px){.shell{width:min(100% - 28px,1180px)}.nav-links a:not(:first-child){display:none}.hero{grid-template-columns:1fr;padding:52px 0 36px;gap:24px}.hero-note{display:none}.directory-tools{margin-bottom:38px}.registry-bar{grid-template-columns:1fr 1fr;margin-bottom:52px}.stat:nth-child(2){border-right:0}.stat:nth-child(-n+2){border-bottom:1px solid rgba(255,255,255,.13)}.record{grid-template-columns:38px 1fr auto;gap:12px}.record-coordinate{grid-column:2/4}.record-time{display:none}.detail-title{grid-template-columns:1fr;align-items:start}.verified{width:max-content}.identity,.layout{grid-template-columns:1fr}.identity-side{border-left:0;border-top:1px solid var(--line)}.layout{gap:10px}.foot{align-items:start;flex-direction:column}.facts{grid-template-columns:1fr 1fr}}
@media(max-width:520px){h1{font-size:3.1rem}.nav{min-height:66px}.nav-links .live{display:inline-flex}.directory-tools{grid-template-columns:1fr}.directory-actions{justify-content:space-between}.registry-bar{grid-template-columns:1fr}.stat{border-right:0;border-bottom:1px solid rgba(255,255,255,.13)}.stat:nth-child(2){border-bottom:1px solid rgba(255,255,255,.13)}.record{grid-template-columns:34px 1fr}.record .type,.record-coordinate{grid-column:2}.identity-main,.identity-side{padding:23px}.facts{grid-template-columns:1fr}.genome-track{grid-template-columns:1fr}.genome-node{border-right:0;border-bottom:1px solid var(--line)}.genome-node:nth-child(2){border-bottom:1px solid var(--line)}.genome-node:not(:last-child):after{content:"↓";right:20px;top:auto;bottom:-12px}.genome-head{align-items:start;flex-direction:column}.parent{grid-template-columns:42px 1fr}.auth{grid-column:2}.artifact-top{display:block}.pagination{gap:8px}.page-link,.page-spacer{min-width:80px}.foot-links{flex-wrap:wrap}}
</style></head><body>
<header class="site-head"><div class="shell nav"><a class="brand" href="/" aria-label="Machine Genome Registry home"><span class="mark"><svg viewBox="0 0 24 24" fill="none" aria-hidden="true"><path d="M5 17V7l7-4 7 4v10l-7 4-7-4Z" stroke="currentColor" stroke-width="1.5"/><circle cx="12" cy="12" r="2.5" fill="currentColor"/><path d="m7.5 8.5 2.2 1.3m4.6 4.4 2.2 1.3m0-7-2.2 1.3m-4.6 4.4-2.2 1.3" stroke="currentColor" stroke-width="1.2"/></svg></span>Machine Genome</a>
<nav class="nav-links" aria-label="Primary"><a href="/#identities">Identities</a><a href="/v1/log/checkpoint">Checkpoint</a><span class="live">{{.Stats.Genes}} genes live</span></nav></div></header>
<main>{{if .View}}
<div class="shell detail-hero"><div class="breadcrumb"><a href="/">Registry</a> / {{.View.Record.Identity.Namespace}} / {{if .View.Registry.Genesis.AgentName}}{{.View.Registry.Genesis.AgentName}}{{else}}{{.View.Record.Identity.Name}}{{end}}</div>
<div class="detail-title"><div><span class="eyebrow">{{.View.Record.Identity.SubjectType}} · {{.View.Record.Identity.Version}}</span><h1>{{if .View.Registry.Genesis.AgentName}}{{.View.Registry.Genesis.AgentName}}{{else}}{{.View.Record.Identity.Name}}{{end}}</h1><p class="description">{{.View.Record.Identity.Description}}</p></div><span class="verified">Included & signed</span></div></div>
<div class="shell"><section class="identity" aria-label="Gene identity"><div class="identity-main"><span class="label">Machine Gene</span><p class="gene">{{.View.Registry.Genesis.Gene}}</p>
<div class="facts">{{if .View.Registry.Genesis.AgentID}}<div class="fact"><span class="label">Agent ID</span><strong class="digest">{{.View.Registry.Genesis.AgentID}}</strong></div>{{end}}{{if .View.Registry.Genesis.ModelID}}<div class="fact"><span class="label">Model coordinate</span><strong>{{.View.Registry.Genesis.ModelProvider}} / {{.View.Registry.Genesis.ModelID}}{{if .View.Registry.Genesis.ModelFamily}} · {{.View.Registry.Genesis.ModelFamily}}{{end}}{{if .View.Registry.Genesis.ContextWindow}} · {{.View.Registry.Genesis.ContextWindow}} tokens{{end}}</strong></div>{{end}}<div class="fact"><span class="label">Namespace</span><strong>{{.View.Record.Identity.Namespace}}</strong></div><div class="fact"><span class="label">Subject type</span><strong>{{.View.Record.Identity.SubjectType}}</strong></div>{{if .View.Registry.Genesis.CreatedDate}}<div class="fact"><span class="label">Created</span><strong>{{.View.Registry.Genesis.CreatedDate}}</strong></div>{{end}}<div class="fact"><span class="label">Registry log</span><strong>Entry {{.View.Registry.Genesis.LogIndex}} of {{.Stats.TreeSize}}</strong></div><div class="fact"><span class="label">Included</span><strong>{{.View.Registry.Genesis.InclusionTime}}</strong></div></div></div>
<div class="identity-side"><span class="label">Controller</span><p class="gene">{{.View.Record.Controller.DID}}</p><div class="fact"><span class="label">Genesis key</span><div class="digest">{{.View.Record.Controller.GenesisKeyID}}</div></div></div></section>
<section class="identity-genome" aria-labelledby="identity-genome-title"><div class="genome-head"><div><span class="eyebrow">Identity at a glance</span><h2 id="identity-genome-title">Identity genome</h2></div><p>Signed claims grouped into the four coordinates agents inspect first.</p></div><div class="genome-track">
<article class="genome-node"><span class="label">Identity</span><strong>{{if .View.Registry.Genesis.AgentName}}{{.View.Registry.Genesis.AgentName}}{{else}}{{.View.Record.Identity.Name}}{{end}}</strong><p>{{if .View.Registry.Genesis.AgentID}}{{.View.Registry.Genesis.AgentID}}{{else}}{{.View.Record.Identity.SubjectType}} · {{.View.Record.Identity.Namespace}}{{end}}</p></article>
<article class="genome-node"><span class="label">Capabilities</span><strong class="genome-number">{{len .View.Record.Capabilities}}</strong><p>{{if .View.Record.Capabilities}}{{range .View.Record.Capabilities}}<a href="/?capability={{.ID | urlquery}}#identities">{{.ID}}</a> {{end}}{{else}}No initial claims{{end}}</p></article>
<article class="genome-node"><span class="label">Training</span><strong class="genome-number">{{len .View.Record.TrainingData}}</strong><p>{{if .View.Record.TrainingData}}declared training-data stages{{else}}No disclosures declared{{end}}</p></article>
<article class="genome-node"><span class="label">Lineage</span><strong class="genome-number">{{len .View.Parents}}</strong><p>{{if .View.Record.Model}}<a href="/?provider={{.View.Record.Model.Provider | urlquery}}#identities">{{.View.Record.Model.Provider}} / {{.View.Record.Model.ModelID}}</a>{{else if .View.Parents}}declared parent relationships{{else}}Root identity{{end}}</p></article>
</div></section>
{{if .View.Registry.Tombstoned}}<p class="notice">The registry has stopped serving this payload. Its cryptographic commitment and inclusion evidence remain.</p>{{end}}
<div class="layout"><div>
<section class="block"><div class="block-title"><h2>Parent lineage</h2><span class="count">{{len .View.Parents}} relationships</span></div>
<div class="lineage">{{range .View.Parents}}{{if .Gene}}<a class="parent" href="/genes/{{.Gene}}"><span class="parent-icon">P</span><span><h3>{{if .Name}}{{.Name}}{{else}}Recorded parent{{end}}</h3><span class="relation">{{.EdgeType}}{{if .SubjectType}} · {{.SubjectType}}{{end}}</span></span>{{if .Authorized}}<span class="auth">Controller authorized</span>{{else}}<span class="auth observed">Observed claim</span>{{end}}</a>{{else}}<div class="parent"><span class="parent-icon">?</span><span><h3>External parent</h3><span class="relation">{{.EdgeType}} · {{.ExternalRef}}</span></span><span class="auth observed">Unresolved</span></div>{{end}}{{else}}<div class="empty-lineage">This is a root record with no declared parents.</div>{{end}}</div></section>
<section class="block"><div class="block-title"><h2>Artifact evidence</h2><span class="count">{{len .View.Record.Genesis.ArtifactCommitments}} commitments</span></div>
{{range .View.Record.Genesis.ArtifactCommitments}}<article class="artifact"><div class="artifact-top"><div><h3>{{.Kind}}</h3>{{if .Note}}<p>{{.Note}}</p>{{end}}</div>{{if .Present}}<span class="present">Source available</span>{{end}}</div><div class="digest">{{.Hash}}</div>{{if .URI}}<p><a href="{{.URI}}" rel="noopener noreferrer">Inspect source evidence ↗</a></p>{{end}}</article>{{else}}<div class="empty-lineage">No artifact commitments were declared.</div>{{end}}</section>
<section class="block"><div class="block-title"><h2>Capabilities</h2><span class="count">{{len .View.Record.Capabilities}} signed claims</span></div>
{{range .View.Record.Capabilities}}<article class="artifact"><div class="artifact-top"><div><h3>{{.Name}}</h3><p><a href="/?capability={{.ID | urlquery}}#identities">{{.ID}}</a>{{if .Category}} · {{.Category}}{{end}}</p>{{if .Description}}<p>{{.Description}}</p>{{end}}</div><span class="present">{{.Level}}</span></div></article>{{else}}<div class="empty-lineage">No initial capability claims were declared.</div>{{end}}</section>
<section class="block"><div class="block-title"><h2>Training data</h2><span class="count">{{len .View.Record.TrainingData}} disclosures</span></div>
{{range .View.Record.TrainingData}}<article class="artifact"><div class="artifact-top"><div><h3>{{.Name}}</h3><p>{{.Stage}}{{if .DataCutoff}} · cutoff {{.DataCutoff}}{{end}}</p>{{if .Description}}<p>{{.Description}}</p>{{end}}</div><span class="present">{{.Disclosure}}</span></div>{{if .Digest}}<div class="digest">{{.Digest}}</div>{{end}}{{if .URI}}<p><a href="{{.URI}}" rel="noopener noreferrer">Inspect disclosure source ↗</a></p>{{end}}</article>{{else}}<div class="empty-lineage">No training-data disclosures were declared.</div>{{end}}</section>
</div><aside><section class="side-card"><span class="label">Registry evidence</span><h3>Public verification trail</h3><div class="metric"><span>Amendments</span><strong>{{len .View.Registry.Amendments}}</strong></div><div class="metric"><span>Attestations</span><strong>{{len .View.Registry.Attestations}}</strong></div><div class="metric"><span>Known descendants</span><strong>{{len .View.Registry.Children}}</strong></div><div class="metric"><span>Conflicting branches</span><strong>{{len .View.Registry.Equivocations}}</strong></div></section>
<section class="side-card"><span class="label">Verify independently</span><h3>Raw cryptographic material</h3><div class="api-links"><a href="/v1/records/{{.View.Registry.Genesis.Gene}}">Canonical record</a><a href="/v1/genes/{{.View.Registry.Genesis.Gene}}">Structured Gene view</a><a href="/v1/log/inclusion/{{.View.Registry.Genesis.LogIndex}}">Merkle inclusion proof</a><a href="/v1/log/checkpoint">Signed checkpoint</a></div></section>
<p class="notice">Lineage records evidence of derivation. It does not imply inherited capability, ownership, endorsement, or legal status.</p></aside></div></div>
{{else}}
<div class="shell"><section class="hero"><div><span class="eyebrow">Open provenance infrastructure</span><h1>Identity for machines. Evidence for everyone.</h1><p class="lede">A public, append-only registry for tracing the models, scaffolds, prompts, and artifacts that make an agent what it is.</p></div><div class="hero-note"><strong>What is a Machine Gene?</strong>An immutable digest of a signed genesis record: a durable anchor for identity, lineage, and evidence.</div></section>
<section class="registry-bar" aria-label="Registry checkpoint"><div class="stat"><strong>{{.Stats.Genes}}</strong><span>registered genes</span></div><div class="stat"><strong>{{.Stats.Objects}}</strong><span>signed records</span></div><div class="stat"><strong>{{.Stats.TreeSize}}</strong><span>log entries</span></div><div class="stat root"><strong>{{.Stats.RootHash}}</strong><span>current Merkle root</span></div></section>
<section id="identities"><div class="section-head"><div><span class="eyebrow">Identity directory</span><h2>Find an agent or model</h2></div><p>{{.Total}} matching records</p></div>
<form class="directory-tools" method="get" role="search" aria-label="Filter identity directory">
<div class="field"><label for="identity-search">Agent ID, name, model, or Gene</label><input id="identity-search" name="q" value="{{.Query}}" placeholder="Search the signed identity"></div>
<div class="field"><label for="identity-type">Identity type</label><select id="identity-type" name="type"><option value="">All identities</option><option value="agent" {{if eq .SubjectType "agent"}}selected{{end}}>Agents</option><option value="model" {{if eq .SubjectType "model"}}selected{{end}}>Models</option><option value="foundation" {{if eq .SubjectType "foundation"}}selected{{end}}>Foundation models</option></select></div>
<div class="field"><label for="identity-provider">Provider</label><select id="identity-provider" name="provider"><option value="">All providers</option>{{range .Providers}}<option value="{{.}}" {{if eq $.Provider .}}selected{{end}}>{{.}}</option>{{end}}</select></div>
<div class="field"><label for="identity-capability">Capability</label><select id="identity-capability" name="capability"><option value="">All capabilities</option>{{range .Capabilities}}<option value="{{.}}" {{if eq $.Capability .}}selected{{end}}>{{.}}</option>{{end}}</select></div>
<div class="directory-actions"><button class="button">Apply</button><a class="reset" href="/#identities">Reset</a></div></form>
<div class="record-list">{{range .Items}}<article class="record"><span class="record-index">#{{.LogIndex}}</span><span><h3><a href="/genes/{{.Gene}}">{{if .AgentName}}{{.AgentName}}{{else}}{{.IdentityName}}{{end}}</a></h3><span class="digest">{{if .AgentID}}{{.AgentID}}{{else}}{{.Namespace}}{{end}}</span></span><span class="record-coordinate">{{if .ModelID}}<strong>{{.ModelProvider}} / {{.ModelID}}</strong>{{if .ModelFamily}}<span class="digest">{{.ModelFamily}}{{if .ContextWindow}} · {{.ContextWindow}} tokens{{end}}</span>{{end}}{{else}}<span class="digest">{{.Gene}}</span>{{end}}</span><span class="record-capabilities">{{if .Capabilities}}<span class="capability-links">{{range .Capabilities}}<a class="chip" href="/?capability={{. | urlquery}}#identities">{{.}}</a>{{end}}</span>{{else}}<span class="digest">No capabilities declared</span>{{end}}</span><span><span class="type">{{if .Foundation}}foundation · {{end}}{{.SubjectType}}</span><span class="record-time">{{if .CreatedDate}}created {{.CreatedDate}}{{else}}{{.InclusionTime}}{{end}}</span></span></article>{{else}}<div class="empty">No matching identities are registered. Try a broader Agent ID, name, capability, provider, or type.</div>{{end}}</div>
<nav class="pagination" aria-label="Identity directory pages">{{if .HasPrevious}}<a class="page-link" rel="prev" href="{{.PreviousURL}}">← Previous</a>{{else}}<span class="page-spacer" aria-hidden="true"></span>{{end}}<span class="page-status"><strong>Page {{.Page}} of {{.PageCount}}</strong>{{if .Total}}Showing {{.PageStart}}–{{.PageEnd}} of {{.Total}}{{else}}No matching identities{{end}}</span>{{if .HasNext}}<a class="page-link" rel="next" href="{{.NextURL}}">Next →</a>{{else}}<span class="page-spacer" aria-hidden="true"></span>{{end}}</nav></section></div>
{{end}}</main><footer><div class="shell foot"><div><a class="brand" href="/"><span class="mark">M</span>Machine Genome</a><p>Verifiable identity and provenance for autonomous systems. Built on signed records and an append-only transparency log.</p></div><div class="foot-links"><a href="/v1/log/checkpoint">Checkpoint</a><a href="/healthz/ready">Service status</a><a href="/v1/search">Search API</a></div></div></footer>
</body></html>`))

func ShutdownServer(ctx context.Context, server *http.Server) error {
	return server.Shutdown(ctx)
}
