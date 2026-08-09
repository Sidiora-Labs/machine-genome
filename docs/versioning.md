# Versioning and compatibility

Machine Genome versions the protocol profile, Go modules, CLI, HTTP API, and
database schema as related but distinct surfaces.

## Release versions

Tagged software releases use semantic versioning: `vMAJOR.MINOR.PATCH`.
Before `v1.0.0`, minor releases may contain intentionally documented breaking
changes. Patch releases are expected to preserve valid behavior except where a
security or consensus-integrity fix must fail closed.

## Protocol field

The `mgs` record field identifies the protocol profile, currently `0.2.0`.
The reference implementation continues to verify immutable `0.1.1` records.
Implementations reject unsupported profile strings. A software release may
support more than one profile in the future, but acceptance must never change
silently.

A protocol-major change is required for:

- different canonical bytes or identifier derivation;
- proof algorithm or proof-purpose changes;
- changed controller authorization;
- incompatible record fields or required values;
- additions to a closed vocabulary when older implementations reject unknown
  values;
- changed lineage commitment or chain semantics.

Editorial clarification that does not change accepted bytes may keep the same
profile and receives an explicit document revision note.

## HTTP API

Stable registry resources are under `/v1`. Compatible additions may add
optional response members or endpoints. Removing or changing a field, status,
media type, authentication rule, or proof meaning requires a new API version.
Clients must ignore optional response fields but must not ignore verification
failures.

## Go API

The module follows normal Go compatibility expectations within a tagged major
version. Pre-`v1` packages may evolve, but public identifier removal requires a
release note and migration guidance.

## Database schema

The database records an internal schema version. A future incompatible change
must ship an explicit, backup-first migration and refuse to open unsupported
newer state. Silent destructive migration is prohibited.

## Deprecation

A deprecation notice identifies the affected surface, replacement, first
deprecated release, earliest removal release, and security implications.
Cryptographic algorithms are never downgraded automatically for compatibility.

## Release process

1. Merge with all required checks passing.
2. Update `CHANGELOG.md`, status, protocol profile, schemas, OpenAPI, and vectors
   as applicable.
3. Run `make release-check VERSION=x.y.z` on a clean tree.
4. Tag the reviewed commit `vX.Y.Z`.
5. Let the release workflow build static archives, checksums, and an SBOM.
6. Verify one archive and checksum independently.
7. Publish compatibility and security notes.
8. For a deployed registry change, retain the pre-upgrade checkpoint and a
   verified backup.

Release artifacts are informative until their provenance/signing policy is
separately published. Checksums detect corruption but are not by themselves a
trusted publisher identity.
