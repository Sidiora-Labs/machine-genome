# Machine Genome Specification

Version: `0.2.0-implementation-profile`
Status: Experimental interoperability profile

MGS 0.2.0 extends the byte-exact cryptography, Gene derivation, proof, lineage,
amendment, attestation, and registry rules in the
[0.1.1 implementation profile](MGS-0.1.1-implementation-profile.md). It adds
machine principals that agents can use to identify and discover one another.
It does not claim standards-body approval.

## 1. Compatibility

The `mgs` value for new records is `0.2.0`. A 0.2.0 implementation MUST retain
verification support for secured 0.1.1 records. It MUST NOT rewrite or re-sign
an old genesis because doing so would create a different Gene.

The new genesis fields are covered by the controller's Data Integrity proof and
therefore by the Gene. Amendment and attestation chain semantics are unchanged.
The `powered-by` lineage edge is added for an agent-to-model runtime relation.
It follows the same authorized, observed, and unresolved states as every other
lineage edge.

## 2. Agent ID and claimed name

An `agent` or `agent-instance` genesis contains:

```json
{
  "identity": {
    "name": "cody-runtime",
    "namespace": "did:key:z...",
    "subjectType": "agent",
    "version": "1.0.0",
    "createdDate": "2026-08-09",
    "agentName": { "given": "Cody" }
  }
}
```

`agentName.given` is selected and signed by the controller. It contains 1 to 32
Unicode letters and may contain an internal apostrophe or hyphen. It is not
globally exclusive by itself.

After the secured genesis is signed, derive:

```text
Agent ID = "urn:mgs:agent:" || Gene
Agent Name = given || " " || GenomeSurname(Gene)
```

This avoids a hash/signature cycle: the Gene-derived values are not inserted
back into the genesis. Agent ID retains the full 256-bit Gene digest. A public
registry MUST index the complete Agent Name case-insensitively and reject a
second Gene claiming the same complete name.

### 2.1 Genome surname

Decode the Gene as base58btc multibase and remove the `0x12 0x20` SHA-256
multihash prefix. Let the 32 remaining bytes be `d`.

1. Select `prefixes[d[0] >> 4]`, `middles[d[1] >> 4]`, and
   `suffixes[d[2] >> 4]` from the ordered tables below.
2. Concatenate those three strings.
3. Append `-` and lowercase hexadecimal bytes `d[3:11]`.

```text
prefixes = Ar Bel Cor Dar El Fen Gal Hal Is Jun Kel Lor Mor Nor Or Per
middles  = a e i o u ae ia io or en al ir on ul er an
suffixes = den far len mar nor ran sen tor vale wen xen yor zen riel voss mere
```

The readable prefix is cosmetic. The 64-bit suffix is collision-resistant and
the registry uniqueness check fails closed; consumers use the full Agent ID for
security decisions.

## 3. Creation date

`identity.createdDate` is required in 0.2.0 and uses `YYYY-MM-DD`. It identifies
the controller-claimed creation date of the subject. `genesis.timestamp` remains
the signed record-authoring time, while registry inclusion time remains an
independent registry fact. Consumers MUST NOT conflate the three.

## 4. Capability claims

Every 0.2.0 genesis contains a `capabilities` array, which may be empty. Each
entry contains a canonical lowercase `id`, human name, optional category and
description, a level (`basic`, `working`, `advanced`, or `expert`), and optional
evidence digests.

Capability entries are controller claims, not benchmark results. Other agents
may use them for discovery and routing, but SHOULD require attestations or
artifact evidence for high-impact delegation. Later `capability-claim` or
`capability-revocation` amendments supersede initial discovery data without
mutating genesis.

## 5. Training-data declarations

Every 0.2.0 genesis contains a `trainingData` array, which may be empty. A
declaration records:

- stage: `pretraining`, `post-training`, `fine-tuning`, `reinforcement`,
  `distillation`, `runtime-learning`, or `other`;
- a name and disclosure level: `full`, `partial`, `summary`, `undisclosed`,
  `unknown`, or `not-applicable`;
- optional description, evidence URI, digest, license, and data cutoff.

An explicit `unknown` or `undisclosed` declaration is preferable to fabricated
precision. Every training-data URI MUST carry an immutable digest. Training-data
amendments can add later disclosures or disputes.
Public records MUST contain commitments and public summaries, not private
datasets, prompts, user conversations, credentials, or personal data.

## 6. Foundation-model records

A `model` genesis in 0.2.0 contains a model profile with primary `provider`,
optional provider list, `modelId`, optional `family`, and the boolean
classification claim `foundation`. It may also record a catalog-reported
`contextWindow` token limit.
Foundation records are normal signed Genes, suitable as parents of agent
records through `powered-by` or the more specific existing lineage edges.

A registry or community observer may seed public model information only under
its own controller namespace. Such a record MUST be described as an observed
third-party claim unless the vendor controller signed it. Catalog inclusion,
provider naming, or registry admission does not establish vendor endorsement,
ownership, capability, safety, or truth of training-data claims.

## 7. Required verification behavior

In addition to the 0.1.1 requirements, a conforming 0.2.0 implementation:

- requires `createdDate`, `capabilities`, and `trainingData` in new genesis;
- requires `agentName.given` for `agent` and `agent-instance` subjects;
- requires a model profile for `model` subjects and rejects it elsewhere;
- validates capability identifiers, levels, evidence digests, training stages,
  disclosure values, cutoff dates, and optional training digests;
- derives Agent ID and Agent Name from the verified secured Gene;
- surfaces agent names, Agent IDs, capabilities, model coordinates, and
  training disclosures without presenting self-claims as certification.
