// Package registry implements the verified single-node Machine Genome registry.
//
// Store.Submit is the durable admission boundary: records are canonicalized,
// cryptographically verified, checked against lineage, chain, and namespace
// state, then atomically indexed and appended to the transparency log.
//
// The package also provides the hardened HTTP API, signed checkpoints, Merkle
// inclusion and consistency proofs, online backups, and a read-only provenance
// explorer.
package registry
