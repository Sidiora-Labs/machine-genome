# MGS 0.1.0-draft protocol audit

Status: resolved in the `0.1.1-implementation-profile` unless marked deferred.

## Cryptographic correctness

1. **Lineage authorization was circular.** The draft asks a parent to sign
   `childGenesisHashPreimage`, but the parent's signature is itself part of that
   preimage. The profile defines a `childCommitment` over the unsigned genesis
   after removing the top-level proof and every `authorizationProof`. The parent
   signs a deterministic `lineage-authorization` document containing that
   commitment, the parent Gene, edge type, and child controller.

2. **`eddsa-jcs-2022` is not direct `Ed25519(JCS(document))`.** The W3C suite
   signs `SHA-256(JCS(proof configuration)) ||
   SHA-256(JCS(unsecured document))`. The implementation follows that algorithm
   and requires `proofPurpose: "assertionMethod"`.

3. **Genesis proof inclusion is valid but needed an explicit two-phase rule.**
   The controller first signs the unsecured genesis. The Gene is then derived
   from the complete secured genesis, including `proof`. There is no hash/sign
   cycle.

4. **JCS input needs strict parsing.** Generic JSON decoders often accept
   duplicate member names, and negative zero canonicalizes to positive zero.
   The reference implementation rejects duplicates, invalid UTF-8, trailing
   values, non-I-JSON numbers, and all lexical forms of negative zero before
   canonicalization.

5. **Hash encoding was underspecified in examples.** MGS SHA-256 values are
   exactly `multibase(base58btc(multihash(sha2-256, digest)))`; the multihash
   prefix bytes are `0x12 0x20`.

## Log and authority model

6. **Third-party failure reports conflicted with the controller chain.** A
   controller-only chain cannot also accept arbitrary outside records without
   races, forged sequence ownership, and unavoidable forks. The profile has:

   - one ordered, controller-signed amendment chain per Gene; and
   - independent, content-addressed attestations, each signed and chained by its
     own issuer.

   A registry MUST accept conforming third-party attestations but never presents
   them as controller-authored amendments.

7. **A multi-registry ecosystem cannot promise that forks never occur.** A
   registry rejects local conflicting sequence numbers. Ecosystem forks are
   equivocation evidence, surfaced rather than hidden. Public registries need
   Merkle inclusion proofs, signed checkpoints, gossip, and witnesses.

8. **Key rotation cannot be ordered by author timestamps.** Client clocks can be
   wrong or malicious. Rotations become effective by chain sequence; registries
   additionally expose independent inclusion time. Emergency recovery needs a
   precommitted recovery policy and is deferred from the core CLI.

## Semantics and governance

9. **Attestation “tiers” are not a total order.** Hardware binding does not imply
   reproducibility or independent review. The profile replaces numeric minimum
   comparisons with orthogonal evidence dimensions: authorship, review,
   reproducibility, and runtime binding. Legacy tier labels may be displayed but
   MUST NOT be compared numerically.

10. **A Gene is stable for a genesis, not a subject's entire lifetime.** A DID
    may legitimately control several Genes, including superseding versions.

11. **Minor-version edge additions are not backward compatible when unknown
    edges must be rejected.** Closed lineage vocabulary changes require a major
    version or an explicitly negotiated extension profile.

12. **Artifact subjects do not always control keys.** `identity.did` identifies
    the subject when one exists; controller identity remains separate. Immutable
    artifacts may additionally use content identifiers.

13. **Mutable evidence URIs are insufficient.** Every externally fetched
    artifact or evidence item must carry an immutable digest.

14. **Presumptive failure inheritance is a consumer policy, not a fact written
    into the protocol.** Resolver output must label it as derived and identify
    the exact policy/version used.

15. **Permanent public preimages conflict with privacy and legal obligations.**
    Public records should contain commitments, not secrets or unnecessary
    personal data. Registries may stop serving unlawful payloads while retaining
    the hash, inclusion proof, and a transparent tombstone. This is availability
    redaction, not cryptographic history rewriting.

## Deferred before a production registry

- DID method adapters beyond offline `did:key`, with SSRF-safe `did:web`
  resolution and a versioned `did:pax` adapter.
- Threshold controllers, recovery keys, key revocation, and historical DID
  resolution.
- Namespace challenge protocol and registrar authorization.
- Transparency-log API, checkpoints, witnesses, cross-registry reconciliation,
  anti-spam policy, privacy review, and abuse appeals.
- A governance process, IANA-style registries, trademark policy, conformance
  suite stewardship, and transfer to a neutral standards body.
