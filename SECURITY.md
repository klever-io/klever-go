# Security Policy

## Overview

The Klever blockchain team takes security vulnerabilities seriously. We appreciate your efforts to responsibly disclose your findings and will make every effort to acknowledge your contributions.

## Supported Versions

We actively support and provide security updates for the following versions:

| Version | Supported          |
| ------- | ------------------ |
| 1.7.x   | :white_check_mark: |
| < 1.7.0 | :x:                |

**Note:** We strongly recommend using the latest stable release to ensure you have the most recent security patches and improvements.

## Reporting a Vulnerability

**Please do NOT report security vulnerabilities through public GitHub issues, discussions, or pull requests.**

Instead, please report security vulnerabilities using one of the following methods:

### Private Security Advisory (Recommended)

Report vulnerabilities through GitHub's private vulnerability reporting:
1. Navigate to the **Security** tab of this repository
2. Click **Report a vulnerability**
3. Fill out the vulnerability details form

### Email

Send details to: **security@klever.org**

Please include the following information in your report:

- **Type of vulnerability** (e.g., consensus failure, smart contract execution bypass, DoS, etc.)
- **Affected component(s)** (e.g., KVM, consensus mechanism, networking layer)
- **Step-by-step instructions** to reproduce the issue
- **Proof of concept** or exploit code (if available)
- **Potential impact** of the vulnerability
- **Suggested mitigation** (if you have one)
- **Your contact information** for follow-up questions

## Vulnerability Severity Classification

We use the following severity levels to classify security issues:

### Critical
- Consensus failures or chain halts (BLS-based slot consensus with Byzantine fault tolerance)
- Unauthorized fund access or theft
- Remote code execution
- Private key exposure
- Byzantine attacks affecting consensus integrity

### High
- Denial of Service affecting network availability
- Smart contract execution vulnerabilities
- Authentication/authorization bypass
- Transaction validation bypass

### Medium
- Information disclosure
- Performance degradation attacks
- Non-critical DoS vectors

### Low
- Issues with limited impact
- Best practice violations
- Security improvements

## Response Timeline

We are committed to addressing security vulnerabilities promptly:

1. **Initial Response**: Within 48 hours of receiving your report
2. **Triage and Assessment**: Within 5 business days
3. **Fix Development**: Depending on complexity and severity
   - Critical: 7-14 days
   - High: 14-30 days
   - Medium: 30-60 days
   - Low: 60-90 days
4. **Coordinated Disclosure**: We will work with you to determine an appropriate disclosure timeline

## Security Update Process

When a security vulnerability is confirmed:

1. We will develop and test a fix
2. We will prepare security advisories
3. We will notify affected users and node operators through official channels
4. We will release the patched version
5. After a reasonable adoption period, we will publish the security advisory with credit to the reporter (if desired)

## Bug Bounty Program

We value the security research community's contributions. Details about our bug bounty program:

- **Scope**: Vulnerabilities in the core blockchain protocol, consensus mechanism, KVM, smart contract execution, and cryptographic implementations
- **Rewards**: Determined based on severity and impact (see classification above)
- **Eligibility**: Must follow responsible disclosure practices

For current bounty amounts and specific program details, please contact **security@klever.org**.

## Out of Scope

The following are generally considered out of scope:

- Issues in third-party dependencies (please report to the respective maintainers)
- Social engineering attacks
- Physical attacks on infrastructure
- Vulnerabilities requiring unlikely user interaction
- Issues already reported or fixed
- Automated scanning results without proof of exploitability

## Responsible Disclosure Guidelines

When researching vulnerabilities, please:

- ✅ Make every effort to avoid privacy violations, data destruction, and service disruption
- ✅ Only interact with accounts you own or have explicit permission to test
- ✅ Do not exploit vulnerabilities beyond what is necessary to demonstrate the issue
- ✅ Keep all vulnerability details confidential until they are resolved
- ✅ Give us reasonable time to fix vulnerabilities before public disclosure

Please **do not**:

- ❌ Access, modify, or delete data that doesn't belong to you
- ❌ Perform DoS/DDoS attacks on the mainnet or public testnet
- ❌ Compromise user privacy or degrade user experience
- ❌ Execute attacks against network participants
- ❌ Publicly disclose vulnerabilities before coordinated release

## Security Best Practices for Users

To help secure the Klever blockchain ecosystem:

- Keep your node software up to date
- Follow secure key management practices
- Use hardware wallets for significant holdings
- Verify transaction details before signing
- Be cautious of social engineering attempts
- Report suspicious activity to the team

