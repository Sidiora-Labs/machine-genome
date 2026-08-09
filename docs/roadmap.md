# Reference implementation roadmap

## Milestone 0 — protocol kernel

Implemented in this working tree:

- byte-exact JCS, multihash, Gene, and Data Integrity operations;
- offline Ed25519 `did:key`;
- genesis structure and controller binding;
- non-circular parent authorization and resolved-parent verification;
- controller amendment chain, rotation-by-sequence, and terminal state;
- independent issuer attestations;
- JSON Schemas, CLI, W3C vector, MGS golden vector, and negative tests.

## Milestone 1 — conformance release

Implemented in this working tree:

- machine-readable RFC 8785 and MGS Gene vectors loaded by the Go suite;
- JSON Schema, OpenAPI, seeded fuzz/property tests, race tests, CodeQL, and
  vulnerability scanning in CI;
- stable HTTP problem codes, media types, release packaging, checksums, and
  SBOM configuration;
- public security, governance, contribution, compatibility, and release
  policies.

Remaining:

- historical controller-state and standardized recovery-policy vectors;
- independent cryptographic and privacy review;
- cross-language vectors validated by another implementation;
- an implementer report with at least two independent implementations.

## Milestone 2 — local registry and explorer

Implemented in this working tree:

- ACID immutable object store plus per-controller chain validation;
- Gene, lineage, attestation, identity, and namespace queries;
- local web explorer with plain-language provenance and evidence views;
- DNS namespace challenge flow with controller-scoped, expiring grants;
- transparent tombstones that retain commitments and proofs;
- constant-time bearer authentication for administrative operations;
- hash-chained audit events, consistent online backups, and integrity checking;
- bounded requests, per-client rate limiting, secure headers, health and metrics.

## Milestone 3 — public transparency service

- Merkle inclusion/consistency proofs and signed checkpoints (implemented);
- TLS proxying, per-client limits, hardened systemd, monitoring endpoints,
  online backup, restore verification, and public deployment (implemented);
- independent witnesses and gossip;
- cross-registry discovery and equivocation presentation;
- staking/fees if required and content-neutral abuse controls;
- independent restore drills, checkpoint key ceremony, incident exercises, and
  published SLO evidence.

## Milestone 4 — agent accounts

- Gene-derived Agent IDs, claimed names, capability discovery, training-data
  declarations, and foundation-model catalog generation (implemented);
- companion challenge and account-binding specification;
- pairwise DIDs and selective disclosure;
- dedicated agent mailbox reference service;
- browser-native signup demo with confirmation link/code handling;
- sponsor/legal delegation credentials and explicit high-impact approvals;
- service SDKs that add “Sign up as an agent” without depending on one registry.

## Milestone 5 — neutral governance

- independent working group and public change process;
- namespace for edge, amendment, attestation, and evidence vocabularies;
- compatibility and deprecation policy;
- intellectual-property and trademark commitments;
- conformance mark governed independently from any registry operator.
