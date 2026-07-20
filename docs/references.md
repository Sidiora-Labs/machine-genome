# Normative reference baseline

The implementation profile is pinned to these documents:

- [RFC 8785 — JSON Canonicalization Scheme](https://www.rfc-editor.org/rfc/rfc8785.html),
  including verified errata, especially rejection of negative zero.
- [W3C Data Integrity EdDSA Cryptosuites v1.0](https://www.w3.org/TR/vc-di-eddsa/),
  `eddsa-jcs-2022`.
- [W3C Decentralized Identifiers v1.0](https://www.w3.org/TR/did-core/).
- [W3C Controlled Identifiers v1.0](https://www.w3.org/TR/controller-document/).
- [RFC 8032 — Edwards-Curve Digital Signature Algorithm](https://www.rfc-editor.org/rfc/rfc8032.html).
- [Multiformats multihash table](https://multiformats.io/multihash/).
- [RFC 9162 — Certificate Transparency Version 2.0](https://www.rfc-editor.org/rfc/rfc9162.html)
  as the design baseline for public append-only log proofs and consistency.
- [RFC 8174](https://www.rfc-editor.org/rfc/rfc8174.html) for BCP 14 requirement
  keywords.

Later revisions of any dependency are not silently adopted. A profile revision
and new conformance vectors are required when changed behavior affects accepted
bytes, signatures, identifiers, DID authorization, or registry proofs.
