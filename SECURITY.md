# Security Policy

## Overview

The Klever blockchain team takes security vulnerabilities seriously. We appreciate your
efforts to responsibly disclose your findings and will make every effort to acknowledge
your contributions.

This policy states, as precisely as we can, **what we treat as a security vulnerability,
what we fix in the open as an ordinary defect, and what we do not treat as a security
issue at all**. We
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

If you already know that no third party can trigger what you found — a defect only the
affected account holder or the node's own operator can cause — it is not a vulnerability
under this policy. An ordinary public issue or pull request is the right channel for it,
and it will be handled faster there. If you are not sure, report it privately and we
will tell you which it is.

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
- **Proof of concept** — required; see "Reproducers" below
- **Potential impact**, stated as a concrete demonstrated outcome
- **Suggested mitigation** (if you have one)
- **Your contact information** for follow-up

### Reproducers

**A working reproducer is a condition of admission, not an optional field.** A report
that does not demonstrate the finding against a named commit SHA or release tag, or
against the public testnet, is closed — with the bucket and the rule stated, as every
report is, but without further discussion of the finding's technical merits. This
applies however the finding was produced, and however plausible the reasoning behind it
looks.

A reproducer is what turns a theoretical report into demonstrated impact; it does not
raise the severity grade on its own. The most useful form is a Go test in this
repository that fails on the affected version, together with the exact command to run
it and its verbatim output. State explicitly which fork flags (`EnableEpochs`) your
reproducer assumes, since most state behaviour in this codebase is fork-gated.

**Denial-of-service findings must be reproduced on a local devnet or a private network
you control, and the report must state the topology you used.** The responsible
disclosure guidelines below prohibit denial of service against mainnet and public
testnet, so neither is available to you for this class of finding.

Please also state what you are **not** claiming. Reports that clearly bound their own
scope are taken more seriously, not less.

## What Counts as a Vulnerability

A vulnerability needs an attacker. Concretely, it requires **someone other than the
affected party who can trigger the impact, and who gains something by doing so** —
access, funds, disruption, or information they were not entitled to.

A defect that only the affected party can cause is a defect, not a vulnerability. That
covers an account holder whose own transaction damages their own balance, and an
operator who opens a route on their own node and then calls it. Both can be real bugs
we want fixed, and we do fix them — in the open, as ordinary issues and pull requests,
credited in the commit and the release notes. They do not go through the advisory
process, because there is no one to coordinate disclosure against.

This test comes before severity. A finding that fails it is not graded and not
published as an advisory, however serious the outcome looks.

## How We Classify Reports

Every report is placed in exactly one of four buckets. The first three are credited.
Reports that are not a security issue are closed with a written explanation citing the
rule; they are not credited in an advisory or in release notes.

Most reports that describe something real land in **fixed in the open**. That is not a
lesser outcome — it is the normal one, and it gets the fix shipped sooner, because
nothing waits on an embargo.

A duplicate is not a bucket of its own and is not "not a security issue." It stays in
the same bucket as the original finding, is credited, and is closed only once the
canonical advisory is public — or, where no advisory will publish, once the fix ships — as
described in the next section.

| Bucket | What it means | Outcome |
| ------ | ------------- | ------- |
| **Security advisory** | Passes the attacker test above, has a demonstrated impact, and is not reduced below Low by the downgrade rules | GitHub Security Advisory published after the fix ships; CVE where the advisory meets the CVE Program's own criteria; credited in the advisory |
| **Fixed in the open** | A real defect or a sound hardening observation that **fails the attacker test** — no third party can trigger it, or triggering it gains them nothing — or one that passes the test but the downgrade rules reduce below Low | Ordinary public issue and pull request, on the normal release schedule, credited in the commit and the release notes; no embargo and no advisory |
| **Security hardening (embargoed)** | The narrow case: fails the attacker test today, but fixing it in public would itself show how to reach a finding that passes | Tracked privately until the fix ships, then credited in the release notes; **no public issue before the fix, and no advisory published** |
| **Not a security issue** | Matches an exclusion below with no sound underlying defect, or is not reproducible | Closed with a written explanation citing the specific rule; where there is a real non-security bug underneath, we say so and point you at the public tracker |

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
advisory publishes, or when the fix ships for a finding that will have no
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
alongside you in the credits, citing their dated reference. Where no advisory will
publish, your
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

1. We keep a single canonical record for the finding: an advisory, or an internal
   ticket or public issue where no advisory will publish.
2. We confirm the duplicate in a comment on your report at triage, with the dated record
   described above and the release the fix is expected in. Your report stays open.
