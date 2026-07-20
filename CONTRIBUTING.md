# Contributing

Contributions are welcome for implementation correctness, interoperability,
documentation, operational safety, and protocol research.

By submitting a contribution, you agree that code and tooling are licensed
under Apache-2.0 and protocol/documentation changes are licensed under CC BY
4.0, as described in [NOTICE](NOTICE).

## Before opening a change

1. Search existing issues and protocol decisions.
2. For a cryptographic or wire-format change, open a protocol proposal first.
3. Keep normative behavior, reference implementation behavior, and operator
   policy visibly separate.
4. Do not include private keys, tokens, personal data, proprietary prompts,
   model weights, or evidence you cannot redistribute.

Small correctness fixes may go directly to a pull request. Security reports
must follow [SECURITY.md](SECURITY.md).

## Development setup

Requirements:

- Go 1.24 or newer;
- GNU Make;
- `jq`;
- Python 3 for repository link checks;
- Docker only for container validation.

```sh
make bootstrap-check
make check
make race
make build
```

`make check` is the required local gate. It validates tests, vet, module
integrity, schemas, OpenAPI syntax, conformance records, documentation links,
formatting, and repository policy files.

## Pull requests

A reviewable pull request:

- explains the user or interoperability problem;
- identifies affected trust boundaries;
- includes positive and negative tests;
- updates schemas, OpenAPI, CLI help, docs, and vectors when behavior changes;
- states compatibility and migration impact;
- avoids unrelated formatting or dependency churn;
- passes all required checks.

Cryptographic changes require a deterministic vector. Registry state changes
require restart and backup/restore coverage. Explorer changes require desktop
and mobile browser evidence.

## Protocol proposals

Open an issue using the protocol proposal template. A proposal must include:

- problem and non-goals;
- exact data model or algorithm;
- security and privacy analysis;
- canonical bytes and deterministic vectors;
- compatibility, versioning, and downgrade behavior;
- alternatives considered;
- implementation and governance impact.

Closed vocabularies and accepted-byte changes are not treated as harmless minor
edits. See [versioning](docs/versioning.md).

## Commit and release hygiene

Use focused commits with imperative summaries. Do not commit generated binaries,
private credentials, local databases, coverage output, or restored backups.
Releases are cut from a clean, reviewed commit using the tagged-release
workflow; see [docs/versioning.md](docs/versioning.md).

## Review standards

Maintainers may decline changes that weaken fail-closed behavior, collapse
evidence into an unexplained trust score, imply vendor endorsement, put mutable
or private material in public records, or represent one registry as neutral
global consensus.
