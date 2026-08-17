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
production use, and a fix present in a final release is not necessarily present in an
earlier release candidate of the same version.

The current release is listed at
https://github.com/klever-io/klever-go/releases/latest.

### How we express affected versions in advisories

Advisories state the affected range as `< <first-patched-version>`, paired with the
patched version — for example `< v1.7.21` with `patched_versions: v1.7.21`. We do not use
`<= <last-known-vulnerable>`, because under semantic versioning a pre-release such as
`v1.7.21-rc1` sorts after `v1.7.20`, so a `<=` range silently excludes release candidates
that are in fact affected.

Version identifiers are written with the `v` prefix, matching our Git tags and Go module
versions.

Where a defect was introduced in a specific release rather than being present since the
beginning, we state a lower bound as well, for example `>= v1.7.14, < v1.7.18`.

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

A report that includes a runnable reproducer is triaged faster and classified higher than
one that does not. The most useful form is a Go test in this repository that fails on the
affected version, together with the exact command to run it and its verbatim output. State
explicitly which fork flags (`EnableEpochs`) your reproducer assumes, since most state
behaviour in this codebase is fork-gated.

Please also state what you are **not** claiming. Reports that clearly bound their own
scope are taken more seriously, not less.

## How We Classify Reports

Every report is placed in exactly one of three buckets. All three are legitimate outcomes
and all three are credited.

| Bucket | What it means | Outcome |
| ------ | ------------- | ------- |
| **Security advisory** | A demonstrated impact from the table below | GitHub Security Advisory published after the fix ships; CVE where appropriate; credited in the advisory |
| **Security hardening** | A real weakness that does not on its own produce an impact from the table below — defence-in-depth, unsafe defaults, missing safety rails | Tracked as a normal issue, fixed on the ordinary release schedule, credited in the release notes; **no advisory published** |
| **Not a security issue** | Matches an exclusion below, is a duplicate, or is not reproducible | Closed with a written explanation citing the specific rule |

We will always tell you which bucket a report landed in and why.

## Duplicates and Findings Already Under Embargo

Some reports describe an issue we already know about — found in internal review, raised by
an audit, or reported earlier by someone else — where the fix is not yet public. This is
common and it is not a criticism of the report.

We follow the norms established by the major vulnerability-coordination platforms, because
they exist to answer exactly this question and researchers already expect them.

### A duplicate is not "not a security issue"

We never close a duplicate with the reasoning we use for out-of-scope reports. A duplicate
report describes a real finding. It is closed as a duplicate, in writing, with a statement
that the finding is valid.

### The burden of proof is on us, not on you

If we close a report because we already knew about the issue, **we must prove that we knew.
An unevidenced claim of prior knowledge is not a valid reason to close a report.** We
consider only the following acceptable as proof, and we will provide one of them:

- A commit or pull request, identified by hash and repository, with its date visible
- An internal ticket or audit finding with a verifiable creation date
- Dated written correspondence that states the vulnerability and its impact

Pointing at a later public disclosure is **not** proof that we knew of an issue earlier.
Neither is an assertion that the finding was "already on our roadmap."

**You have the right to ask us for this evidence, and we will provide it.** If we cannot,
the report is treated as a new finding.

### Findings from audits and internal review

Not every finding we know about starts life as a GitHub advisory. Third-party audit
findings arrive as an audit deliverable, and internal review findings start as tickets or
commits. To keep provenance honest, we record these **when they are found, not when they
are questioned**:

- A finding that meets the advisory bar gets a **draft advisory opened at the time it is
  recorded**, with the auditor or internal finder credited and the audit reference and its
  date noted in the advisory.
- Findings below that bar are tracked as ordinary dated tickets, which serve the same
  evidentiary purpose.

We can cite an audit finding's identifier and date as proof of prior knowledge without
publishing the audit report itself, so a report still under its own embargo does not
prevent us from substantiating a claim to you.

**If you report something we hold in an audit or ticket but for which no advisory exists
yet, we do not open our own advisory and close yours.** We accept your report as the
canonical advisory and add the original finder alongside you in the credits, citing their
dated reference. You keep the record you filed; they keep their attribution.

**If we have no dated record at all, it is not a known issue.** A finding that was
discussed informally, or was on someone's list but never written down, does not meet the
standard above. In that case the report is new, and it is credited as new.

### What counts as a duplicate

We use the remediation test rather than the vulnerability class: **two findings are the
same only if fixing one removes the need to change code for the other.**

- Reports that reach the same subsystem through a different code path are **separate
  findings**, even when the eventual fix ends up shared, and even when they carry the same
  CWE.
- Several instances of one weakness are a single finding only when one framework- or
  interface-level change resolves all of them.
- A report that establishes an impact we had not established is a new finding, not a
  duplicate, even where the underlying defect was known.

If your report causes us to change code we would not otherwise have changed, it is not a
duplicate.

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

