# Conformance test data

Files under `vectors/` are public, deterministic interoperability inputs. They
contain no production secrets.

- `jcs-rfc8785.json` pins the RFC 8785 canonicalization example used by the Go
  suite.
- `mgs-gene-v0.1.1.json` pins the deterministic genesis parameters and expected
  Gene for MGS 0.1.1.

Implementers should consume the JSON files directly rather than transcribing
values from documentation. A vector change that alters expected bytes or
digests requires protocol compatibility review.
