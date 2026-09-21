# Security Policy

## Overview

The Klever blockchain team takes security vulnerabilities seriously. We appreciate your
efforts to responsibly disclose your findings and will make every effort to acknowledge
your contributions.

This policy states, as precisely as we can, **what we treat as a security vulnerability,
what we treat as hardening, and what we do not treat as a security issue at all**. We
publish these criteria so that reporters know before they invest effort how a finding
will be classified, and so that our decisions are consistent and reviewable rather than
case-by-case.

## Supported Versions

**Only the latest released version is supported.** Security fixes ship forward in the next
release; we do not backport them to earlier releases, and there are no long-term support
branches. If you are not on the most recent release, assume you are missing security fixes.

Release-candidate (`-rcN`) tags are pre-release builds. They are not supported for
production use unless we explicitly ask operators to run a specific candidate, which we
do when an urgent security fix cannot wait for the final release. That candidate carries
the fix it was cut for and is supported until the final release ships; move to the final
when it does. The reverse does not hold: candidates are cut incrementally, so a fix
present in the final release, or in a later candidate, is not necessarily present in an
earlier candidate of the same version.

The current release is listed at
https://github.com/klever-io/klever-go/releases/latest.

### How we express affected versions in advisories

Advisories state the affected range as `< <first-patched-version>`, paired with the
patched version — for example `< 1.7.21` with `patched_versions: 1.7.21`. We do not use
`<= <last-known-vulnerable>`, because under semantic versioning a pre-release such as
`1.7.21-rc1` sorts after `1.7.20`, so a `<=` range silently excludes release candidates
that are in fact affected.

The first patched version is the first tag that carries the fix, and that includes a
release candidate when the fix first shipped in one and we asked operators to run it —
for example `< 1.7.21-rc1` with `patched_versions: 1.7.21-rc1`. Naming the final
release instead would report that candidate as still vulnerable. Where the fix first
shipped in the final, the range is `< 1.7.21`, which correctly includes every candidate
of that version. The range and the patched version always name the same version.

Advisory version fields are written without the `v` prefix, which is how the GitHub
Advisory Database and the Go vulnerability database record Go versions. Git tags and Go
module versions keep the prefix: `1.7.21` in an advisory is the tag `v1.7.21`.

Where a defect was introduced in a specific release rather than being present since the
beginning, we state a lower bound as well, for example `>= 1.7.14, < 1.7.18`.

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues,
discussions, or pull requests.**

### Private Security Advisory (Recommended)

1. Navigate to the **Security** tab of this repository
2. Click **Report a vulnerability**
3. Fill out the vulnerability details form

### Email

Send details to: **security@klever.io**

Please include:

- **Type of vulnerability** (e.g. consensus failure, execution bypass, DoS)
- **Affected component(s)** (e.g. KVM, consensus, networking, state)
- **Step-by-step instructions** to reproduce
- **Proof of concept** — see "Reproducers" below
- **Potential impact**, stated as a concrete demonstrated outcome
- **Suggested mitigation** (if you have one)
- **Your contact information** for follow-up

### Reproducers

A report that includes a runnable reproducer is triaged faster than one that does not.
A reproducer is what turns a theoretical report into demonstrated impact; it does not
raise the severity grade on its own. The most useful form is a Go test in this
repository that fails on the affected version, together with the exact command to run
it and its verbatim output. State explicitly which fork flags (`EnableEpochs`) your
reproducer assumes, since most state behaviour in this codebase is fork-gated.

Please also state what you are **not** claiming. Reports that clearly bound their own
scope are taken more seriously, not less.

## How We Classify Reports

Every report is placed in exactly one of three buckets. Advisory and hardening
findings are credited. Reports that are not a security issue are closed with a
written explanation citing the rule; they are not credited in an advisory or in
release notes.

A duplicate is not a fourth bucket and is not "not a security issue." It stays in
the same bucket as the original finding, is credited, and is closed only once the
canonical advisory is public — or, for a hardening finding, once the fix ships — as
described in the next section.

| Bucket | What it means | Outcome |
| ------ | ------------- | ------- |
| **Security advisory** | A demonstrated impact from the table below | GitHub Security Advisory published after the fix ships; CVE where appropriate; credited in the advisory |
| **Security hardening** | A real weakness that does not on its own produce an impact from the table below — defence-in-depth, unsafe defaults, missing safety rails — including a report that matches an exclusion below but rests on a sound observation | Tracked internally (or as a private unpublished advisory) until the fix ships, then credited in the release notes; **no public issue before the fix, and no advisory published** |
| **Not a security issue** | Matches an exclusion below with no sound underlying weakness, or is not reproducible | Closed with a written explanation citing the specific rule; not credited |

We will always tell you which bucket a report landed in and why.

