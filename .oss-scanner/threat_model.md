# klever-go threat model

klever-go is the Klever blockchain node: consensus, the P2P network, transaction processing, state, and the Klever
Virtual Machine (KVM), which executes untrusted WebAssembly smart contracts. The authoritative policy is
[SECURITY.md](../SECURITY.md); this file is its short form for the scanner. Where they differ, SECURITY.md governs.

## Who is an attacker

A vulnerability needs **someone other than the affected party who can trigger the impact and gains something**
(funds, access, disruption, non-public information). A defect that only the account holder or the node operator can
cause against themselves is an ordinary bug: report it as such, do not grade it.

## Untrusted input (in scope)

- **P2P network**: blocks, headers, transactions, consensus messages, peer-discovery and sync messages from any peer
  (`network/`, `node/`, consensus packages). Remote crash, memory/CPU exhaustion, state divergence, or consensus
  safety/liveness failure triggered by a peer is in scope.
- **Transactions**: every node accepts signed transactions by design, so submission itself is not a finding. In scope
  is anything that lets a transaction exceed what its signer authorised, bypass fee/gas/bandwidth accounting, create or
  destroy value, or corrupt state (`data/`, contract and asset processing, accounts).
- **Smart contracts**: attacker-supplied WASM executing in the KVM (`kvm/`, `vmcommon/`, `kapps/`). In scope: sandbox
  escape, host-hook memory-safety bugs, gas under-metering, non-determinism between nodes, and anything that crashes
  or stalls a node while it executes a contract.
- **Cryptography** (`crypto/`): signature verification, multisig and BLS aggregation, hashing, key handling.
- **REST/WebSocket API** (`/subscribe`, `/transaction/*`, node routes): in scope for an unauthenticated or
  low-privilege remote caller reaching more than the route is meant to expose, or exhausting resources.
  Findings that pass the attacker test and demonstrate impact with the API bound beyond its `localhost:8080`
  default remain in scope, with the one-level reachability downgrade. Routes that change node state or operator
  configuration stay in scope whatever their reachability. The lack of authentication on `/transaction/broadcast`
  and `/transaction/send` is not a finding; the signature is the authorisation. Hardening is documented in
  `docs/node-api-hardening.md`.
- **Storage and genesis** (`storage/`, `genesis/`): parsing of untrusted blobs fetched from peers or snapshots.

## Not in scope (do not report)

- REST API exposure alone, without demonstrated impact that passes the attacker test.
- An operator acting on their own node (their config, keys, database, routes).
- An attacker who already has code execution, filesystem write access, or an account on the node host, unless
  the finding gives them more than that access already does.
- Operator misconfiguration or key-file mismanagement.
- Counters and telemetry on diagnostic routes that identify no peer or user, carry no key material, and reveal no
  unpublished chain state.
- Test code, fixtures, mocks, and `integrationTest/`.
- Credential-hashing strength where the shipped default fails closed and the credential file already sits alongside
  material of equal or greater sensitivity.
- Dependency CVEs with no demonstrated path through klever-go.
- Centralization, governance, and economic-design concerns that do not stem from a code defect.
- Social engineering, phishing, and physical attacks.
- Denial of service against a node you control, and anything not reproduced.

## Reproducer requirement

A finding without a working reproducer is not accepted. The best form is a **Go test in this repository that fails on
the current commit**, with the exact `go test` command and its verbatim output. Native code needs
`LD_LIBRARY_PATH=/src/kvm/wasmer2`, which the image sets. Most state behaviour is **fork-gated** through `EnableEpochs`
(`config/enableEpochs.go`): state which flags the reproducer sets, and prefer the latest fork configuration. A bug that
only exists with a fork flag disabled is not live on mainnet unless that fork has not activated.

Denial-of-service claims must be reproduced against a local or private network and describe the topology.

## Severity

Grade by demonstrated impact, then apply downgrades (non-default config, a privileged actor whose role already
confers comparable power, local or non-default reachability), each one level.
Do not apply a downgrade again if the matched impact already incorporates it; cite the rule as reasoning without
decrementing twice for the same fact.

- **Critical**: systemic impact (the chain, every holder, or a node's keys), or impact that pays the attacker and
  therefore scales.
- **High**: severe harm bounded to identifiable victims or to availability.
- **Medium**: harm bounded to a single account or node, or non-public data exposed on a remotely reachable endpoint
  under the shipped configuration, or harm reachable only under an asset configuration its issuer chose (the
  non-default-config downgrade is already priced into that route).
- **Low**: impact that reaches no further than a local or non-default surface.

A finding reduced below Low is an ordinary bug: report it as such, do not grade it.

Where an impact matches more than one level, the highest applies. We publish no per-level example list, so grade by
what you demonstrated, not by how the bug works.

State which downgrade rules you applied. Severity does not decide whether a finding is real; the attacker test does.

## Reports and patches

- One finding per report, with the affected commit, the component, the reproducer, and what you are **not** claiming.
- Patches should be minimal, keep existing style, and include a regression test. Never change consensus-visible
  behaviour unconditionally: it must sit behind a new `EnableEpochs` flag so that replaying history is unaffected.
- Check `develop` and the published advisories first. Findings fixed on an unreleased branch are still useful, so cite
  the commit.
