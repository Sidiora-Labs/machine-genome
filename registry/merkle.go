package registry

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

type InclusionProof struct {
	LogIndex  uint64   `json:"logIndex"`
	TreeSize  uint64   `json:"treeSize"`
	Leaf      string   `json:"leaf"`
	LeafHash  string   `json:"leafHash"`
	RootHash  string   `json:"rootHash"`
	AuditPath []string `json:"auditPath"`
}

type ConsistencyProof struct {
	FirstSize  uint64   `json:"firstSize"`
	SecondSize uint64   `json:"secondSize"`
	FirstRoot  string   `json:"firstRoot"`
	SecondRoot string   `json:"secondRoot"`
	Path       []string `json:"path"`
}

func (s *Store) InclusionProof(index, size uint64) (InclusionProof, error) {
	leaves, err := s.LogLeaves()
	if err != nil {
		return InclusionProof{}, err
	}
	if size == 0 {
		size = uint64(len(leaves))
	}
	if size > uint64(len(leaves)) || index >= size {
		return InclusionProof{}, errors.New("inclusion proof index or tree size is out of range")
	}
	hashes := make([][]byte, size)
	for i := range hashes {
		hashes[i] = merkleLeaf([]byte(leaves[i]))
	}
	path := inclusionPath(hashes, int(index))
	encodedPath := make([]string, len(path))
	for i, hash := range path {
		encodedPath[i] = base64.RawURLEncoding.EncodeToString(hash)
	}
	return InclusionProof{
		LogIndex: index, TreeSize: size, Leaf: leaves[index],
		LeafHash:  base64.RawURLEncoding.EncodeToString(hashes[index]),
		RootHash:  base64.RawURLEncoding.EncodeToString(merkleRoot(hashes)),
		AuditPath: encodedPath,
	}, nil
}

func (s *Store) ConsistencyProof(firstSize, secondSize uint64) (ConsistencyProof, error) {
	leaves, err := s.LogLeaves()
	if err != nil {
		return ConsistencyProof{}, err
	}
	if firstSize == 0 || firstSize > secondSize || secondSize > uint64(len(leaves)) {
		return ConsistencyProof{}, errors.New("consistency proof sizes must satisfy 0 < firstSize <= secondSize <= current tree size")
	}
	hashes := make([][]byte, secondSize)
	for i := range hashes {
		hashes[i] = merkleLeaf([]byte(leaves[i]))
	}
	path := consistencyPath(hashes, int(firstSize), true)
	encoded := make([]string, len(path))
	for i, hash := range path {
		encoded[i] = base64.RawURLEncoding.EncodeToString(hash)
	}
	return ConsistencyProof{
		FirstSize: firstSize, SecondSize: secondSize,
		FirstRoot:  base64.RawURLEncoding.EncodeToString(merkleRoot(hashes[:firstSize])),
		SecondRoot: base64.RawURLEncoding.EncodeToString(merkleRoot(hashes)),
		Path:       encoded,
	}, nil
}

func TreeRoot(leaves []string) string {
	hashes := make([][]byte, len(leaves))
	for i, leaf := range leaves {
		hashes[i] = merkleLeaf([]byte(leaf))
	}
	return base64.RawURLEncoding.EncodeToString(merkleRoot(hashes))
}

func VerifyInclusion(proof InclusionProof) error {
	leafHash, err := base64.RawURLEncoding.DecodeString(proof.LeafHash)
	if err != nil {
		return err
	}
	if actual := merkleLeaf([]byte(proof.Leaf)); !equalBytes(actual, leafHash) {
		return errors.New("inclusion leaf hash mismatch")
	}
	index := proof.LogIndex
	last := proof.TreeSize - 1
	hash := leafHash
	for _, encoded := range proof.AuditPath {
		sibling, err := base64.RawURLEncoding.DecodeString(encoded)
		if err != nil {
			return err
		}
		if index&1 == 1 || index == last {
			hash = merkleNode(sibling, hash)
			for index&1 == 0 && index != 0 {
				index >>= 1
				last >>= 1
			}
		} else {
			hash = merkleNode(hash, sibling)
		}
		index >>= 1
		last >>= 1
	}
	root, err := base64.RawURLEncoding.DecodeString(proof.RootHash)
	if err != nil {
		return err
	}
	if !equalBytes(hash, root) {
		return fmt.Errorf("inclusion root mismatch")
	}
	return nil
}