## Duplicates and Findings Already Under Embargo

Some reports describe an issue we already know about — found in internal review, raised by
an audit, or reported earlier by someone else — where the fix is not yet public. This is
common and it is not a criticism of the report.

We follow the norms established by the major vulnerability-coordination platforms, because
they exist to answer exactly this question and researchers already expect them.

### A duplicate is not "not a security issue"

We never close a duplicate with the reasoning we use for out-of-scope reports. A duplicate
report describes a real finding. GitHub gives us no duplicate status, only Close, so we
record the disposition in your report thread: that the finding is valid, which advisory it
duplicates, and the dated record behind that. We close the report when the canonical
advisory publishes, or when the fix ships for a hardening finding that will have no
advisory, so "closed" never points at something you cannot see.

### How we substantiate a known-issue claim

When we say a report describes something we already knew about, we say it from a dated
record, not from memory. The records we rely on are:

- A commit or pull request, identified by hash and repository, with its date visible
- An internal ticket or audit finding with a verifiable creation date
- Dated written correspondence that states the vulnerability and its impact

A later public disclosure does not show that we knew earlier, and neither does a finding
having been "on our roadmap"; we do not cite either.

You can ask to see the record, and we will share what the embargo allows. Where no dated
record exists, we treat the report as new.

### Findings from audits and internal review

Not every finding we know about starts life as a GitHub advisory. Third-party audit
findings arrive as an audit deliverable, and internal review findings start as tickets or
commits. To keep provenance honest, we record these **when they are found, not when they
are questioned**:

- A finding that meets the advisory bar gets a **draft advisory opened at the time it is
  recorded**, with the auditor or internal finder credited and the audit reference and its
  date noted in the advisory.
- Findings below that bar are tracked as internal dated tickets, which serve the same
  evidentiary purpose.

We can cite an audit finding's identifier and date as proof of prior knowledge without
publishing the audit report itself, so a report still under its own embargo does not
prevent us from substantiating a claim to you.

**If you report something we hold in an audit or ticket but for which no advisory exists
yet, we do not open our own advisory and close yours.** Where the finding meets the
advisory bar, we accept your report as the canonical advisory and add the original finder
alongside you in the credits, citing their dated reference. Where it is hardening, your
report stays open as the record until the fix ships, and the release notes credit you
alongside the original finder. You keep the record you filed; they keep their attribution.

**If we have no dated record at all, it is not a known issue.** A finding that was
discussed informally, or was on someone's list but never written down, does not meet the
standard above. In that case the report is new, and it is credited as new.

### What counts as a duplicate

We use one test, and it turns on what we had already recorded: **a later report of a
path we already recorded is a duplicate of that path. A report that causes us to
change a path we had not recorded is not.** Put another way, a report is a duplicate
only if we would have changed the same code anyway — and the dated record described
above is how that is demonstrated, not our recollection.

Independently reachable paths are separate findings even if we later extract a shared
helper. Sharing a CWE is not enough.

- Several instances of one weakness collapse only when one already-planned
  framework- or interface-level change resolves all of them.
- A report that establishes an impact we had not established is a new finding, not a
  duplicate, even where the underlying defect was known.

### What we can tell you while the fix is embargoed

- That your finding overlaps something already tracked, and whether the overlap is total or
  partial
- The date we first recorded it, with the evidence above
- Which release or fork the remediation is expected in, once known
- Nothing that would disclose an unpublished vulnerability belonging to a third party, and
  no details of another reporter's submission

### Credit

**If you found an issue independently, you are credited — even if you were not the first to
tell us about it.**

Many programs recognise only the first valid report, because a reward has to be paid once.
Credit does not work that way: naming everyone who found an issue takes nothing away from
anyone who found it earlier. So we name everyone.

- **Any report that reaches us before the finding is publicly disclosed is credited as an
  independent discovery**, whether or not we recorded it first.
- Where the overlap is partial, we credit the part you contributed and state what it was.
- Where you report something already fixed on an unreleased branch, we cite the commit so
  you can verify it, and still record the report.

### How crediting works in practice

One vulnerability gets one advisory. We do not open a second advisory for a duplicate,
because two GitHub Security Advisory identifiers for a single issue fragment the record for
everyone downstream who consumes them. Instead:

1. We keep a single canonical record for the finding: an advisory, or an internal ticket
   where the finding is hardening.
2. We confirm the duplicate in a comment on your report at triage, with the dated record
   described above and the release the fix is expected in. Your report stays open.
3. We **add you to that advisory's credits** at the same time, normally as `finder` for
   independent discovery or `reporter` where you were first to notify us. GitHub sends you
   a **credit request.** Accept it if you want your username to appear in the published
   advisory. Declining is a supported choice and we will not press you on it; GitHub does
   not display a credit publicly unless you accept. For a hardening finding we record you
   for the release-note credit instead.
