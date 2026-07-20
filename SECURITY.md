# Security policy

## Supported versions

Security fixes are applied to the latest tagged `0.1.x` reference release and
the `main` branch. The protocol is experimental; compatibility-preserving fixes
are preferred, but a critical fail-closed correction may reject inputs accepted
by an earlier implementation.

| Version | Security support |
| --- | --- |
| Latest `0.1.x` | Supported |
| Older pre-release snapshots | Not supported |

## Report a vulnerability

Do not open a public issue for suspected vulnerabilities.

Use the repository's **Security → Report a vulnerability** flow to create a
private GitHub Security Advisory:

<https://github.com/paxlabs-inc/machine-genome/security/advisories/new>

Include:

- the affected version or commit;
- the component and trust boundary;
- reproduction steps or a minimal test vector;
- realistic impact;
- whether keys, private records, registry availability, or transparency
  consistency may be affected;
- any suggested mitigation or embargo constraints.

Never send real private keys, administrator tokens, private evidence, or
personal data. Use newly generated test material.

Maintainers will acknowledge a complete report within seven calendar days,
triage severity and scope, coordinate a fix and disclosure window, and credit
the reporter unless anonymity is requested. This target is not a paid bounty or
a warranty.

## High-priority classes

- signature acceptance without controller authorization;
- canonicalization ambiguity, duplicate-key acceptance, or digest mismatch;
- lineage authorization replay or child-commitment confusion;
- amendment-chain reordering or controller-rotation bypass;
- transparency proof or checkpoint verification errors;
- registry admission that bypasses proof, lineage, or namespace validation;
- secret disclosure, path traversal, SSRF, request smuggling, or authentication
  bypass;
- backup, tombstone, or audit-chain behavior that silently rewrites history.

## Disclosure

For a confirmed issue, the project will publish an advisory describing affected
versions, impact, remediation, and protocol consequences. If accepted bytes or
cryptographic meaning change, the release will include new conformance vectors
and a compatibility note.