## Deploying / Exposing the REST API

The node's REST and WebSocket API performs **no origin checking**, and applies access control only
where a route is explicitly marked `secured` in `api.yaml`. Whether it is safe is entirely a function
of how you deploy it. This section is the operator-facing counterpart to the user guidance above, and
covers both deployables: the validator/observer node (`config/node/`) and the seednode
(`config/seednode/`), which ship separate API configurations.

### The exposure model

By default the API binds to `localhost:8080` (`DefaultRestInterface`, `common/facade/nodeFacade.go`),
reachable only from the node host. **Exposing it beyond that is a deliberate operator choice, and the
node does not second-guess it.**

If you expose the API, it MUST be fronted by a reverse proxy that terminates TLS, enforces
origin/CORS policy, and requires authentication. Firewall the API port so the node is reachable only
through that proxy. The node itself will not reject a cross-origin request: `CheckOrigin` returns
`true` unconditionally at all three WebSocket entry points
(`network/api/websocket/routes.go`, `network/api/api.go`, `cmd/seednode/api/api.go`).
That is by design — origin policy belongs to the proxy — but it means an exposed node with no proxy
has no origin protection at all.

**Do not run a browser on a validator host.** Because the API has no origin control, any page you
visit can issue cross-origin requests to `localhost:8080` and reach the node.

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

**`/node/peerinfo`, `/node/p2pstatus`** — both ship enabled and **unauthenticated** (`open: true`, no
`secured`). `/node/peerinfo` returns the addresses and validator public keys of every peer you are
connected to; the `pid` query parameter only filters that list, and omitting it returns all of them.
`/node/p2pstatus` reports the node's own p2p listen addresses. Together they describe your network
topology. Set `secured: true`, or `open: false`, unless you intend that data to be public.

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

`/subscribe` connection and subscription limits are tunable under `webServer` in
`config/node/config.yaml`:

| Setting | Purpose | `0` means |
|---|---|---|
| `webSocketConnections` | node-wide cap on live connections | unlimited |
| `webSocketConnectionsPerIP` | per-source-IP cap | unlimited |
| `webSocketMaxAddressesPerSubscribe` | addresses accepted in one subscribe call | use the built-in default |
| `webSocketMaxAddressesPerClient` | total addresses one connection may watch | use the built-in default |

Note the split in the last column. Only the two connection caps treat `0` as unlimited. The two
address caps fall back to their built-in defaults on any non-positive value, so they **cannot be
disabled** — to lift them, set an explicit high value rather than `0`.

**Behind a reverse proxy, every client shares the proxy's IP**, so `webSocketConnectionsPerIP` will
throttle all of them together. Raise it, or set it to `0` to disable, for proxied deployments — and
enforce per-client limits at the proxy instead.

Note that the HTTP throttlers (`simultaneousRequests`, `sameSourceRequests`) release their slot at
the HTTP-to-WebSocket upgrade, so they do not bound live WebSocket connections. The `webSocket*`
settings are what do.

### Operational checklist

- [ ] API bound to `localhost` unless deliberately exposed
- [ ] If exposed: reverse proxy enforcing TLS, origin/CORS, and authentication
- [ ] API port firewalled to the proxy host
- [ ] Real credentials configured; all placeholder entries replaced, in both `config/node/api.yaml`
      and `config/seednode/api.yaml` if you run a seednode
- [ ] `/log` secured or disabled
- [ ] `/subscribe` secured or disabled if not intended to be public
- [ ] `/node/peerinfo` and `/node/p2pstatus` disabled or secured — they expose network topology
- [ ] Seednode `/peers` disabled or secured unless network topology is meant to be public
- [ ] `--profile-mode` off, or API localhost-only — `/debug/pprof` is unauthenticated
- [ ] `/swagger` blocked at the proxy if you do not want the API surface enumerated
- [ ] `webSocketConnectionsPerIP` adjusted if behind a proxy
- [ ] No browser running on validator hosts
- [ ] Node software kept up to date
- [ ] Key management per the practices above

## Security Audits

Our codebase undergoes regular security audits by reputable third-party firms. Audit reports are published on our website and documentation.

## Contact

For any security-related questions or concerns:

- **Email**: security@klever.org
- **Website**: https://klever.org
- **Documentation**: https://docs.klever.org

## Acknowledgments

We would like to thank the security researchers and community members who help keep Klever safe. Contributors who follow responsible disclosure practices will be acknowledged (with permission) in our security advisories.

---

**Last Updated**: August 2026