3. We **add you to that advisory's credits** at the same time, normally as `finder` for
   independent discovery or `reporter` where you were first to notify us. GitHub sends you
   a **credit request.** Accept it if you want your username to appear in the published
   advisory. Declining is a supported choice and we will not press you on it; GitHub does
   not display a credit publicly unless you accept. Where no advisory will publish we
   record you
   for the release-note credit instead.
4. We close your report when the canonical advisory publishes, with a comment that links
   it. Where no advisory will publish we close when the fix ships, with a comment that
   links the
   release notes.

Two things follow from this that are worth stating plainly:

- **Your open report is your channel during the embargo.** We post the milestones there:
  fix merged, release tagged, fork activated where relevant, advisory published or,
  where none will publish, release notes out. Where the
  canonical advisory originated from our own internal review, we also add you as a
  collaborator on it so you can follow the fix directly. Where it originated from another
  researcher, we do not, because that would expose their submission; your own report
  thread carries the updates instead.
- **Closing your report is not a rejection.** It happens at publication, or at release
  where no advisory will publish; the closing comment links the public advisory or the
  release
  notes, and that comment remains in your report thread.

If your report is a variant rather than a duplicate under the test above, none
of this applies — it is accepted and tracked as its own finding.

If you believe we have mis-assigned duplicate status or priority, say so. We will show you
the dated evidence we relied on, and we will correct the record if we cannot support it.

## Severity Classification

This applies to findings that have already passed the attacker test above. We classify
by **demonstrated impact**, not by how the finding was discovered or how sophisticated
it is. We then apply the downgrade rules below, which account for how reachable that
impact actually is.

Severity does not decide whether we publish an advisory — the attacker test does. A
finding that a third party can trigger gets an advisory even at Low. The one exception is
a finding the downgrade rules reduce *below* Low: that is too slight to coordinate
disclosure around, so it is fixed in the open like any other minor defect.

We grade against four levels. **Critical** is reserved for impacts that are systemic —
the chain, every holder, or a node's keys — or that pay the attacker and therefore scale.
**High** covers severe harm that is bounded to identifiable victims or to availability.
**Medium** covers harm bounded to a single account or node. **Low** covers impact that
on its own reaches no further than a local or non-default surface, and any higher impact
that the downgrade rules bring to exactly Low.
Where a demonstrated impact matches more than one level, the highest applies; a bypass is
graded by what it demonstrably reaches, not by its mechanism.

One situation reaches Medium by a route that already prices a downgrade rule into the
grade: harm reachable only under an asset configuration its issuer chose. That is not
hypothetical — an issuer-chosen configuration is live on mainnet and holds real
balances, and it is non-default only in the sense that we did not pick it. Where a
finding matches on that route, Rule 1 is priced in and is not applied to it again.

Harm that only the affected account holder's own transaction can cause used to sit here
too. It no longer does: it fails the attacker test, so it is a defect we fix in the
open rather than a graded vulnerability.

We do not publish a list of example vulnerabilities per level. Which level a finding
lands on is a judgement about demonstrated impact, and we state that reasoning on the
report rather than inviting findings to be written against a taxonomy.

> **Note:** "information disclosure" is not automatically Medium. It is Medium only when
> the disclosed data is non-public *and* the endpoint is reachable remotely under the
> configuration we ship. Node telemetry, counters, and diagnostic state reachable only on
> loopback are Low, or fixed in the open.

### Downgrade Rules

These are applied after the impact is matched. Each rule that applies reduces the
severity by one level. A finding reduced below Low is fixed in the open, unless
publishing the fix would itself show how to reach a finding that passes the attacker
test.

**A rule is not applied if the matched impact already incorporates it.** We still cite
the rule as reasoning so the match is reviewable; we do not decrement twice for the same
fact.

1. **Non-default configuration** — the impact requires an asset, chain, or node
   configuration that differs from what we ship, and that an issuer or operator chose.
2. **Self-inflicted trigger** — *superseded by the attacker test.* Where the impact
   requires the affected party's own action and no third party can cause it, the finding
   leaves the vulnerability track entirely and is fixed in the open; it is no longer
   downgraded by one level. Kept in place so that advisories citing Rule 3 or Rule 4
   stay readable.
3. **Privileged actor** — the impact requires an actor who already holds a privileged
   role over the affected asset or node, where that role already confers comparable power.
4. **Local or non-default reachability** — the impact requires access to the local
   host, or requires an interface bound beyond the loopback default we ship.

Worked example, in the order we apply it:

1. **Attacker test** — an asset's issuer can inflict the impact on holders of that
   asset and gains by doing so. Someone other than the affected party is involved and
   gains, so this is a vulnerability and not a defect.
2. **Matched impact** — harm bounded to the accounts holding that asset, reachable only
   under an asset configuration its issuer chose. **Starting grade: Medium**, with
   Rule 1 priced into that route and not applied again.
