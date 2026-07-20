# “Sign up as an agent” protocol sketch

Status: design input for a companion specification, not part of the MGS
cryptographic core.

The useful third signup option is not “pretend the agent is a human” and it is
not “give every website one privileged vendor API.” It is a service-recognized
machine principal with constrained authority, an auditable owner relationship,
and an ordinary browser-compatible confirmation path.

## Roles

- **Agent subject:** identified by a DID and an MGS Gene.
- **Controller:** keys authorized to act for the agent.
- **Account sponsor:** optional human or organization that accepts legal and
  billing responsibility without becoming the agent's cryptographic identity.
- **Service:** the site creating the account.
- **Agent mailbox:** a dedicated address or inbox endpoint for confirmation
  links, one-time codes, receipts, notices, and recovery events.

The mailbox is a communications endpoint, not the root identity. Losing email
must not silently transfer the DID.

## Browser-compatible registration ceremony

1. A site displays **Sign up as an agent** and generates a single-use challenge:

   ```json
   {
     "type": "AgentAccountChallenge",
     "serviceOrigin": "https://service.example",
     "nonce": "base64url-random-32-bytes",
     "expires": "2026-07-20T14:05:00Z",
     "termsHash": "z...",
     "requestedCapabilities": ["account.create", "email.receive"],
     "callback": "https://service.example/agent-signup/complete"
   }
   ```

2. The agent reviews the origin, terms commitment, requested capabilities, and
   sponsor requirements in its browser session.
3. The agent submits its DID, Gene, mailbox address, and a Data Integrity proof
   over the challenge. Optional sponsor credentials are separate and explicitly
   scoped.
4. The service resolves and verifies the DID, Gene, controller state, challenge
   origin, nonce, expiry, and terms hash. It does not infer trust from a display
   name.
5. The service sends a confirmation link or code to the dedicated agent
   mailbox. The agent may read it through a browser UI, IMAP, JMAP, or a mailbox
   provider adapter. No one transport is mandatory.
6. The agent completes the callback in the same controlled browser session and
   signs a final account-binding statement containing the service account ID.
7. Both sides retain receipts. Later key rotation follows MGS/DID controller
   state; mailbox changes require a fresh signed ceremony.

## Browser autonomy

An agent should be able to complete the ceremony through the same browser flows
available to people: navigation, form filling, cookie/session handling,
confirmation links, file downloads, and consent screens. APIs and MCP-style
connectors are accelerators, not identity prerequisites.

Browser execution still requires:

- origin and redirect allowlists;
- clear distinction between reading and submitting;
- explicit policy for purchases, legal acceptance, publishing, and account
  recovery;
- secret isolation from page content;
- phishing-resistant challenge binding;
- durable receipts and screenshots or DOM evidence for important actions;
- a human escalation path when CAPTCHA or service policy forbids automation.

The protocol must not encourage bypassing a service's access controls or
misrepresenting an agent as a person.

## Mailbox requirements

A dedicated agent mailbox service should expose:

- stable addresses scoped to an agent DID;
- encrypted storage and narrowly scoped delegated access;
- parsed links/codes plus the original message;
- sender authentication results and phishing warnings;
- event receipts suitable for an audit trail;
- revocation, rotation, retention, and sponsor recovery policies;
- rate limits and abuse reporting;
- browser UI access so the agent is not dependent on a proprietary API.

Mailbox providers can publish an attestation binding an address to a DID without
publishing private message content.

## Privacy and unlinkability

Using one global DID everywhere creates a perfect tracking identifier. Agents
SHOULD use pairwise service DIDs and prove their relationship to a public MGS
Gene only when necessary. Services should request the minimum attributes and
must not require public disclosure of private prompts, memory, email, or owner
identity merely to recognize a machine principal.

## Separation from MGS

MGS answers “what subject and lineage does this controller claim?” Agent Account
answers “what authority is this machine principal requesting from this service?”
The protocols should compose, but neither should be embedded inside the other.
