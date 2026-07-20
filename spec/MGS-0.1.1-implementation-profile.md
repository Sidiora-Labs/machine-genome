# Machine Genome Specification

Version: `0.1.1-implementation-profile`
Status: Experimental interoperability profile

This document tightens the supplied `0.1.0-draft` where a byte-exact
implementation decision is required. It does not claim standards-body approval.

## 1. Record families

An MGS implementation recognizes three independently signed families:

1. `genesis`: immutable subject identity and lineage. Its complete secured JCS
   representation hashes to the Gene.
2. `amendment`: an ordered controller-authored chain rooted at a Gene.
3. `attestation`: a third-party statement referencing a Gene and chained by its
   issuer, never inserted into the subject controller's amendment chain.

MGS records are I-JSON objects. Unknown lineage edge types are rejected. Unknown
amendment and attestation types are preserved but have no protocol semantics.

## 2. Canonicalization and content identifiers

Canonical JSON is RFC 8785 JCS. Implementations MUST reject duplicate object
members, invalid UTF-8, non-finite or non-I-JSON numbers, negative zero, and
trailing JSON values.

`digest(value)` is:

```text
"z" || base58btc(0x12 || 0x20 || SHA-256(JCS(value)))
```

The Gene is `digest(complete secured genesis)`, including its Data Integrity
proof. An amendment or attestation record hash is calculated the same way over
the complete secured record.

## 3. Data Integrity proofs

The required suite is W3C `eddsa-jcs-2022`. Proof options contain:

```json
{
  "type": "DataIntegrityProof",
  "cryptosuite": "eddsa-jcs-2022",
  "created": "2026-07-20T14:00:00Z",
  "verificationMethod": "did:key:z...#z...",
  "proofPurpose": "assertionMethod"
}
```

The signed bytes are:

```text
SHA-256(JCS(proof options)) || SHA-256(JCS(document without proof))
```

and `proofValue` is multibase base58btc of the 64-byte Ed25519 signature.
MGS timestamps use RFC 3339 UTC, a `Z` suffix, and exactly second precision.

The verifier MUST establish that the verification method is authorized for the
`assertionMethod` relationship of the expected controller. Offline `did:key`
resolution is mandatory in this profile. Other DID methods are adapter-defined.

## 4. Genesis and lineage

The subject types and closed lineage edge vocabulary are those in the source
`0.1.0-draft`.

To authorize an edge without a signature cycle:

1. Begin with a genesis that has no top-level `proof`.
2. Remove `authorizationProof` from every parentage edge.
3. Compute `childCommitment = digest(result)`.
4. Reconstruct this authorization document:

```json
{
  "mgs": "0.1.1",
  "recordType": "lineage-authorization",
  "childCommitment": "z...",
  "childController": "did:key:z...",
  "parentGene": "z...",
  "edgeType": "instance-of"
}
```

5. The parent controller signs that document using the Data Integrity algorithm.
6. Store only its resulting `proof` object as the edge's
   `authorizationProof`.
7. After all edge proofs are present, the child controller signs the complete
   unsecured genesis. The Gene hashes that final secured genesis.

Verification repeats these steps, resolves the exact parent genesis, proves that
its content hashes to `parentGene`, and checks that the authorization signer is a
controller authorized by the parent state. The core profile supports the parent
genesis controller. Historical rotated-controller proofs require the registry
profile.

An unauthorized edge has `authorized: false`, no authorization proof, and is
accepted. An unresolved historical edge additionally has `unresolved: true` and
an `externalRef`; it carries no cryptographic authority.

## 5. Controller amendments

For the first amendment, `seq` is `1` and `prev` equals the Gene. Each later
record has `seq = previous.seq + 1` and `prev = digest(previous secured
amendment)`. Every amendment is signed by the active subject controller.

A `controller-rotation` body names `newControllerDid`. It is signed by the old
controller and becomes active after that sequence position, never by the
self-reported timestamp. Terminal supersession permits only later `dispute`
records.

Two valid records at the same `(gene, seq, prev)` are controller equivocation.
Verifiers preserve and surface both branches; registries MUST NOT silently pick
one.

## 6. Third-party attestations

An attestation contains `subjectGene`, `issuer`, `attestationType`, `body`, and
an optional issuer-local `seq`/`prev`. The proof signer must equal `issuer`.
Registry inclusion ordering is independent from the subject amendment chain.

Attestation evidence uses orthogonal dimensions:

```json
{
  "authorship": "self|operator|third-party",
  "review": "none|independent",
  "reproducibility": "unavailable|partial|reproducible",
  "runtimeBinding": "none|hardware"
}
```

Every evidence URI is paired with an immutable hash. A subject controller may
issue an acknowledgment, mitigation, or dispute amendment referencing the
attestation hash.

## 7. Registry requirements

A conforming public registry:

- verifies records before inclusion;
- preserves canonical bytes and returns content hashes;
- rejects local chain conflicts and exposes observed ecosystem equivocation;
- returns registry inclusion time separately from author time;
- provides append-only Merkle inclusion and consistency proofs;
- signs checkpoints and supports witness/gossip verification;
- accepts valid unauthorized lineage and third-party attestations under
  content-neutral anti-abuse rules;
- verifies namespace control through a documented challenge protocol;
- supports transparent availability tombstones without pretending that a
  cryptographic commitment was deleted.

Registry APIs, federation, and governance are outside this core profile.