4. We close your report when the canonical advisory publishes, with a comment that links
   it. For a hardening finding we close when the fix ships, with a comment that links the
   release notes.

Two things follow from this that are worth stating plainly:

- **Your open report is your channel during the embargo.** We post the milestones there:
  fix merged, release tagged, fork activated where relevant, advisory published or, for
  hardening, release notes out. Where the
  canonical advisory originated from our own internal review, we also add you as a
  collaborator on it so you can follow the fix directly. Where it originated from another
  researcher, we do not, because that would expose their submission; your own report
  thread carries the updates instead.
- **Closing your report is not a rejection.** It happens at publication, or at release
  for a hardening finding; the closing comment links the public advisory or the release
  notes, and that comment remains in your report thread.

If your report is a variant rather than a duplicate under the test above, none
of this applies — it is accepted and tracked as its own finding.

If you believe we have mis-assigned duplicate status or priority, say so. We will show you
the dated evidence we relied on, and we will correct the record if we cannot support it.

## Severity Classification

We classify by **demonstrated impact**, not by how the finding was discovered or how
sophisticated it is. We then apply the downgrade rules below, which account for how
reachable that impact actually is.

Critical is reserved for impacts that are systemic — the chain, every holder, or a node's
keys — or that pay the attacker and therefore scale. High covers severe harm that is
bounded to identifiable victims or to availability. Where a demonstrated impact matches
more than one bullet, the highest applies; a bypass is graded by what it demonstrably
reaches, not by its mechanism.

### Critical

- Consensus failure, chain halt, or chain split
- Unauthorized minting, or theft of funds belonging to another account — by whatever
  route, including a signature, transaction-validation, or authorization bypass that lets
  an attacker act as the victim
- Remote code execution on a node
- Private key extraction

### High

- Remote unauthenticated node crash or resource exhaustion that affects network availability
- Transaction or signature validation bypass whose demonstrated impact stops short of a
  Critical bullet — for example, a quorum check that still requires a majority of genuine
  signatures
- Authorization bypass over another account's assets or permissions that stops short of
  theft or takeover — for example, altering an asset's properties without reaching its
  holders' balances
- Permanent loss or freezing of user funds that a third party can inflict on a victim

### Medium

- Permanent loss or freezing of user funds reachable only through the affected user's own
  transaction
- Permanent loss or freezing of user funds reachable only under a non-default asset
  configuration
- Bounded denial of service against an individual node
- State inconsistency that does not affect consensus
- Disclosure of non-public node or user data, reachable remotely under the **shipped
  default configuration**

### Low

- Any impact above that the downgrade rules bring to exactly Low
- Information disclosure reachable only from the local host
- Missing hardening on a debug, diagnostic, or non-default surface

> **Note:** "information disclosure" is not automatically Medium. It is Medium only when
> the disclosed data is non-public *and* the endpoint is reachable remotely under the
> configuration we ship. Node telemetry, counters, and diagnostic state reachable only on
> loopback are Low or hardening.

### Downgrade Rules

These are applied after the impact is matched. Each rule that applies reduces the
severity by one level. A finding reduced below Low is classified as hardening.

**A rule is not applied if the matched impact bullet already incorporates it.** We
still cite the rule as reasoning so the match is reviewable; we do not decrement
twice for the same fact.

1. **Non-default configuration** — the impact requires an asset, chain, or node
   configuration that differs from what we ship, and that an issuer or operator chose.
2. **Self-inflicted trigger** — the impact requires the affected user's own transaction,
   with no third party able to cause it.
3. **Privileged actor** — the impact requires an actor who already holds a privileged
   role over the affected asset or node, where that role already confers comparable power.
4. **Local or non-default reachability** — the impact requires access to the local host,
   or requires an interface bound beyond the loopback default we ship.

Worked example: permanent loss of user funds that requires the user's own transaction
*and* a non-default asset configuration matches the Medium self-inflicted bullet. Rule 2
is priced into that bullet and is not applied again; Rule 1 is not, so it applies once and
the result is Low — not hardening. Matching the non-default-asset bullet instead reaches
the same place by the mirror route, which is why the two are listed separately.

The same applies at Low: "information disclosure reachable only from the local host"
already incorporates Rule 4; Rule 4 is not applied again to drop that finding to
hardening.

We will state which rules we applied. If you think we applied one incorrectly, say so —
several published advisories on this repository were re-rated after a reporter pushed back
with a better argument.

### What "shipped default" means

Classification uses the **mainnet deployment as we publish it**: the release binary or
image, the current mainnet configuration package from backup.mainnet.klever.org, and
the run command in the Node Operations Guide. The YAML in this repository is the
developer checkout; a setting counts as shipped only if it survives into that package,
and where the two differ, the package governs.

