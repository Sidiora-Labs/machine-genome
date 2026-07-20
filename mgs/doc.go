// Package mgs implements the Machine Genome Specification protocol kernel.
//
// It provides strict RFC 8785 JSON canonicalization, content-addressed Genes,
// W3C eddsa-jcs-2022 Data Integrity proofs, offline Ed25519 did:key
// verification, lineage authorization, controller amendment chains, and
// independent attestations.
//
// The package is intentionally transport- and registry-independent. Callers
// must supply explicit key and genesis resolvers when extending the base
// offline profile.
package mgs
