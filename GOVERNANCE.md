# Governance

Machine Genome is currently an experimental open-source project with
maintainer-led governance. It is not a standards body, certification authority,
or neutral global registry.

## Decision classes

### Implementation changes

Bug fixes, performance work, documentation, deployment assets, and compatible
API improvements are accepted through reviewed pull requests. Maintainers are
responsible for preserving security boundaries and release quality.

### Protocol changes

Changes to accepted bytes, canonicalization, proof verification, identifiers,
lineage vocabulary, amendment semantics, evidence dimensions, or registry proof
meaning require:

1. a public protocol proposal;
2. security, privacy, and compatibility analysis;
3. deterministic cross-implementation vectors;
4. an explicit profile version decision;
5. documented dissent and alternatives;
6. maintainer approval after a public review period.

Consensus is preferred. If consensus is not reached, maintainers may defer the
proposal rather than force a wire-format change.

### Security changes

Maintainers may privately develop and release a fail-closed security correction.
The public advisory must explain protocol and compatibility impact after the
coordinated disclosure window.

## Roles

- **Contributors** author issues, proposals, code, tests, docs, or reviews.
- **Maintainers** merge changes, manage releases and advisories, and protect the
  conformance boundary.
- **Registry operators** run deployments and set operational policy. Operating
  the reference registry does not grant authority over independent
  implementations.
- **Implementers and witnesses** provide interoperability evidence and may
  challenge ambiguous or operator-specific behavior.

The current maintainer set is the set of people with merge authority in the
canonical GitHub repository. Changes to that set should be announced in the
repository.

## Transparency and conflicts

Substantive decisions belong in public issues or pull requests, except during a
security embargo. Maintainers and reviewers should disclose material conflicts
of interest, including employment by an affected registry or model vendor.

No project mark may be used to imply independent certification until a separate
conformance-mark policy and neutral governance process exist.

## Toward neutral stewardship

The intended long-term direction is an independent working group with multiple
implementations, public change control, an IANA-style vocabulary registry,
intellectual-property commitments, a trademark policy, and independent
conformance stewardship. Those institutions do not yet exist and are not
simulated by this document.