Under that deployment the REST API binds to `localhost:8080`
(`common/facade.DefaultRestInterface`). Only `--rest-api-interface` moves the listener
off loopback — `--rest-api-interface=0.0.0.0:8080` or `--rest-api-interface :8080` — and
the guide does not pass it. Docker `-p 8080:8080` and `--network=host` carry a
non-loopback listener onto the network but do not rebind one themselves. Either way, the
change is an operator choice, and an operator who makes it is expected to put
authentication and TLS in front of the listener.

## Not a Security Issue

The following are not treated as vulnerabilities and are not published as advisories.
Where the underlying observation is sound, the report is accepted as **hardening** and
credited; where it is not, it is closed as not a security issue. We would rather receive
these than not.

- **REST API exposure.** The shipped deployment binds the REST API to loopback
  (`localhost:8080`). Exposing it more widely is an operator decision, and securing
  that deployment is an operator responsibility. If you bind beyond loopback, put the
  listener behind authentication and TLS (a reverse proxy is the usual shape) and do
  not leave mutate routes unauthenticated. Exposure on its own is not a finding. Where a
  finding does carry demonstrated impact, requiring a non-loopback bind takes the Rule 4
  reachability downgrade rather than leaving scope outright — it is hardening only once
  that downgrade reduces it below Low. Routes that *mutate* node state or configuration
  stay in scope as vulnerabilities.
- **Missing `secured:` on read-only diagnostic routes.** Treated as hardening.
  Tightening those flags is a change to the mainnet configuration package, or an operator
  edit where a deployment publishes the API. Routes that *mutate* node state or
  configuration stay in scope as vulnerabilities.
- **Local attackers already present.** If an attacker must already have code execution,
  filesystem write access, or an account on the node host, they can generally do worse
  directly. The bar for these is correspondingly higher.
- **Debug and diagnostic surfaces** that expose only counters, operational telemetry,
  or cached protocol bookkeeping — that is, data that does not identify peers or
  users, carry key material, or reveal unpublished chain state.
- **Credential-hashing strength** where the shipped default fails closed and the
  credential file already sits alongside material of equal or greater sensitivity.
- **Operator misconfiguration or key-file mismanagement**, including losing or failing to
  provision key material. Improving the safety rails around these is hardening.
- **Duplicates and issues already fixed** on an unreleased branch are not listed here as
  exclusions, because they describe real findings. They are handled under "Duplicates and
  Findings Already Under Embargo" above, and are still credited. Please check `develop`
  before reporting.
- **Theoretical impact without a reproducer**, or automated scanner output with no
  demonstrated exploitability.
- **Third-party dependencies** — please report to the respective maintainers. If a
  dependency issue is reachable through our code in a way the upstream advisory does not
  describe, that is in scope.
- **Test code and fixtures.** Test servers and helpers in this repository are not intended
  for production use and are not hardened.
- **Social engineering, phishing, and physical attacks.**
- **Centralization, governance, and economic-design concerns** that do not stem from a
  code defect. These are welcome as ordinary issues or discussions.

## Response Timeline

These are the targets we plan around. When we see that one will be missed, we say so in
your report thread with the reason and the new expected date.

1. **Initial response**: within 36 hours
2. **Triage and bucket assignment**: within 5 business days, with the reasoning stated
3. **Fix available in a tagged build**, counted from the report date. A release candidate
   we ask operators to run counts; fork-gated activation follows the epoch schedule and is
   outside this window.
   - Critical: 14 days
   - High: 30 days
   - Medium: 90 days
   - Low / hardening: on the ordinary release schedule
4. **Coordinated disclosure**: timeline agreed with you, per the model below

## Disclosure Model

Klever follows a **coordinated, fix-first disclosure model**. Vulnerabilities are
remediated privately and disclosed after operators have had the opportunity to upgrade.
This is consistent with practice for comparable node software.

Two properties of this codebase shape our timing:

- Many state-behaviour fixes are **fork-gated** (`EnableEpochs`). A fix that has shipped in
  a release is not yet active on the network until its fork epoch activates. We disclose
  after **activation**, not after release, where premature disclosure would describe live
  reachable behaviour.
- Where a finding gives an asset issuer or operator an economic advantage over their own
  users, we delay disclosure until the fix is active, so publication does not amount to
  distributing a recipe.

| Severity | Disclosure timing |
| -------- | ----------------- |
| Low / Medium | Approximately four weeks after the fix is released and, where fork-gated, active |
| High | After the fix is active and adoption is confirmed |
| Critical | Case by case; details may be limited or withheld while networks upgrade |

Reporters are credited in the published advisory unless they ask not to be. Hardening
findings are credited in release notes.

