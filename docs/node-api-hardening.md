# Node API Hardening

Operator-facing guidance for deploying a Klever node's REST API. Intake policy for
security reports lives in [SECURITY.md](../SECURITY.md).

## Security Best Practices for Node Operators

- Keep node software up to date
- Run the current mainnet configuration package from backup.mainnet.klever.org and
  refresh it when a release says to; the YAML in this repository is a developer checkout
- Bind the REST API to loopback — `--rest-api-interface=localhost:8080` — unless
  something off-host genuinely needs it
- Read the `--rest-api-interface` value your node actually starts with before assuming
  where it listens. The flag overrides the built-in `localhost:8080`, so the default
  tells you nothing once it is set.
- A listener reachable off-host requires authentication and TLS termination in front of
  it. Do not leave routes that reconfigure the node unauthenticated.
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

The API listens wherever `--rest-api-interface` points it. The built-in default is
`localhost:8080` (`DefaultRestInterface`, `common/facade/nodeFacade.go`), reachable only from the
node host, but the flag overrides that and a non-loopback value puts the API on the network.
**Check the value your node is started with, and treat the API as exposed until you have
confirmed it is bound to loopback.** The node does not second-guess either choice.

An exposed API MUST be fronted by a reverse proxy that terminates TLS, enforces
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

**This command is only correct for `hasher.type: sha256`.** `NewHasher` also accepts `keccak` and
`blake2b` (`crypto/hashing/factory/hasherFactory.go`); if you select either, generate the digest
with that algorithm instead. A digest produced by the wrong algorithm parses fine and then fails
every authenticated request.

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
  node:
    routes:
      - name: /debug
        open: true
        secured: true       # or open: false to remove the route entirely
      - name: /peerinfo
        open: true
        secured: true       # peer addresses and validator public keys
      - name: /p2pstatus
        open: true
        secured: true       # the node's own p2p listen addresses
      - name: /heartbeatstatus
        open: true
        secured: true       # heartbeat view of the validator set

credentials:
  - username: <operator>
    password: <hex sha256 digest of the password>
hasher:
  type: sha256
```

**This edits the `log`, `subscribe` and `node` entries of the shipped file — it is not a
replacement for the whole file.** The four `node` routes are the ones an `api.yaml` written before
they shipped secured will still serve open, which is why they are listed here rather than left to
the defaults. The real `apiPackages` block also carries the other route groups (`address`,
`transaction`, `block`, `node`, `vm`, …); dropping them leaves `apiPackages` without those keys, and
every route whose group is missing fails its enabled check and is never registered. The same applies
to the indentation: `log`/`subscribe` must stay nested under `apiPackages`, while `credentials` and
`hasher` stay at the top level. Get that wrong and the config parses without error while silently
discarding the credentials, which lands you in the HTTP 500 state described above.

Pair it with `--rest-api-interface=localhost:8080` plus a reverse proxy, or a firewall rule
restricting the port to the proxy host.

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
connects and restored when the last one disconnects, so two overlapping sessions do not revert
the node to each other's setting. Because the restore happens at the last disconnect, reapply the
intended profile, or restart the tailer set, after a verbose debugging session.

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

The two WebSocket routes take different stances on origin:

- **`/log` enforces an origin allowlist.** A client that sends no `Origin` header (the log viewer,
  `curl`, `wscat`) is always allowed — `Origin` is set by the browser and page script cannot forge
  it, so its absence means no page is driving the connection. A request that *does* carry an
  `Origin` is a browser, and is admitted only if `logWebSocketAllowedOrigins` lists it. The empty
  default therefore blocks every web page while leaving normal tooling working. This matters
  because `/log` can be Basic-Auth protected: without it, any site an operator visits could open
  `ws://localhost:8080/log` and stream node logs on their credentials.
- **`/subscribe` does not enforce origin**: the node is expected to run headless behind an
  operator proxy that owns origin/CORS policy. **A secured `/subscribe` requires origin
  enforcement at that proxy.** Either enforce `Origin` there, or leave `/subscribe` unsecured
  and treat its feed as public.

### Seednode `/log`

The seednode's `/log` route (`cmd/seednode/api/api.go`) shares the sender with the node, so the
handshake limit and deadline, the rolling `pongWait` deadline, the ping loop, the write deadline,
the profile refcount, the log-injection guard and the panic containment all apply, and its
upgrader blocks every browser origin (there is no allowlist to configure, so no browser can open
it at all). That matters because the seednode ships `/log` with `secured: true`
(GHSA-9v8p-frvj-2pcm), and `secured` also turns profile application on: without the
origin check, a page an operator visited could stream seednode logs on cached Basic credentials
and mute the process-global logger.

Live seednode `/log` connections are capped at a built-in 32, node-wide, with no per-IP dimension
and no configuration knob: the seednode has no `webServer` antiflood section to read one from,
and every live session registers a process-global observer that formats every log line, so
unbounded is the wrong default for a route nobody needs tens of. Rejected upgrades are budgeted
the same way as the node's, one line per window with a counter of their own. Keep the route
disabled unless you are actively tailing it.

### Operational checklist

- [ ] `--rest-api-interface` read off the running node, not assumed from the default
- [ ] API bound to `localhost` unless something off-host needs it
- [ ] If reachable off-host: reverse proxy enforcing TLS, origin/CORS, and authentication
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
