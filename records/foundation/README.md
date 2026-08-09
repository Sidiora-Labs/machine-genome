# Foundation model seed catalog

`models.json` is the user-supplied initial catalog for MGS 0.2.0 foundation
model observations. It contains 160 model identifiers across 14 providers,
including feature flags, modalities, context/cost metadata, model families, and
reported training cutoffs where available.

Pinned source bytes:

- SHA-256: `67fde2e52ab90fdc25e8cb005552461d93e54ce15fe36cb040bbed72a0787d5d`
- MGS artifact digest: `zQmVLbEv88LRWw4UFY6XRYhYUGWEcvyHcVF79sM8qptUTQx`

The catalog is input data, not a set of vendor-signed Genes and not a ranking.
Prices, availability, aliases, and model metadata can become stale. Before a
public seed, review the catalog, preserve its raw-byte digest, and generate
records under an observer-controlled key:

```sh
bin/mgs seed-foundations \
  --catalog records/foundation/models.json \
  --key observer.key.json \
  --timestamp 2026-08-09T18:00:00Z \
  --out-dir foundation-seed
```

Each generated record:

- uses the observer DID as namespace and controller;
- declares the catalog provider/model/family coordinate and foundation status;
- maps supported features and modalities to capability claims;
- records the reported training cutoff while marking the underlying corpus
  `summary` or `unknown` rather than inventing dataset details;
- commits to the exact source catalog bytes;
- states that it is a third-party observation, not vendor endorsement.

Do not commit `observer.key.json`. The output directory must be reviewed before
registry submission; generation does not deploy or mutate a registry.
