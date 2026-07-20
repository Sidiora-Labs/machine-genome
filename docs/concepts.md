# Concepts and trust semantics

Machine Genome separates cryptographic facts from conclusions a consumer may
choose to draw from them.

## Gene

A **Gene** is the multibase/base58btc encoding of a SHA-256 multihash over the
RFC 8785 canonical bytes of a complete, signed genesis record:

```text
"z" || base58btc(0x12 || 0x20 || SHA-256(JCS(secured genesis)))
```

Changing any identity, lineage, evidence commitment, controller, timestamp, or
proof byte produces a different Gene. A Gene identifies one genesis, not every
version of a subject forever.

## Genesis

A genesis record declares:

- display identity and subject type;
- a namespace and optional subject DID;
- typed parent relationships;
- the controller DID and genesis verification method;
- immutable artifact commitments;
- the controller's Data Integrity proof.

The proof establishes controller authorship of the declaration. It does not
make every assertion in the declaration objectively true.

## Lineage edge

A lineage edge is one of three states:

- **authorized** — the parent controller signed the deterministic child
  commitment for that edge;
- **observed** — the child controller or registry operator records the
  relationship without a parent authorization;
- **unresolved** — a historical or external parent is named, but no registry
  Gene is available.

Consumers must preserve this distinction. In particular, an observed model
relationship is not vendor endorsement.

## Amendment

An amendment is part of an ordered, controller-authored chain rooted at a Gene.
Sequence and previous-record digest establish order. Controller rotation becomes
effective by chain position, not by an untrusted author clock.

Amendments do not mutate genesis. They record claims such as controller
rotation, capability statements, supersession, mitigation, or dispute.

## Attestation

An attestation is an independently signed statement about a Gene. It belongs to
the issuer's evidence stream, never to the subject controller's amendment
chain. Evidence quality is represented by independent dimensions rather than a
single misleading trust score.

## Registry and transparency log

A registry verifies records, preserves canonical bytes, assigns an independent
inclusion time and log index, and appends each accepted digest to a Merkle tree.
Signed checkpoints commit to the current tree size and root. Inclusion proofs
show that a record was present; consistency proofs show append-only growth
between two tree sizes.

A single registry remains a single operator. Independent witnesses and
checkpoint gossip are needed to detect split views.

## Tombstone

A tombstone stops public delivery of a payload for a documented operational or
legal reason. It does not remove the digest or rewrite transparency history.
The commitment and inclusion proof remain verifiable.

## What consumers may conclude

| Evidence | Supported conclusion | Unsupported conclusion |
| --- | --- | --- |
| Valid genesis proof | Controller signed these exact claims | Claims are objectively true |
| Authorized edge | Parent controller approved this exact relationship | Child inherited capability |
| Observed edge | Child/operator asserted the relationship | Parent vendor endorsed it |
| Artifact digest | Retrieved bytes match the commitment | Artifact is safe or lawful |
| Registry inclusion proof | Digest was included in that tree | Every observer saw one view |
| Attestation | Issuer signed the statement | Statement is automatically trusted |
