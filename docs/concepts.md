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

## Agent identity and name claims

MGS 0.2.0 derives the collision-safe Agent ID
`urn:mgs:agent:<Gene>` from the secured genesis. The controller signs a chosen
given name, and the protocol appends a deterministic family name derived from
the Gene digest. A registry uniquely claims the complete name. The readable
name is for discovery; security-sensitive references use the full Agent ID.

## Capabilities and training data

Capabilities are signed, structured self-claims that other agents can search
when routing work. A level is useful discovery metadata, not a universal score
or proof of competence. Evidence digests and independent attestations provide
stronger support.

Training-data declarations record stage, disclosure level, cutoff, license, and
optional evidence. `unknown` and `undisclosed` are valid explicit states. Public
records should commit to evidence, never embed private datasets, prompts,
memories, or user transcripts.

Model subjects include a provider/model coordinate and may declare themselves
as foundation models. Community-observed model Genes are not vendor-authorized
unless the vendor controller signs them.

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
| Derived Agent ID | ID is bound to this secured Gene | Controller is a human or legal person |
| Capability claim | Controller advertised this strength | Agent can perform it safely or reliably |
| Training disclosure | Controller signed this disclosure | Dataset list is complete or lawful |
| Authorized edge | Parent controller approved this exact relationship | Child inherited capability |
| Observed edge | Child/operator asserted the relationship | Parent vendor endorsed it |
| Artifact digest | Retrieved bytes match the commitment | Artifact is safe or lawful |
| Registry inclusion proof | Digest was included in that tree | Every observer saw one view |
| Attestation | Issuer signed the statement | Statement is automatically trusted |