3. **Rule 3 applies once** — but only because the role the issuer already holds over
   this asset confers the same power the finding reaches, so the finding hands them
   nothing they could not already do. Where the impact goes beyond that power — reaching
   balances the issuer could not already move, or minting they could not already perform
   — the role is not comparable, Rule 3 does not apply, and the grade stands.
4. **Final grade: Low** — and still a published advisory, because the attacker test
   decides that, not the grade.

The same guard applies at Low. Where the graded impact already assumes local-only
reachability, Rule 4 is not applied again to push the finding out of the graded range.

Two reachability cases come up often enough to state as worked applications of Rule 4.
Neither is an exclusion: the finding stays in scope and is graded.

- **A finding that requires the REST API bound beyond the loopback default** takes the
  Rule 4 downgrade rather than leaving scope outright. It becomes a public fix only
  once that downgrade reduces it below Low.
- **A finding that requires a deployment exposed or misconfigured beyond what we
  ship** takes the same downgrade where it still carries demonstrated impact. Where the
  misconfiguration is itself the whole finding, it is not a security issue — see the
  exclusions below.

We will state which rules we applied. If you think we applied one incorrectly, say so.

### What "shipped default" means

Classification uses **the software as we ship it**: the release binary or image and
its built-in defaults, plus the current mainnet configuration package from
backup.mainnet.klever.org. The YAML in this repository is the developer checkout; a
setting counts as shipped only if it survives into that package, and where the two
differ, the package governs.

How a given operator invokes the binary — which flags they pass, which ports they
publish, what they put in front of it — is a property of their deployment, not of what
we ship.

The REST API's shipped default is loopback: `common/facade.DefaultRestInterface` is
`localhost:8080`, reachable only from the node host. `--rest-api-interface` is the only
thing that moves the listener off it, and passing that flag is a deployment choice. An
operator who makes it is expected to put authentication and TLS in front of the
listener.

Docker does not rebind the listener on its own, and the two network modes behave
differently:

- Under `--network=host` the container shares the host network namespace, so whatever
  address the node is told to bind is the address it occupies on the host: a loopback
  bind stays on host loopback, and an all-interfaces bind is reachable off-host.
  Published ports (`-p`) have no effect in this mode.
- Under the default bridge network, `-p 8080:8080` forwards to the container's own
  address, which cannot reach a process bound to `127.0.0.1` inside the container.
  Publishing a port therefore exposes only a listener already bound to all interfaces —
  and it publishes it on every host interface by default, ahead of a host firewall.

Wherever the listener ends up, the deployment hardening guide covers what to put in
front of it.

## Scope

This policy covers the core blockchain protocol, consensus, KVM, smart contract
execution, state and account handling, networking, and cryptographic implementations in
this repository.

Findings in other Klever properties — the wallet, the hub, the web properties, and the
SDKs — are outside this policy, but they reach us at the same address:
security@klever.io.

## Fixed in the Open

These are real weaknesses that no third party can turn against someone else, together
with findings that a third party can reach but the downgrade rules reduce below Low. We
want them reported and we do fix them, as ordinary public issues and pull requests on the
normal release schedule, credited in the commit and the release notes. They are not
graded and not published as advisories. Where a public fix would itself show how to
reach a finding that passes the attacker test, we hold it privately instead and credit
it in the release notes when it ships.

- **Missing `secured:` on a read-only diagnostic route that is not remotely
  reachable.** On the loopback default we ship, tightening the flag is defence in depth
  and we do it in the open. **Where a deployment does make such a route remotely
  reachable and it is unsecured, a third party who reaches it and obtains non-public
  node or peer data has gained something** — that passes the attacker test and is graded
  under the information-disclosure note above, subject to the Rule 4 downgrade for
  requiring a non-loopback bind. Routes that change *node state or operator
  configuration* — the redundancy control, the log profile, and anything else that
  reconfigures the process —
  stay in scope as vulnerabilities whatever their reachability. Protocol submission
  endpoints are a separate class: every node accepts signed transactions by design, the
  signature is the authorization, and an unauthenticated `/transaction/broadcast` or
  `/transaction/send` is not a finding.
- **Safety rails around operator misconfiguration and key-file handling.** The
  misconfiguration itself is not a security issue — see below — but making it harder to
  get wrong is worth doing.

## Not a Security Issue

The following are not treated as vulnerabilities and are not published as advisories.
These are rules, not case-by-case judgements. Where the underlying observation is
sound **and demonstrated**, the report is accepted and fixed in the open, and credited;
where it is not, it is closed as not a security issue. A sound observation with no runnable
reproduction is closed under the first rule below, because we cannot confirm it — being
right in principle does not substitute for showing it. We would rather receive these
than not.

