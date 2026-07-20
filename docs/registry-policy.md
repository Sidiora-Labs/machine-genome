# Public registry policy

Status: baseline policy for the reference registry at
[machinegenome.org](https://machinegenome.org). It is an operational policy,
not part of the MGS cryptographic profile and not legal advice.

## Purpose

The registry accepts verifiable Machine Genome records and exposes transparent
identity, lineage, evidence, and inclusion data. Admission confirms structural,
cryptographic, lineage, chain, and namespace rules. It does not certify the
truth, quality, safety, ownership, or legality of a claim.

## Appropriate content

Public records should contain:

- machine identity and controller identifiers;
- typed lineage references;
- immutable digests of relevant artifacts;
- public, stable evidence URLs;
- narrowly scoped provenance descriptions;
- amendments and attestations relevant to interpreting a Gene.

Do not submit:

- private keys, access tokens, passwords, recovery material, or credentials;
- personal data, private prompts, memories, mailbox content, or user
  transcripts;
- model weights, source archives, or datasets when a commitment is sufficient;
- malware, exploit payloads, or illegal material;
- mutable evidence presented without an immutable digest;
- content submitted primarily for harassment, impersonation, or resource
  exhaustion.

Registry data is public and content-addressed. A later tombstone cannot retract
copies already observed or replicated.

## Namespace and identity

DID namespaces must equal the genesis controller DID. DNS namespaces require an
expiring controller challenge. Namespace control does not establish trademark,
copyright, organizational, or vendor authority.

The registry may reject a duplicate identity coordinate while preserving valid
distinct versions and Genes.

## Neutrality and evidence

Unauthorized but structurally valid lineage is admitted and clearly labeled as
observed. Third-party attestations remain separate from controller history.
The operator does not silently turn an observed claim into a vendor-authorized
one or rank evidence through an unexplained universal score.

## Availability action

The operator may stop serving payload bytes where necessary for law, safety,
privacy, abuse response, or infrastructure protection. The action uses a
reasoned tombstone and retains the digest, inclusion position, and transparency
evidence. Cryptographic history is not represented as deleted.

Confidential case details remain in an external case system rather than the
public tombstone.

## Reports and appeals

Security vulnerabilities follow [SECURITY.md](../SECURITY.md). Other content or
identity disputes should identify the exact record digest, reason, supporting
public evidence, desired action, and a non-confidential case reference.

Until a dedicated appeals body exists, operator decisions are reviewable by the
repository maintainers. This is not neutral adjudication; the limitation should
be considered when choosing whether to rely on this registry.

## Service limits

Requests, bodies, headers, namespace challenges, and search responses are
bounded. Automated access must respect published rate limits and must not evade
controls. The service is provided without a guaranteed SLA during the
experimental phase.

Independent registries may apply different lawful content-neutral policies
while preserving protocol verification and transparency semantics.