1. We keep a single canonical advisory for the finding.
2. We **add you to that advisory's credits**, normally as `finder` for independent
   discovery or `reporter` where you were first to notify us.
3. GitHub sends you a **credit request that you must accept.** Until you accept, your credit
   stays pending and may not appear when the advisory is published, so please accept it. We
   will remind you once.
4. We then close your own report with a comment that states it is a valid duplicate, names
   the canonical advisory, and gives the dated evidence described above.

Two things follow from this that are worth stating plainly:

- **Closing your report is not a rejection.** GitHub offers maintainers no "duplicate"
  status — the only available action is to close — so the disposition lives in the closing
  comment. That comment is part of your report thread and remains available to you.
- **Access to the canonical advisory depends on who filed it.** If it originated from our
  own internal review, we can add you as a collaborator so you can follow the fix. If it
  originated from another researcher, we will not, because that would expose their
  submission. In that case you are credited but will see the advisory when it publishes.

If your report is a variant rather than a duplicate under the remediation test above, none
of this applies — it is accepted and tracked as its own finding.

If you believe we have mis-assigned duplicate status or priority, say so. We will show you
the dated evidence we relied on, and we will correct the record if we cannot support it.

## Severity Classification

We classify by **demonstrated impact**, not by how the finding was discovered or how
sophisticated it is. We then apply the downgrade rules below, which account for how
reachable that impact actually is.

### Critical

- Consensus failure, chain halt, or chain split
- Unauthorized minting, or theft of funds belonging to another account
- Remote code execution on a node
- Private key extraction

### High

- Remote unauthenticated node crash or resource exhaustion that affects network availability
- Transaction or signature validation bypass
- Authorization bypass granting control over another account's assets or permissions
- Permanent loss or freezing of user funds that a third party can inflict on a victim

### Medium

- Permanent loss or freezing of user funds reachable only through the affected user's own
  transaction, or only under a non-default asset configuration
- Bounded denial of service against an individual node
- State inconsistency that does not affect consensus
- Disclosure of non-public node or user data, reachable remotely under the **shipped
  default configuration**

### Low

- Any impact above, reduced by one or more downgrade rules
- Information disclosure reachable only from the local host
- Missing hardening on a debug, diagnostic, or non-default surface

> **Note:** "information disclosure" is not automatically Medium. It is Medium only when
> the disclosed data is non-public *and* the endpoint is reachable remotely under the
> configuration we ship. Node telemetry, counters, and diagnostic state reachable only on
> loopback are Low or hardening.

### Downgrade Rules

These are applied after the impact is matched. Each rule that applies reduces the
severity by one level. A finding reduced below Low is classified as hardening.

1. **Non-default configuration** — the impact requires an asset, chain, or node
   configuration that differs from what we ship, and that an issuer or operator chose.
2. **Self-inflicted trigger** — the impact requires the affected user's own transaction,
   with no third party able to cause it.
3. **Privileged actor** — the impact requires an actor who already holds a privileged
   role over the affected asset or node, where that role already confers comparable power.
4. **Local or non-default reachability** — the impact requires access to the local host,
   or requires an interface bound beyond the loopback default we ship.

We will state which rules we applied. If you think we applied one incorrectly, say so —
several published advisories on this repository were re-rated after a reporter pushed back
with a better argument.

## Not a Security Issue

The following are not treated as vulnerabilities. Most are still accepted as **hardening**
where the underlying observation is sound, and we would rather receive them than not — but
they will not be published as advisories.

- **REST API exposure.** The node binds its REST API to loopback by default. Exposing it
  more widely is an operator decision, and securing that deployment is an operator
  responsibility. See our REST API deployment hardening documentation. Findings that
  depend on the API being bound beyond the shipped default are hardening at most.
- **Missing `secured:` on read-only diagnostic routes.** Treated as hardening. Routes that
  *mutate* node state or configuration are in scope as vulnerabilities.
- **Local attackers already present.** If an attacker must already have code execution,
  filesystem write access, or an account on the node host, they can generally do worse
  directly. The bar for these is correspondingly higher.
- **Debug and diagnostic surfaces** that are disabled by default, or that only expose
  counters and operational telemetry.
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

1. **Initial response**: within 48 hours
2. **Triage and bucket assignment**: within 5 business days, with the reasoning stated
3. **Fix development**:
   - Critical: 7–14 days
   - High: 14–30 days
   - Medium: 30–60 days
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
- **A stated decision.** Every report receives a bucket, a severity, and the reasoning
  behind both — including which downgrade rules we applied and why.
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
- Do not expose the REST API beyond loopback without authentication and TLS termination
- Review the REST API deployment hardening documentation before running a public observer
- Back up and protect validator key material; verify your node starts under the identity
  you registered
- Follow secure key management practices and use hardware wallets for significant holdings

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

**Last Updated**: August 2026
