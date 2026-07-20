# Changelog

All notable changes to the reference implementation are documented here. The
format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
versioning follows [docs/versioning.md](docs/versioning.md).

## [Unreleased]

### Added

- Publication-grade project, security, governance, contribution, release, API,
  conformance, container, and deployment documentation.
- Responsive registry explorer with lineage, artifact, controller, and
  independent verification views.

## [0.1.1] - 2026-07-20

### Added

- Strict RFC 8785 JSON canonicalization with ambiguous-input rejection.
- SHA-256 multihash Genes and raw artifact commitments.
- W3C `eddsa-jcs-2022` signing and offline Ed25519 `did:key` verification.
- Non-circular lineage authorization and replay-resistant child commitments.
- Ordered controller amendments, rotation-by-sequence, independent
  attestations, and equivocation evidence.
- ACID registry with DNS namespace challenges, tombstones, hash-chained
  administrative audit, online backups, and integrity verification.
- Merkle inclusion and consistency proofs with signed checkpoints.
- Hardened HTTP API, Prometheus metrics, systemd service, TLS proxy, and public
  explorer.
- Initial five-record Neo lineage set.

[Unreleased]: https://github.com/paxlabs-inc/machine-genome/compare/v0.1.1...HEAD
[0.1.1]: https://github.com/paxlabs-inc/machine-genome/releases/tag/v0.1.1