func VerifyConsistency(proof ConsistencyProof) error {
	if proof.FirstSize == 0 || proof.FirstSize > proof.SecondSize {
		return errors.New("invalid consistency proof sizes")
	}
	firstRoot, err := base64.RawURLEncoding.DecodeString(proof.FirstRoot)
	if err != nil {
		return err
	}
	secondRoot, err := base64.RawURLEncoding.DecodeString(proof.SecondRoot)
	if err != nil {
		return err
	}
	if proof.FirstSize == proof.SecondSize {
		if len(proof.Path) != 0 || !equalBytes(firstRoot, secondRoot) {
			return errors.New("equal-size consistency proof is invalid")
		}
		return nil
	}
	path := make([][]byte, len(proof.Path))
	for i, encoded := range proof.Path {
		path[i], err = base64.RawURLEncoding.DecodeString(encoded)
		if err != nil || len(path[i]) != sha256.Size {
			return errors.New("invalid consistency proof hash")
		}
	}
	firstNode := proof.FirstSize - 1
	secondNode := proof.SecondSize - 1
	for firstNode&1 == 1 {
		firstNode >>= 1
		secondNode >>= 1
	}
	var firstHash, secondHash []byte
	if firstNode == 0 {
		firstHash = append([]byte(nil), firstRoot...)
		secondHash = append([]byte(nil), firstRoot...)
	} else {
		if len(path) == 0 {
			return errors.New("consistency proof path is empty")
		}
		firstHash = append([]byte(nil), path[0]...)
		secondHash = append([]byte(nil), path[0]...)
		path = path[1:]
	}
	for _, hash := range path {
		if secondNode == 0 {
			return errors.New("consistency proof has extra nodes")
		}
		if firstNode&1 == 1 || firstNode == secondNode {
			firstHash = merkleNode(hash, firstHash)
			secondHash = merkleNode(hash, secondHash)
			for firstNode&1 == 0 && firstNode != 0 {
				firstNode >>= 1
				secondNode >>= 1
			}
		} else {
			secondHash = merkleNode(secondHash, hash)
		}
		firstNode >>= 1
		secondNode >>= 1
	}
	if secondNode != 0 || !equalBytes(firstHash, firstRoot) || !equalBytes(secondHash, secondRoot) {
		return errors.New("consistency proof root mismatch")
	}
	return nil
}

func merkleLeaf(data []byte) []byte {
	sum := sha256.Sum256(append([]byte{0}, data...))
	return sum[:]
}

func merkleNode(left, right []byte) []byte {
	input := make([]byte, 1, 1+len(left)+len(right))
	input[0] = 1
	input = append(input, left...)
	input = append(input, right...)
	sum := sha256.Sum256(input)
	return sum[:]
}

func merkleRoot(leaves [][]byte) []byte {
	if len(leaves) == 0 {
		sum := sha256.Sum256(nil)
		return sum[:]
	}
	if len(leaves) == 1 {
		return append([]byte(nil), leaves[0]...)
	}
	split := largestPowerOfTwoLessThan(len(leaves))
	return merkleNode(merkleRoot(leaves[:split]), merkleRoot(leaves[split:]))
}

func inclusionPath(leaves [][]byte, index int) [][]byte {
	if len(leaves) <= 1 {
		return nil
	}
	split := largestPowerOfTwoLessThan(len(leaves))
	if index < split {
		return append(inclusionPath(leaves[:split], index), merkleRoot(leaves[split:]))
	}
	return append(inclusionPath(leaves[split:], index-split), merkleRoot(leaves[:split]))
}

func consistencyPath(leaves [][]byte, firstSize int, complete bool) [][]byte {
	if firstSize == len(leaves) {
		if complete {
			return nil
		}
		return [][]byte{merkleRoot(leaves)}
	}
	split := largestPowerOfTwoLessThan(len(leaves))
	if firstSize <= split {
		return append(consistencyPath(leaves[:split], firstSize, complete), merkleRoot(leaves[split:]))
	}
	return append(consistencyPath(leaves[split:], firstSize-split, false), merkleRoot(leaves[:split]))
}

func largestPowerOfTwoLessThan(value int) int {
	power := 1
	for power<<1 < value {
		power <<= 1
	}
	return power
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var result byte
	for i := range left {
		result |= left[i] ^ right[i]
	}
	return result == 0
}