Two neighbouring cases are deliberately not on this list. Sound weaknesses that no
third party can turn against someone else are under "Fixed in the Open" above, and
findings that stay in scope but take a reachability downgrade are worked through under
"Downgrade Rules", also above.

- **Reports with no runnable reproduction**, including findings derived from reading
  source code, from static analysis, and from model output, and including automated
  scanner output with no demonstrated exploitability. Theoretical impact is not impact;
  see "Reproducers" above.
- **REST API exposure.** Where the listener is reachable is a property of the
  deployment, and securing the deployment is an operator responsibility. If it is
  reachable off-host, put it behind authentication and TLS (a reverse proxy is the usual
  shape) and do not leave mutate routes unauthenticated. Exposure on its own is not a
  finding.
  Deployment hardening for the API is documented in
  [docs/node-api-hardening.md](docs/node-api-hardening.md); a report that restates that
  document back to us is not a finding.
- **Denial of service against your own node, or that you cannot reproduce.** Load you
  generate against a node you control, and consumption that degrades nothing beyond it,
  is not a finding. A demonstrated remote crash or resource exhaustion of a node the
  reporter does **not** control is a different matter: it passes the attacker test, and
  it is graded by how far it reaches — a single node, or network availability.
- **An operator acting on their own node.** Opening or unsecuring a route in your own
  `api.yaml`, binding the listener where you choose, and then calling it yourself
  crosses no boundary: you already control the node, its configuration, its keys, and
  its database, and you can reach the same result without going through the API at all.
  That is not a finding at any severity. What *is* in scope is a defect that lets
  someone other than the operator reach the same result.
- **Local attackers already present.** If an attacker must already have code execution,
  filesystem write access, or an account on the node host, they can generally do worse
  directly. The bar for these is correspondingly higher.
- **Debug and diagnostic surfaces** that expose only counters, operational telemetry,
  or cached protocol bookkeeping — that is, data that does not identify peers or
  users, carry key material, or reveal unpublished chain state.
- **Credential-hashing strength** where the shipped default fails closed and the
  credential file already sits alongside material of equal or greater sensitivity.
- **Operator misconfiguration or key-file mismanagement**, including losing or failing
  to provision key material.
- **Issues already public.** A vulnerability that is already disclosed in a published
  advisory, a public issue, or a public write-up needs no coordinated disclosure and is
  not reopened as a new report. Duplicates of findings still under embargo, and issues
  already fixed on an unreleased branch, are a different case: they describe real
  findings, they are credited, and they are handled under "Duplicates and Findings
  Already Under Embargo" above. Please check the published advisories and `develop`
  before reporting.
- **Third-party dependency CVEs with no demonstrated exploit path through klever-go** —
  please report to the respective maintainers. If a dependency issue is reachable through
  our code in a way the upstream advisory does not describe, that is in scope.
- **Test code and fixtures.** Test servers and helpers in this repository are not intended
  for production use and are not hardened.
- **Social engineering, phishing, and physical attacks.**
- **Centralization, governance, and economic-design concerns** that do not stem from a
  code defect. These are welcome as ordinary issues or discussions.

## Response

We aim to acknowledge reports that meet the requirements above, and we do not commit to
fixed response times.

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

This table covers findings that pass the attacker test. Findings fixed in the open are
not on it: they ship on the normal release schedule with no embargo.

Reporters are credited in the published advisory unless they ask not to be. Findings
fixed in the open are credited in the commit and the release notes.

## Recognition

We do not operate a bug bounty program and we publish no reward tiers. Reports to this
repository should not be filed on the expectation of payment. We would rather say that
plainly than imply terms we have not set.

What we do commit to:

- **Attribution.** Reporters are credited in the published advisory, or in the commit
  and the release notes for findings fixed in the open, unless you ask us not to be
  named.
- **A stated decision.** Every report receives a bucket and the reasoning behind it.
  Advisory findings also receive a severity, including which downgrade rules we applied
  and why. Where a report fails the attacker test we say which part it fails — no third
  party, or no gain.
- **Safe harbour.** We will not pursue legal action against, or ask platforms to act
  against, anyone who researches and reports in good faith under the responsible
  disclosure guidelines below. If you are unsure whether an activity is covered, ask us
  first at security@klever.io and we will answer before you proceed.

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

## Node Operators

Deployment hardening — REST API exposure, credentials, WebSocket limits, and the
operational checklist — is in [docs/node-api-hardening.md](docs/node-api-hardening.md).

## Security Audits

The codebase undergoes periodic third-party security audits. Reports are available on
request at security@klever.io.

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
