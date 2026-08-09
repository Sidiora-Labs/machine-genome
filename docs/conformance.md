# Conformance suite

Conformance is byte-exact. Passing structural JSON Schema validation alone does
not establish MGS compatibility.

## Required behavior

An `0.2.0` implementation must agree on all 0.1.1 cryptographic behavior plus:

- Agent ID and genome-surname derivation;
- agent-name validation and registry uniqueness;
- capability and training-data vocabulary validation;
- foundation model profiles and `powered-by` lineage acceptance.

Both supported profiles must agree on:

- valid and invalid I-JSON inputs;
- RFC 8785 canonical bytes;
- SHA-256 multihash and Gene encoding;
- W3C `eddsa-jcs-2022` proof construction;
- controller and verification-method authorization;
- child commitment and lineage authorization;
- amendment sequence, rotation, and terminal state;
- attestation issuer separation;
- Merkle leaf/node domain separation and proof verification.

## Repository gates

`make check` performs:

1. unit, integration, adversarial, and seeded fuzz tests;
2. Go vet and module checksum verification;
3. JSON Schema and OpenAPI syntax checks;
4. verification of every published signed record and expected Gene;
5. documentation-link and repository-policy checks;
6. formatting and whitespace checks.

`make race` repeats all Go tests with the race detector. `make coverage` emits a
package and total coverage report for review; coverage percentage is evidence,
not a substitute for boundary-focused vectors.

## Machine-readable vectors

Vectors live under `testdata/vectors/` and are versioned with the profile:

- `jcs-rfc8785.json` — canonicalization input and exact output;
- `mgs-gene-v0.1.1.json` — deterministic key seed, genesis fields, timestamp,
  and expected Gene;
- `mgs-agent-v0.2.0.json` — deterministic agent genesis, expected Gene, Agent
  ID, genome family name, and complete claimed name;
- published signed records under `records/neo/` — realistic secured genesis and
  lineage examples.

Vector files never contain production private keys. Deterministic seeds are
test-only and must never control a real subject.

## Negative tests

The suite rejects duplicate keys, negative-zero lexical forms, unpaired
surrogates, trailing JSON, invalid UTF-8, proof tampering, authorization replay,
chain gaps, competing sequences, expired/consumed DNS challenges, oversized
bodies, wrong media types, unauthorized administration, invalid Merkle paths,
and database/audit inconsistencies.

## Claiming conformance

Until an independent implementer report and conformance-mark policy exist,
projects should say:

> Implements the experimental MGS 0.2.0 profile, verifies immutable 0.1.1
> records, and passes the published Machine Genome reference vectors as of
> commit `<commit>`.

They should not claim standards-body certification or global registry
consensus.

## Adding a vector

A protocol-affecting pull request must include:

- human-readable purpose and threat addressed;
- machine-readable input;
- exact expected bytes, digest, proof, or failure;
- independent derivation where practical;
- a test that loads the vector file;
- versioning and compatibility analysis.