## Recognition and Rewards

We do not currently operate a bounty program with published reward tiers, and we would
rather say so plainly than imply terms we have not set.

What we do commit to:

- **Attribution.** Reporters are credited in the published advisory, or in the release
  notes for findings classified as hardening, unless you ask us not to be named.
- **A stated decision.** Every report receives a bucket and the reasoning behind it.
  Advisory and hardening findings also receive a severity, including which downgrade
  rules we applied and why.
- **Safe harbour.** We will not pursue legal action against, or ask platforms to act
  against, anyone who researches and reports in good faith under the responsible
  disclosure guidelines below. If you are unsure whether an activity is covered, ask us
  first at security@klever.io and we will answer before you proceed.

**Monetary awards are discretionary.** We may recognise reports that are especially
severe, especially well-evidenced, or that prevent a real incident. Because there are no
published tiers, no severity rating on this repository constitutes an offer or an
entitlement, and we would ask reporters not to invest effort on the assumption of payment.

**Scope** for the purposes of this policy: the core blockchain protocol, consensus, KVM,
smart contract execution, state and account handling, networking, and cryptographic
implementations in this repository.

We are working toward a formally hosted program. If and when one launches, its published
impacts-in-scope list and reward ranges will become authoritative over this section, and we
will say so here.

## Responsible Disclosure Guidelines

Please:

- ✅ Avoid privacy violations, data destruction, and service disruption
- ✅ Only interact with accounts you own or have explicit permission to test
- ✅ Do not exploit beyond what is necessary to demonstrate the issue
- ✅ Keep details confidential until we have coordinated disclosure
- ✅ Give us reasonable time to fix before public disclosure

Please do not:

- ❌ Access, modify, or delete data that is not yours
- ❌ Perform DoS/DDoS against mainnet or public testnet
- ❌ Compromise user privacy or degrade user experience
- ❌ Execute attacks against network participants
- ❌ Publicly disclose before coordinated release

## Security Best Practices for Node Operators

- Keep node software up to date
- Run the current mainnet configuration package from backup.mainnet.klever.org and
  refresh it when a release says to; the YAML in this repository is a developer checkout
- Leave the REST API on loopback (`localhost:8080`) unless you have a reason not to
- If you bind beyond loopback — a non-loopback `--rest-api-interface`, reached either by
  Docker `-p 8080:8080` or `--network=host` — require authentication and TLS termination
  in front of the listener. Do not leave mutate routes unauthenticated.
- Back up and protect validator key material; verify your node starts under the identity
  you registered
- Follow secure key management practices and use hardware wallets for significant holdings

## Deploying / Exposing the REST API

