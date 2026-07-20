# Registry HTTP API

Status: production profile for the single-node reference registry.

The API returns JSON except for canonical MGS records, the HTML explorer, and
Prometheus text metrics. Errors use an RFC 9457-style problem object with a
stable `code`. Request bodies are bounded and unknown JSON members are rejected
on control endpoints.

## Public endpoints

| Method | Path | Purpose |
| --- | --- | --- |
| `GET` | `/healthz/live` | Process liveness |
| `GET` | `/healthz/ready` | Storage readiness and object counts |
| `GET` | `/metrics` | Low-cardinality Prometheus metrics |
| `POST` | `/v1/records` | Verify and submit one MGS record |
| `GET` | `/v1/records/{digest}` | Return exact canonical bytes or a tombstone |
| `GET` | `/v1/genes/{gene}` | Genesis, chain, evidence, children, and conflicts |
| `GET` | `/v1/search?q=...&limit=...` | Search registered genesis metadata |
| `POST` | `/v1/namespace-challenges` | Create a DNS controller challenge |
| `POST` | `/v1/namespace-challenges/{id}/verify` | Verify and consume a challenge |
| `GET` | `/v1/log/checkpoint` | Return the current signed Merkle checkpoint |
| `GET` | `/v1/log/inclusion/{index}?treeSize=N` | Return an inclusion proof |
| `GET` | `/v1/log/consistency?firstSize=A&secondSize=B` | Return a consistency proof |
| `GET` | `/` and `/genes/{gene}` | Plain-language explorer |

Record submission uses `Content-Type: application/mgs+json` and is limited to
1 MiB. A new record returns `201`; byte-identical resubmission returns `200`.
Invalid proof, lineage, chain, subject, or namespace state returns a structured
4xx response. A locally competing amendment is not admitted to the canonical
chain: it returns `409` and is retained separately as equivocation evidence.

The registry returns author time from the signed record separately from its own
`inclusionTime` and zero-based `logIndex`.

## Namespace challenge

Create:

```json
{
  "namespace": "agents.example",
  "controllerDid": "did:key:z..."
}
```

The response identifies a `_mgs-challenge.agents.example` TXT name and an exact
`mgs-registry=...` value. Publish it, then submit the token portion:

```json
{"token":"base64url-value"}
```

Challenges expire after 15 minutes, can be consumed once, and grant that exact
controller DID access to that namespace for 30 days. Grants do not let the
caller forge a genesis: normal MGS controller proof verification still applies.

## Administrative endpoints

Administrative calls require `Authorization: Bearer TOKEN`. The token is read
from a mode-`0600` file at process start and compared by hash in constant time.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/v1/admin/tombstones/{digest}` | Stop serving a payload while retaining its commitment |
| `GET` | `/v1/admin/audit?limit=N` | Inspect recent hash-chained audit events |
| `POST` | `/v1/admin/backups` | Create a transactionally consistent online backup |

A tombstone body contains a mandatory plain-language `reason` and optional
external `reference`. Tombstoning never removes the object from the database,
renumbers the transparency log, or invalidates an inclusion proof.

## Transparency verification

Leaves are `SHA-256(0x00 || UTF8(contentDigest))`; internal nodes are
`SHA-256(0x01 || left || right)`, matching the RFC 9162 domain-separation
construction. Proof hashes use unpadded base64url. The Go package exports
`registry.VerifyInclusion`, `registry.VerifyConsistency`, and
`registry.VerifyCheckpoint` for offline verification.

Checkpoints are JCS objects signed by the configured Ed25519 checkpoint key.
They are suitable input to independent polling, witness, and gossip services.
This repository does not yet operate those independent parties.
