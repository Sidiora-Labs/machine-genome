package registry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/paxlabs-inc/machine-genome/mgs"
	bolt "go.etcd.io/bbolt"
)

var domainPattern = regexp.MustCompile(`^(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z]{2,63}$`)

type TXTResolver interface {
	LookupTXT(context.Context, string) ([]string, error)
}

type NamespaceChallenge struct {
	ID            string `json:"id"`
	Namespace     string `json:"namespace"`
	ControllerDID string `json:"controllerDid"`
	Method        string `json:"method"`
	RecordName    string `json:"recordName"`
	RecordValue   string `json:"recordValue,omitempty"`
	ExpiresAt     string `json:"expiresAt"`
	VerifiedAt    string `json:"verifiedAt,omitempty"`
	tokenHash     [32]byte
}

type NamespaceGrant struct {
	Namespace     string `json:"namespace"`
	ControllerDID string `json:"controllerDid"`
	VerifiedAt    string `json:"verifiedAt"`
	ExpiresAt     string `json:"expiresAt"`
	ChallengeID   string `json:"challengeId"`
}

type storedChallenge struct {
	ID            string `json:"id"`
	Namespace     string `json:"namespace"`
	ControllerDID string `json:"controllerDid"`
	Method        string `json:"method"`
	RecordName    string `json:"recordName"`
	ExpiresAt     string `json:"expiresAt"`
	VerifiedAt    string `json:"verifiedAt,omitempty"`
	TokenHash     string `json:"tokenHash"`
}

func (s *Store) CreateNamespaceChallenge(namespace, controller string) (NamespaceChallenge, string, error) {
	namespace = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(namespace), "."))
	if !domainPattern.MatchString(namespace) {
		return NamespaceChallenge{}, "", errors.New("namespace must be an ASCII DNS name with at least two labels")
	}
	if !strings.HasPrefix(controller, "did:key:") {
		return NamespaceChallenge{}, "", errors.New("controllerDid must be an offline-verifiable did:key")
	}
	multikey := strings.TrimPrefix(controller, "did:key:")
	if _, err := (mgs.DIDKeyResolver{}).ResolveEd25519(context.Background(), controller+"#"+multikey); err != nil {
		return NamespaceChallenge{}, "", errors.New("controllerDid is not a valid Ed25519 did:key")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return NamespaceChallenge{}, "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	idBytes := make([]byte, 18)
	if _, err := rand.Read(idBytes); err != nil {
		return NamespaceChallenge{}, "", err
	}
	id := base64.RawURLEncoding.EncodeToString(idBytes)
	now := s.now().UTC().Truncate(time.Second)
	hash := sha256.Sum256([]byte(token))
	challenge := NamespaceChallenge{
		ID: id, Namespace: namespace, ControllerDID: controller, Method: "dns-01",
		RecordName:  "_mgs-challenge." + namespace,
		RecordValue: "mgs-registry=" + token,
		ExpiresAt:   now.Add(15 * time.Minute).Format(time.RFC3339),
		tokenHash:   hash,
	}
	stored := storedChallenge{
		ID: challenge.ID, Namespace: challenge.Namespace, ControllerDID: challenge.ControllerDID,
		Method: challenge.Method, RecordName: challenge.RecordName, ExpiresAt: challenge.ExpiresAt,
		TokenHash: base64.RawURLEncoding.EncodeToString(hash[:]),
	}
	encoded, _ := json.Marshal(stored)
	err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketChallenges)
		cursor := bucket.Cursor()
		active := 0
		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			var existing storedChallenge
			existingExpiry, parseErr := time.Time{}, error(nil)
			if decodeErr := json.Unmarshal(value, &existing); decodeErr == nil {
				existingExpiry, parseErr = time.Parse(time.RFC3339, existing.ExpiresAt)
			}
			if parseErr != nil || !now.Before(existingExpiry) {
				if err := cursor.Delete(); err != nil {
					return err
				}
				continue
			}
			active++
		}
		if active >= 100_000 {
			return errors.New("registry has reached its active namespace challenge limit")
		}
		if err := bucket.Put([]byte(id), encoded); err != nil {
			return err
		}
		return s.appendAudit(tx, "namespace.challenge.created", "public", id, map[string]any{
			"namespace": namespace, "controllerDid": controller,
		})
	})
	if err != nil {
		return NamespaceChallenge{}, "", err
	}
	return challenge, token, nil
}

func (s *Store) VerifyNamespaceChallenge(ctx context.Context, id, token string, resolver TXTResolver) (NamespaceGrant, error) {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	var stored storedChallenge
	err := s.db.View(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketChallenges).Get([]byte(id))
		if value == nil {
			return ErrNotFound
		}
		return json.Unmarshal(value, &stored)
	})
	if err != nil {
		return NamespaceGrant{}, err
	}
	expires, err := time.Parse(time.RFC3339, stored.ExpiresAt)
	if err != nil || !s.now().Before(expires) {
		return NamespaceGrant{}, errors.New("namespace challenge expired")
	}
	wantHash, err := base64.RawURLEncoding.DecodeString(stored.TokenHash)
	if err != nil {
		return NamespaceGrant{}, errors.New("stored namespace challenge is corrupt")
	}
	gotHash := sha256.Sum256([]byte(token))
	if len(wantHash) != len(gotHash) || subtle.ConstantTimeCompare(wantHash, gotHash[:]) != 1 {
		return NamespaceGrant{}, errors.New("namespace challenge token is invalid")
	}
	records, err := resolver.LookupTXT(ctx, stored.RecordName)
	if err != nil {
		return NamespaceGrant{}, fmt.Errorf("look up namespace TXT record: %w", err)
	}
	expected := "mgs-registry=" + token
	found := false
	for _, record := range records {
		if subtle.ConstantTimeCompare([]byte(record), []byte(expected)) == 1 {
			found = true
		}
	}
	if !found {
		return NamespaceGrant{}, errors.New("namespace TXT record does not contain the challenge value")
	}
	now := s.now().UTC().Truncate(time.Second)
	grant := NamespaceGrant{
		Namespace: stored.Namespace, ControllerDID: stored.ControllerDID,
		VerifiedAt: now.Format(time.RFC3339), ExpiresAt: now.Add(30 * 24 * time.Hour).Format(time.RFC3339),
		ChallengeID: stored.ID,
	}
	encodedGrant, _ := json.Marshal(grant)
	err = s.db.Update(func(tx *bolt.Tx) error {
		value := tx.Bucket(bucketChallenges).Get([]byte(id))
		if value == nil {
			return ErrNotFound
		}
		var current storedChallenge
		if err := json.Unmarshal(value, &current); err != nil {
			return err
		}
		if current.VerifiedAt != "" {
			return errors.New("namespace challenge was already consumed")
		}
		current.VerifiedAt = grant.VerifiedAt
		encodedChallenge, _ := json.Marshal(current)
		if err := tx.Bucket(bucketChallenges).Put([]byte(id), encodedChallenge); err != nil {
			return err
		}
		if err := tx.Bucket(bucketGrants).Put(joinKey(grant.Namespace, grant.ControllerDID), encodedGrant); err != nil {
			return err
		}
		return s.appendAudit(tx, "namespace.challenge.verified", "public", id, map[string]any{
			"namespace": grant.Namespace, "controllerDid": grant.ControllerDID,
			"grantExpiresAt": grant.ExpiresAt,
		})
	})
	return grant, err
}
