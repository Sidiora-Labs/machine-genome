# `mgs` command-line reference

The CLI supports offline protocol work and registry operation. Run
`mgs <command> -h` for the flags accepted by an installed version.

## Identity and record authoring

### `mgs keygen`

Generate an Ed25519 `did:key` controller file.

```sh
mgs keygen --out controller.key.json
```

The output contains private seed material and is created with mode `0600`.

### `mgs init-genesis`

Create an unsigned root genesis.

```sh
mgs init-genesis \
  --key controller.key.json \
  --name NAME \
  --namespace DNS_OR_DID \
  --subject-type agent \
  --version 1.0.0 \
  [--description TEXT] \
  [--timestamp RFC3339] \
  --out genesis.unsigned.json
```

The generated record has empty parentage and artifact commitments. Add those
fields before signing when needed.

### `mgs authorize-edge`

Add a parent-controller authorization proof to one zero-based lineage edge.

```sh
mgs authorize-edge \
  --in child.unsigned.json \
  --edge 0 \
  --key parent.key.json \
  [--created RFC3339] \
  --out child.authorized.json
```

Authorize every required edge before signing the child genesis.

### `mgs sign`

Sign an unsigned genesis, amendment, or attestation.

```sh
mgs sign \
  --in unsigned.json \
  --key controller.key.json \
  [--created RFC3339] \
  --out secured.json
```

By default the proof timestamp is the record timestamp.

## Digest and verification

### `mgs canonicalize`

Write the RFC 8785 representation of a JSON object:

```sh
mgs canonicalize --in record.json
```

### `mgs digest`

Canonicalize a JSON object and return its MGS SHA-256 multihash:

```sh
mgs digest --in record.json
```

### `mgs artifact-digest`

Hash raw bytes without JSON canonicalization:

```sh
mgs artifact-digest --in artifact.bin
tar -cf - source-directory | mgs artifact-digest --in -
```

Input from stdin is bounded to 1 GiB.

### `mgs gene`

Return the Gene of a secured genesis:

```sh
mgs gene --in genesis.json
```

### `mgs verify`

Verify a secured genesis, amendment, or attestation using the profile's offline
`did:key` resolver:

```sh
mgs verify --in secured.json
```

### `mgs verify-chain`

Verify genesis followed by an ordered amendment chain:

```sh
mgs verify-chain genesis.json amendment-1.json amendment-2.json
```

## Registry operation

### `mgs registry-serve`

```sh
mgs registry-serve \
  --data-dir DIR \
  --checkpoint-key checkpoint.key.json \
  --admin-token-file admin.token \
  [--listen 127.0.0.1:8080] \
  [--public-base-url https://registry.example] \
  [--backup-dir DIR] \
  [--rate-per-minute 120] \
  [--tls-cert fullchain.pem --tls-key privkey.pem]
```

The server refuses cleartext on a non-loopback address unless
`--allow-insecure-public` is explicitly supplied for a trusted same-host TLS
proxy. `--allow-unverified-namespaces` disables DNS namespace admission control
and must not be used on a public registry.

### `mgs registry-backup`

Create an exclusive, transactionally consistent offline backup:

```sh
mgs registry-backup --data-dir DIR --out registry.backup.db
```

Do not run this command against a database already opened by another process;
use the authenticated online backup endpoint for a live service.

### `mgs registry-verify`

Verify database object digests, metadata correspondence, and administrative
audit-chain integrity:

```sh
mgs registry-verify --data-dir DIR
```

### `mgs version`

Print the protocol/software version and build metadata:

```sh
mgs version
mgs version --json
```

## Exit and output behavior

Successful commands exit `0`. Usage, verification, filesystem, and operational
failures exit nonzero and prefix the final error with `mgs:`. Record-producing
commands write only the requested content to stdout, making them suitable for
pipelines. Registry request logs are structured JSON on stderr.
