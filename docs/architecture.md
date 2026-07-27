# Reference architecture

This document describes the Go reference implementation and single-node
registry. The normative byte and proof rules remain in the versioned protocol
profile.

## Components

```text
┌─────────────────────────────────────────────────────────────────────┐
│ author / controller                                                 │
│  mgs CLI or Go package: author → canonicalize → sign → verify       │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ application/mgs+json
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│ HTTP boundary                                                       │
│  size/media limits · request IDs · rate limit · security headers    │
└──────────────────────────────┬──────────────────────────────────────┘
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│ registry.Store.Submit                                               │
│  strict parse → signature → controller → lineage → namespace        │
│  → chain state → one ACID admission transaction                     │
└───────────────┬───────────────────────────────┬─────────────────────┘
                │                               │
                ▼                               ▼
┌──────────────────────────────┐  ┌──────────────────────────────────┐
│ bbolt object/index database  │  │ append-only digest log           │
│ canonical bytes + metadata   │  │ Merkle proofs + signed root      │
└───────────────┬──────────────┘  └────────────────┬─────────────────┘
                │                                  │
                └────────────────┬─────────────────┘
                                 ▼
                     API · explorer · metrics
```

## Package boundaries

### `mgs`

The protocol kernel is stateless. It owns strict JSON handling, canonical
digests, DID key resolution, proof algorithms, record validation, lineage
authorization, amendment chains, and attestations.

`KeyResolver` and `GenesisResolver` are explicit trust adapters. The default
profile supplies only offline Ed25519 `did:key`; adding a network resolver must
define SSRF controls, historical resolution, caching, revocation, and failure
semantics.

### `registry`

The registry package owns admission policy and durable state. `Store.Submit` is
the shared verification boundary for every transport. It canonicalizes input
again, dispatches by record family, verifies cryptographic and lineage state,
enforces namespace control, then writes the object, indexes, log leaf, and audit
event in one bbolt transaction.

The HTTP layer owns protocol translation, bounded bodies, authentication,
operational endpoints, signed checkpoints, and the read-only explorer. It does
not receive a method that inserts an unverified object.

### `cmd/mgs`

The CLI is a thin composition layer. It deliberately keeps key generation and
offline verification usable without a running registry.

## Durable data model

The database uses separate buckets for:

- canonical objects keyed by digest;
- immutable metadata keyed by digest;
- Genes and identity coordinates;
- ordered amendment and attestation indexes;
- parent-to-child relationships;
- observed equivocation branches;
- transparency log positions;
- namespace challenges and grants;
- tombstones;
- chained administrative audit events;
- database schema metadata.

The object digest, metadata digest, and log leaf must agree. `registry-verify`
checks object integrity and the administrative audit chain. bbolt supplies
single-node ACID transactions; it is not a distributed consensus system.

## Record admission

### Genesis

1. Reject input outside the size and canonical JSON constraints.
2. Validate the secured genesis structure.
3. Verify the controller's Data Integrity proof.
4. Recompute the Gene.
5. Resolve every local parent and verify authorized edges.
6. Enforce DID or DNS namespace control.
7. Reject an occupied identity coordinate.
8. Atomically store canonical bytes, metadata, lineage indexes, log leaf, and
   audit event.

### Amendment

The store resolves genesis and all accepted prior amendments, verifies the
complete chain and active controller, then accepts the next sequence. A valid
competing record for an already occupied `(gene, seq, prev)` is retained as
equivocation evidence but not selected as canonical.

### Attestation

The store verifies the independent issuer and requires the subject Gene to
exist. It does not insert the attestation into the subject controller chain.

## Transparency

Every admitted object digest is a Merkle leaf in inclusion order. The registry
uses RFC 9162-style domain separation:

- leaf hash: `SHA-256(0x00 || leaf bytes)`;
- node hash: `SHA-256(0x01 || left || right)`.

Checkpoints contain tree size, root hash, registry DID, verification method,
timestamp, algorithm, and an Ed25519/JCS signature. Inclusion and consistency
proof verification is available as library code so observers need not trust the
serving registry.

## Deployment topology

The production reference topology is:

```text
client → DNS / optional Cloudflare → nginx TLS + public rate limit
       → loopback-only mgs process → private bbolt volume
                                  ↘ private backup directory
```

The checkpoint key and administrator token enter through systemd credentials.
The process runs under a dedicated user with a strict systemd sandbox. nginx
owns public client-address interpretation; the Go service deliberately ignores
forwarded address headers.

## Failure behavior

- Invalid or ambiguous records fail before storage.
- A failed transaction cannot leave an object without its indexes or log leaf.
- A tombstone preserves commitment and inclusion evidence.
- Process restart reopens the same durable indexes and log sequence.
- A database lock fails startup rather than creating a second writer.
- Backup uses a consistent read transaction and an exclusive output file.
- An unavailable DNS resolver fails namespace verification closed.
- Unknown record families and lineage edge types are rejected.

## Extension rules

Preserve a single admission boundary. A new transport, UI, or import tool must
call `Store.Submit`. New DID adapters must implement explicit network and
historical-state policy. Changes to accepted canonical bytes, proof meaning, or
closed vocabularies require a profile version and conformance vectors.
