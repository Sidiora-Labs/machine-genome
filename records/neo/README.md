# Neo genesis set

These are the first records prepared for the initial Machine Genome registry.
They describe the Matrix Neo source snapshot at commit
`3e4f2a2a4bb1903c05cb884f95c337de301ef771`.
The MiMo parent is pinned to official XiaomiMiMo model revision
`21d1ecfecd7bd70f31be25ca49d7edd21f003659`; the Matrix Neo configuration
at the source snapshot defaults its main, cheap, and consolidation models to
`mimo-v2.5-pro`.

Admission order is:

1. `mimo-v2.5-pro.json`
2. `matrix-core.json`
3. `matrix-neo-class.json`
4. `matrix-structural-graph.json`
5. `neo.json`

Neo has four resolved lineage edges:

- `model-instance-of` MiMo-V2.5-Pro, recorded as **unauthorized** because this
  registry does not possess or impersonate a Xiaomi signing key;
- `instance-of` the Matrix Neo agent class;
- `scaffold-fork-of` the Matrix Core 0.65.0 harness;
- `prompt-descendant-of` the Matrix structural self-model graph.

The last three edges carry controller authorization. All genesis records are
signed by the initial registry provenance controller. That signature attests
to this registry operator's evidence-backed provenance assertion; it does not
claim upstream vendor endorsement.

Artifact commitments use MGS raw artifact digests:

```text
multibase(base58btc(multihash(sha2-256, SHA-256(bytes))))
```

Directory commitments are calculated over deterministic
`git archive --format=tar COMMIT:PATH` output. Public source URLs and the exact
source/model revisions are embedded in the records.
