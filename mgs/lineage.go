package mgs

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
)

type GenesisResolver interface {
	ResolveGenesis(context.Context, string) ([]byte, error)
}

type LineageAuthorization struct {
	MGS             string `json:"mgs"`
	RecordType      string `json:"recordType"`
	ChildCommitment string `json:"childCommitment"`
	ChildController string `json:"childController"`
	ParentGene      string `json:"parentGene"`
	EdgeType        string `json:"edgeType"`
}

// ChildCommitment hashes the genesis with the top-level proof and every
// authorizationProof removed. This is the stable, non-circular value signed by
// every consenting parent.
func ChildCommitment(genesis []byte) (string, error) {
	object, err := decodeObject(genesis)
	if err != nil {
		return "", err
	}
	if object["recordType"] != RecordGenesis {
		return "", errors.New("child commitment requires a genesis record")
	}
	delete(object, "proof")
	if parentage, exists := object["parentage"]; exists {
		edges, ok := parentage.([]any)
		if !ok {
			return "", errors.New("genesis parentage must be an array")
		}
		for index, rawEdge := range edges {
			edge, ok := rawEdge.(map[string]any)
			if !ok {
				return "", fmt.Errorf("parentage[%d] must be an object", index)
			}
			delete(edge, "authorizationProof")
		}
	}
	canonical, err := encodeObject(object)
	if err != nil {
		return "", err
	}
	return Digest(canonical)
}

func authorizationDocument(genesis []byte, edgeIndex int) (LineageAuthorization, error) {
	var record GenesisRecord
	if err := unmarshal(genesis, &record); err != nil {
		return LineageAuthorization{}, err
	}
	if record.RecordType != RecordGenesis || !supportedVersion(record.MGS) {
		return LineageAuthorization{}, errors.New("lineage authorization requires a supported MGS genesis")
	}
	if edgeIndex < 0 || edgeIndex >= len(record.Parentage) {
		return LineageAuthorization{}, fmt.Errorf("edge index %d is out of range", edgeIndex)
	}
	edge := record.Parentage[edgeIndex]
	if !edge.Authorized || edge.Unresolved || edge.ParentGene == "" {
		return LineageAuthorization{}, errors.New("only a resolved authorized edge can receive an authorization proof")
	}
	if !edgeTypes[edge.EdgeType] {
		return LineageAuthorization{}, fmt.Errorf("unknown edgeType %q", edge.EdgeType)
	}
	commitment, err := ChildCommitment(genesis)
	if err != nil {
		return LineageAuthorization{}, err
	}
	return LineageAuthorization{
		MGS:             record.MGS,
		RecordType:      RecordAuthorization,
		ChildCommitment: commitment,
		ChildController: record.Controller.DID,
		ParentGene:      edge.ParentGene,
		EdgeType:        edge.EdgeType,
	}, nil
}

// AuthorizeEdge signs a deterministic authorization document and embeds only
// its Data Integrity proof in the requested edge.
func AuthorizeEdge(genesis []byte, edgeIndex int, options ProofOptions, privateKey ed25519.PrivateKey) ([]byte, error) {
	authorization, err := authorizationDocument(genesis, edgeIndex)
	if err != nil {
		return nil, err
	}
	payload, err := pretty(authorization)
	if err != nil {
		return nil, err
	}
	secured, err := SignDocument(payload, options, privateKey)
	if err != nil {
		return nil, err
	}
	var securedAuthorization struct {
		Proof Proof `json:"proof"`
	}
	if err := unmarshal(secured, &securedAuthorization); err != nil {
		return nil, err
	}
	object, err := decodeObject(genesis)
	if err != nil {
		return nil, err
	}
	edges := object["parentage"].([]any)
	edge := edges[edgeIndex].(map[string]any)
	proofMap := map[string]any{}
	proofRaw, _ := json.Marshal(securedAuthorization.Proof)
	if err := json.Unmarshal(proofRaw, &proofMap); err != nil {
		return nil, err
	}
	edge["authorizationProof"] = proofMap
	return pretty(object)
}

// VerifyLineage resolves every parent and verifies every consenting-parent
// signature. It currently accepts keys from the exact parent genesis
// controller; historical rotated keys belong to the registry profile.
func VerifyLineage(ctx context.Context, genesis []byte, keys KeyResolver, parents GenesisResolver) error {
	record, err := ValidateGenesis(genesis, true)
	if err != nil {
		return err
	}
	for index, edge := range record.Parentage {
		if edge.Unresolved {
			continue
		}
		parentRaw, err := parents.ResolveGenesis(ctx, edge.ParentGene)
		if err != nil {
			return fmt.Errorf("parentage[%d]: resolve parent: %w", index, err)
		}
		if err := DigestMatches(parentRaw, edge.ParentGene); err != nil {
			return fmt.Errorf("parentage[%d]: %w", index, err)
		}
		parentGene, err := VerifyGenesis(ctx, parentRaw, keys)
		if err != nil {
			return fmt.Errorf("parentage[%d]: invalid parent genesis: %w", index, err)
		}
		if parentGene != edge.ParentGene {
			return fmt.Errorf("parentage[%d]: resolved parent Gene mismatch", index)
		}
		parent, err := ValidateGenesis(parentRaw, true)
		if err != nil {
			return fmt.Errorf("parentage[%d]: invalid parent genesis: %w", index, err)
		}
		if !edge.Authorized {
			continue
		}
		authorization, err := authorizationDocument(genesis, index)
		if err != nil {
			return fmt.Errorf("parentage[%d]: %w", index, err)
		}
		payloadObject := map[string]any{}
		payloadRaw, _ := json.Marshal(authorization)
		if err := json.Unmarshal(payloadRaw, &payloadObject); err != nil {
			return err
		}
		proofRaw, _ := json.Marshal(edge.AuthorizationProof)
		var proofObject map[string]any
		if err := json.Unmarshal(proofRaw, &proofObject); err != nil {
			return err
		}
		payloadObject["proof"] = proofObject
		securedAuthorization, err := pretty(payloadObject)
		if err != nil {
			return err
		}
		method, err := VerifyDocument(ctx, securedAuthorization, keys)
		if err != nil {
			return fmt.Errorf("parentage[%d]: authorization proof: %w", index, err)
		}
		controller, _ := verificationController(method)
		if controller != parent.Controller.DID {
			return fmt.Errorf("parentage[%d]: authorization signer %q is not parent controller %q", index, controller, parent.Controller.DID)
		}
		if method != parent.Controller.GenesisKeyID {
			return fmt.Errorf("parentage[%d]: implementation profile only accepts the parent genesis key", index)
		}
		if edge.AuthorizationProof.Created > record.Genesis.Timestamp {
			return fmt.Errorf("parentage[%d]: authorization was created after child genesis", index)
		}
	}
	return nil
}

type MapGenesisResolver map[string][]byte

func (records MapGenesisResolver) ResolveGenesis(_ context.Context, gene string) ([]byte, error) {
	record, exists := records[gene]
	if !exists {
		return nil, fmt.Errorf("gene %s not found", gene)
	}
	return append([]byte(nil), record...), nil
}
