# Implementation status

Status as of 2026-08-09. This document distinguishes code presence from
operational trust.

| Capability | Implemented | Automated validation | Publicly operated |
| --- | :---: | :---: | :---: |
| Strict RFC 8785 canonicalization | Yes | Yes | Yes |
| SHA-256 multihash Genes | Yes | Yes | Yes |
| `eddsa-jcs-2022` signing and verification | Yes | W3C vector | Yes |
| Offline Ed25519 `did:key` | Yes | Yes | Yes |
| Authorized and observed lineage | Yes | Yes | Yes |
| Controller amendment chains and rotation | Yes | Yes | Yes |
| Independent attestations | Yes | Yes | Yes |
| ACID registry and restart-safe indexes | Yes | Yes | Yes |
| DNS namespace challenges | Yes | Yes | Yes |
| Tombstones and administrative audit chain | Yes | Yes | Yes |
| Merkle inclusion and consistency proofs | Yes | Yes | Yes |
| Signed checkpoints | Yes | Yes | Yes |
| Responsive, paginated identity explorer | Yes | Browser acceptance | Yes |
| MGS 0.2.0 Agent IDs and claimed names | Yes | Unit / registry | Yes |
| Capability and training-data discovery | Yes | Unit / registry / live explorer | Yes |
| Foundation catalog seed generation | Yes | 160-record CLI and live admission | Yes |
| Automated backup and integrity verification | Yes | Yes | Yes |
| Independent witness network | No | No | No |
| Checkpoint gossip / split-view detection | No | No | No |
| Cross-registry federation | No | No | No |
| DID methods beyond `did:key` | No | No | No |
| Threshold control and standardized recovery | No | No | No |
| Independent cryptographic audit | No | No | No |
| Neutral standards governance | No | No | No |

The public instance at
[machinegenome.org](https://machinegenome.org) demonstrates the reference
registry. As of 2026-08-09 it contains 173 Genes, including the original Neo
lineage and 160 observer-signed foundation-model records across 14 providers.
Those model records are third-party catalog observations, not vendor
endorsements. The registry is not represented as a neutral global authority.
