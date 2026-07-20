# Threat model

Status: security design baseline for the `0.1.1` reference implementation.
This is not an independent audit.

## Assets

- controller and checkpoint private keys;
- administrator token;
- canonical record bytes and their availability state;
- identity, lineage, amendment, and attestation indexes;
- transparency log order and signed roots;
- namespace grants;
- administrative audit history;
- backup confidentiality and integrity;
- operator and user confidence in what a proof actually establishes.

## Trust boundaries

1. Untrusted record bytes enter the CLI or HTTP API.
2. DID and parent resolution supply authorization context.
3. DNS supplies namespace-control evidence.
4. The verified admission boundary writes durable state.
5. The registry checkpoint signer commits to one log view.
6. nginx or another trusted proxy supplies TLS and public client rate limits.
7. Operators access private credentials, backups, and administrative endpoints.

## Adversaries

The design assumes attackers may:

- submit malformed, ambiguous, oversized, replayed, or adversarial JSON;
- own valid keys and make deceptive claims;
- reuse an authorization proof against a changed child;
- race or fork amendment chains;
- issue low-quality or hostile attestations;
- control a DNS namespace temporarily;
- scrape public records and correlate stable identifiers;
- exhaust CPU, memory, disk, file descriptors, or rate-limit state;
- observe or tamper with traffic where TLS is absent;
- compromise an operator token, proxy, registry host, DNS account, or signing
  key;
- operate a registry that presents inconsistent views to different clients.

## Security properties

### Canonical bytes and identifiers

The parser rejects duplicate members, invalid UTF-8, trailing values, negative
zero, non-I-JSON numbers, and non-object top-level values. Equivalent valid JSON
has one RFC 8785 representation. Genes and record digests use a fixed SHA-256
multihash encoding.

### Authorization

Data Integrity verification binds proof configuration and unsecured document
hashes. Verification methods must be authorized for the expected controller.
Lineage authorization signs a deterministic child commitment that excludes all
embedded authorization and top-level proof fields, preventing a signature
cycle while binding every substantive child claim.

### Chain integrity

Amendments are ordered by sequence and previous digest. Rotation becomes active
by verified chain position. Competing valid branches are evidence, not input to
an arbitrary last-write-wins rule.

### Registry integrity

Every public write passes the same verified store boundary. Object, metadata,
indexes, log position, and audit event are transactional. Administrative token
comparison is constant time. Secrets and database files require restrictive
filesystem modes.

### Transparency

Merkle proofs and signed checkpoints make omission or rewriting detectable to
an observer that retains checkpoints. Tombstones do not remove commitments.

## Abuse cases and mitigations

| Abuse | Mitigation | Residual risk |
| --- | --- | --- |
| Duplicate-key parser differential | Strict pre-parse before JCS | Unknown dependency defects |
| Signature replay on altered lineage | Deterministic child commitment | Compromised parent key |
| Fake vendor endorsement | Explicit observed/authorized state | Users may ignore labels |
| Namespace squatting | DID equality or expiring DNS challenge | DNS/account compromise |
| Amendment overwrite | Sequence/prev validation and conflict evidence | Cross-registry forks |
| Malicious attestation | Independent issuer identity and evidence dimensions | Consumers choose trust |
| Payload deletion disguised as history rewrite | Transparent tombstone | Operator can deny availability |
| Request exhaustion | Body/header/time limits and proxy/backend rate limits | Distributed volumetric attack |
| Forwarded-IP spoofing | App ignores headers; proxy trusts only known networks | Stale proxy network list |
| Token timing oracle | Hash then constant-time compare | Token theft elsewhere |
| Backup disclosure | Private directory/mode and operator procedure | Off-host handling errors |
| Split-view registry | Verifiable checkpoints and consistency proofs | No witness/gossip network yet |

## Privacy

Registry data is intentionally public and content-addressed. Redaction cannot
make already replicated bytes disappear. Records should contain immutable
commitments rather than personal data, prompts, memory, mailbox content,
credentials, or proprietary artifacts. Stable DIDs and Genes enable
correlation; pairwise identifiers belong in companion account protocols.

## Non-goals

This release does not:

- prove that provenance claims are truthful;
- prove model behavior, safety, consciousness, ownership, or legal identity;
- protect a host after root compromise;
- recover lost controller keys;
- provide threshold control or hardware key custody;
- prevent an isolated registry from serving a split view;
- moderate external artifacts;
- make DNS a permanent source of historical controller state;
- provide anonymity or unlinkability.

## Known gaps

Independent cryptographic and privacy audits, fuzzing at scale, witness/gossip
deployment, federation, historical DID resolution, key recovery, threshold
controllers, and formal availability objectives remain open work. See
[status](status.md) and [roadmap](roadmap.md).

## Review checklist

Every security-sensitive change should answer:

1. What untrusted bytes or identities cross a new boundary?
2. What exact bytes are signed or hashed?
3. Does failure reject, retry, or silently continue?
4. Can state be partially written or reordered?
5. Can an old proof be replayed under changed semantics?
6. Does the change disclose stable or private data?
7. What deterministic negative vector proves the boundary?
8. Does the public explanation overstate what was established?