The node's REST API performs **no origin checking** (the node `/log` WebSocket route is the sole
exception — see [WebSocket origin policy](#websocket-origin-policy)), and applies access control
only where a route is explicitly marked `secured` in `api.yaml`. Whether it is safe is entirely a
function of how you deploy it. This section is the operator-facing counterpart to the user guidance above, and
covers both deployables: the validator/observer node (`config/node/`) and the seednode
(`config/seednode/`), which ship separate API configurations.

### The exposure model

By default the API binds to `localhost:8080` (`DefaultRestInterface`, `common/facade/nodeFacade.go`),
reachable only from the node host. **Exposing it beyond that is a deliberate operator choice, and the
node does not second-guess it.**

If you expose the API, it MUST be fronted by a reverse proxy that terminates TLS, enforces
origin/CORS policy, and requires authentication. Firewall the API port so the node is reachable only
through that proxy. Outside `/log`, the node itself will not reject a cross-origin request:
`CheckOrigin` returns `true` unconditionally for `/subscribe` (`network/api/websocket/routes.go`).
That is by design — origin policy belongs to the proxy — but it means an exposed node with no proxy
has essentially no origin protection. `/log` is the exception on both deployables: the node route
(`network/api/api.go`) enforces `logWebSocketAllowedOrigins`, and the seednode route
(`cmd/seednode/api/api.go`) has no allowlist and blocks every browser origin, because `/log` can be
Basic-Auth protected and streams internal node state.

**Do not run a browser on a validator host.** Because the rest of the API has no origin control, any
page you visit can issue cross-origin requests to `localhost:8080` and reach the node.

### Per-endpoint guidance

Most routes are configured in `config/node/api.yaml` (`config/seednode/api.yaml` for the seednode);
the exceptions are `/debug/pprof/*` and `/swagger/*`, both covered below. For the configured ones,
two flags govern each route, and they do **not** mean what their names suggest when combined:

- `open` controls whether the route is **registered at all**.
- `secured` only **attaches Basic Auth** to a route that is already open.

> **`secured: true` with `open: false` does not produce an authenticated endpoint — it produces no
> endpoint.** The route is simply absent. The node logs a warning for `/subscribe` in this case
> (`network/api/api.go`); there is no equivalent warning for other routes, so check your
> config rather than relying on a log line.

**`/log`** — ships enabled and authenticated (`open: true`, `secured: true`). It streams node-wide
logs, which can include operational detail you would not want public. Keep `secured: true` if it is
reachable off-host, or set `open: false` to remove it entirely.

**`/subscribe`** — ships enabled and **unauthenticated** (`open: true`, no `secured`). It is a public
event feed by design. For a public or mainnet deployment, add `secured: true` to require Basic Auth
on the handshake, or set `open: false` to disable it. Its resource limits are covered below.

**`/node/debug`, `/node/peerinfo`, `/node/p2pstatus`, `/node/heartbeatstatus`** — ship enabled and
authenticated (`open: true`, `secured: true`). `/node/debug` returns cached interceptor and resolver
state. `/node/peerinfo` returns the addresses and validator public keys of every peer you are
connected to; the `pid` query parameter only filters that list, and omitting it returns all of them.
`/node/p2pstatus` reports the node's own p2p listen addresses, and `/node/heartbeatstatus` the
heartbeat view of the validator set. Together they describe your network topology. Upgrading the
binary does not rewrite an existing `api.yaml`: a node configured before these defaults keeps
serving them unauthenticated, and logs a warning at startup for each one (`network/api/api.go`).
Add `secured: true`, or set `open: false`, on each of them.

**`/debug/pprof/*`** — registered only when the node runs with `--profile-mode`, and **not governed
by `api.yaml`**: the routes are attached directly to the gin engine outside the normal route-group
registration (`network/api/api.go`), so they have no `open`/`secured` flag and no Basic Auth. The
flag is the only control. `/debug/pprof/heap` and `/debug/pprof/goroutine` dump process memory and
full goroutine stacks to any caller that can reach the port. Never run with `--profile-mode` on an
exposed node; if you must profile, keep the API bound to `localhost` and tunnel to it.

**`/swagger/*`** — registered unconditionally when the API starts (`network/api/api.go`), before the
`api.yaml` routes: no `open`/`secured` flag, no Basic Auth, and unlike `/debug/pprof/*` not even a
CLI flag to disable it. It serves the Swagger UI and the compiled-in spec, generated at build time,
so it lists every route the binary knows about including the ones you set `open: false`. No runtime
state leaks through it, so this is surface enumeration rather than data disclosure. Block it at the
reverse proxy if that matters to you.

### Seednode

The seednode is a separate deployable with its own API config (`config/seednode/api.yaml`), its own
`credentials` block, and its own routes. Hardening `config/node/api.yaml` does nothing for it — go
through this section a second time against the seednode file.

Its shipped defaults differ from the node's:

- **`/log`** — `open: true`, `secured: true`, same as the node.
- **`/peers`** — `open: true` with **no `secured`**. It exposes connected peer addresses, i.e. your
  network topology. Set `open: false` to remove it, or `secured: true` to require auth, unless you
  intend that data to be public.
- **`/node/metrics`** — `open: true` and deliberately unsecured, because Prometheus does not send
  Basic Auth. Restrict it at the network layer rather than in `api.yaml`, unless your scraper is
  configured for credentials.

### Credentials

Authentication is HTTP Basic Auth (`network/api/middleware/authHandler.go`). The `password` field
in `api.yaml` is **not** the password — it is the **hex-encoded digest** of the password under the
configured hasher (`authHandler.go`; hasher selected by `hasher.type`, `sha256` by default).

The shipped credentials are placeholders and are not usable — `config/node/api.yaml` ships two
entries, and `config/seednode/api.yaml` ships its own:

```yaml
credentials:
  - username: example
    password: hashed password
  - username: example2
    password: hashed password
hasher:
  type: sha256
```

Replace **every** entry, in both files, before enabling `secured` anywhere. Generate the digest
without leaving the plaintext password in your shell history:

```bash
read -rs -p 'password: ' pw && printf '%s' "$pw" | sha256sum | cut -d' ' -f1; unset pw
```

(`sha256sum` is GNU coreutils; on macOS use `shasum -a 256`.)

Leaving the credentials list **empty** does not disable auth — it makes every authenticated request
fail with HTTP 500.

### Recommended hardened configuration

For a node whose API is reachable off-host, start from this and adjust:

```yaml
# config/node/api.yaml
apiPackages:
  log:
    routes:
      - name: /log
        open: true
        secured: true       # or open: false to remove the route entirely
  subscribe:
    routes:
      - name: /subscribe
        open: true
        secured: true       # public feed by default; require auth when exposed

credentials:
  - username: <operator>
    password: <hex sha256 digest of the password>
hasher:
  type: sha256
```

**This edits the `log` and `subscribe` entries of the shipped file — it is not a replacement for the
whole file.** The real `apiPackages` block also carries the other route groups (`address`,
`transaction`, `block`, `node`, `vm`, …); dropping them leaves `apiPackages` without those keys, and
every route whose group is missing fails its enabled check and is never registered. The same applies
to the indentation: `log`/`subscribe` must stay nested under `apiPackages`, while `credentials` and
`hasher` stay at the top level. Get that wrong and the config parses without error while silently
discarding the credentials, which lands you in the HTTP 500 state described above.

Pair it with: `--rest-api-interface=localhost:8080` (the default) plus a reverse proxy, or a firewall
rule restricting the port to the proxy host.

### WebSocket resource limits

`/subscribe` and `/log` connection and subscription limits are tunable under `webServer` in
`config/node/config.yaml`:

| Setting | Purpose | `0` means |
|---|---|---|
| `webSocketConnections` | node-wide cap on live `/subscribe` connections | unlimited |
| `webSocketConnectionsPerIP` | per-source-IP cap for `/subscribe` | unlimited |
| `webSocketMaxAddressesPerSubscribe` | addresses accepted in one subscribe call | use the built-in default |
| `webSocketMaxAddressesPerClient` | total addresses one connection may watch | use the built-in default |
| `logWebSocketConnections` | node-wide cap on live `/log` connections | use the built-in default |
| `logWebSocketConnectionsPerIP` | per-source-IP cap for `/log` | unlimited |
| `logWebSocketAllowedOrigins` | browser origins allowed to open `/log` | block every browser origin |

Note the split in the last column. `webSocketConnections`, `webSocketConnectionsPerIP` and
`logWebSocketConnectionsPerIP` treat `0` as unlimited. The two address caps and
`logWebSocketConnections` fall back to their built-in defaults on `0` (the fields are unsigned, so
there is no negative to reject), so they **cannot be disabled** — to lift them, set an explicit
high value rather than `0`.

`logWebSocketConnections` is in the second group on purpose. Before the `/log` cap existed,
streaming ran on the request goroutine and so held a `simultaneousRequests` slot for the whole
connection, bounding live `/log` connections at that setting (100 in the shipped config). Streaming
now runs off that goroutine and the
slot is released at the upgrade, so treating `0` as unlimited would leave a node upgraded with a
`config.yaml` predating the key *weaker* than before. It falls back to 32 instead, and the node
logs a warning at startup when that happens. The per-IP cap keeps `0` = unlimited because behind a
proxy it has to be disableable; the node-wide cap still bounds the route when it is off.

The `/log` caps are deliberately far smaller than `/subscribe`'s (32/8 versus 4096/1024). Every
live `/log` connection registers a process-global log observer, so each log line is formatted and
fanned out once per connection; `/log` is an operator diagnostic route, not a public feed.

**A raised log profile stays raised while any `/log` session is connected.** On a secured `/log`,
an authenticated client may send a logger profile in its handshake, and that profile is applied to
the *process-global* logger — so `*:TRACE` writes trace output to every configured sink (disk
included), not just to that websocket. The original profile is snapshotted when the first session
connects and restored when the last one disconnects, which is what stops two overlapping sessions
from reverting the node to each other's setting. The trade-off is that the raised profile is only
reverted at the *last* disconnect: a session that raised verbosity and left keeps the node at that
level for as long as any other `/log` client — including an idle one that answers pings and never
asked for it — stays connected. Restart the tailer set, or reapply the intended profile, after a
verbose debugging session.

**Behind a reverse proxy, every client shares the proxy's IP**, so the per-IP caps throttle all of
them together. Raise them, or set them to `0` to disable, for proxied deployments — and enforce
per-client limits at the proxy instead. `logWebSocketConnectionsPerIP` is the one that bites first:
at its default of 8, a proxied deployment reaches the per-IP limit long before the node-wide 32.

Per-IP caps (and the `sameSourceRequests` throttler) bucket IPv6 sources by their `/64` prefix.
Keying on the full `/128` would let anyone holding a routed `/64` pick a fresh source address per
connection and walk past every per-IP limit. `/64` is a reduction, not an identity: it is the
smallest prefix ISPs delegate, but `/56` and `/48` are common, so one customer can still hold 256
to 65536 buckets. The `/log` per-IP cap is backstopped by the node-wide cap; `sameSourceRequests`
is not, and there the quota is multiplied by the client's allocation size. Link-local zone
identifiers are dropped before bucketing, NAT64 (`64:ff9b::/96`) keys on the embedded IPv4 since
the translator — not the peer — writes those bits, and Teredo and 6to4 are bucketed like any other
IPv6 because their embedded IPv4 is client-constructed.

Note that neither HTTP throttler bounds live WebSocket connections. `simultaneousRequests`
releases its slot at the HTTP-to-WebSocket upgrade, and `sameSourceRequests` counts requests per
source until its periodic reset, so a long-lived socket costs it exactly one request. The
`webSocket*` and `logWebSocket*` settings are what do.

### WebSocket origin policy

The two WebSocket routes take deliberately different stances, because they differ in what an
attacker gains by driving one from a web page:

- **`/log` enforces an origin allowlist.** A client that sends no `Origin` header (the log viewer,
  `curl`, `wscat`) is always allowed — `Origin` is set by the browser and page script cannot forge
  it, so its absence means no page is driving the connection. A request that *does* carry an
  `Origin` is a browser, and is admitted only if `logWebSocketAllowedOrigins` lists it. The empty
  default therefore blocks every web page while leaving normal tooling working. This matters
  because `/log` can be Basic-Auth protected: without it, any site an operator visits could open
  `ws://localhost:8080/log` and stream node logs on their credentials.
- **`/subscribe` does not enforce origin** (KLC-2450): the node is expected to run headless behind
  an operator proxy that owns origin/CORS policy. This is a delegation, not an absence of risk. An
  earlier version of this document said `/subscribe` "carries no ambient credentials" — that is
  wrong. When the route is marked `secured`, Basic Auth is attached to it exactly as it is to
  `/log`, and browsers replay cached Basic credentials on a same-host WebSocket handshake. **A
  secured `/subscribe` reachable from a browser without a proxy enforcing `Origin` is exposed to
  the same CSWSH that the `/log` allowlist closes.** Either enforce origin at the proxy, or leave
  `/subscribe` unsecured and treat its feed as public.

### Seednode `/log`

The seednode's `/log` route (`cmd/seednode/api/api.go`) shares the sender with the node, so the
handshake limit and deadline, the rolling `pongWait` deadline, the ping loop, the write deadline,
the profile refcount, the log-injection guard and the panic containment all apply, and its
upgrader blocks every browser origin (there is no allowlist to configure, so no browser can open
it at all). That matters because the seednode ships `/log` with `secured: true`
(GHSA-9v8p-frvj-2pcm / KLC-2438), and `secured` also turns profile application on: without the
origin check, a page an operator visited could stream seednode logs on cached Basic credentials
and mute the process-global logger.

Live seednode `/log` connections are capped at a built-in 32, node-wide, with no per-IP dimension
and no configuration knob: the seednode has no `webServer` antiflood section to read one from,
and every live session registers a process-global observer that formats every log line, so
unbounded is the wrong default for a route nobody needs tens of. Rejected upgrades are budgeted
the same way as the node's, one line per window with a counter of their own. Keep the route
disabled unless you are actively tailing it.

### Operational checklist

- [ ] API bound to `localhost` unless deliberately exposed
- [ ] If exposed: reverse proxy enforcing TLS, origin/CORS, and authentication
- [ ] API port firewalled to the proxy host
- [ ] Real credentials configured; all placeholder entries replaced, in both `config/node/api.yaml`
      and `config/seednode/api.yaml` if you run a seednode
- [ ] `/log` secured or disabled
- [ ] `/subscribe` secured or disabled if not intended to be public
- [ ] `/node/debug`, `/node/peerinfo`, `/node/p2pstatus` and `/node/heartbeatstatus` secured or
      disabled — shipped secured, but an `api.yaml` from before the upgrade is not rewritten
- [ ] Seednode `/peers` disabled or secured unless network topology is meant to be public
- [ ] `--profile-mode` off, or API localhost-only — `/debug/pprof` is unauthenticated
- [ ] `/swagger` blocked at the proxy if you do not want the API surface enumerated
- [ ] `webSocketConnectionsPerIP` and `logWebSocketConnectionsPerIP` adjusted if behind a proxy
- [ ] `logWebSocketConnections` sized for the deployment — `0` falls back to the built-in 32
- [ ] `logWebSocketAllowedOrigins` left empty unless a browser-based log viewer is actually used
- [ ] Seednode `/log` disabled unless actively in use — its cap is a built-in 32 with no per-IP dimension
- [ ] No browser running on validator hosts
- [ ] Node software kept up to date
- [ ] Key management per the practices above

## Security Audits

Our codebase undergoes regular security audits by reputable third-party firms. Audit
reports are published on our website and documentation.

## Contact

- **Email**: security@klever.io
- **Website**: https://klever.org
- **Documentation**: https://docs.klever.org

## Acknowledgments

We thank the security researchers and community members who help keep Klever safe.
Contributors who follow responsible disclosure are acknowledged, with permission, in our
advisories and release notes.

---

**Last Updated**: September 2026
