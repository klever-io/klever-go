# Validator CPU Preflight

The validator binary runs a CPU preflight check at startup, immediately before
loading the BLS signing key. When the preflight is enforced (the default,
`preferences.enforceCpuPreflight=true`), a host that fails the gate exits
before the key file is unlocked — defense-in-depth against deploying a
validator key on hardware that cannot keep up with consensus. The
warn-only path (`enforceCpuPreflight=false`) and the emergency env
bypass (`KLEVER_SKIP_CPU_CHECK=1`) deliberately allow startup to
continue past a failure; both log a loud Warn so the bypass is auditable
in fleet logs. The preflight verifies that the host has sufficient SHA-256
hardware acceleration to keep up with consensus and TX processing on a
production network.

## Why this exists

A field investigation across the Klever validator fleet found a ~5× spread in
smart-contract TX processing time — ~600 ms on some validators vs ~120 ms on
peers with otherwise comparable specs. The slow nodes uniformly lacked the
**SHA-NI** instruction set (Skylake-X / Cascade Lake Xeon and earlier never
received it), and the SHA-256 throughput delta correlated with the wall-time
disparity. SHA-NI absence is the most likely contributing cause but was not
conclusively proven to be the sole cause — the consensus log confirms the
protocol's own "leader hardware too weak" detection treats this as a
hardware-class issue regardless of the underlying instruction.

The preflight is grounded in **measured SHA-256 throughput** rather than the
SHA-NI feature flag. A node whose throughput cannot keep leader-mode TX
processing within the protocol's 425 ms hardware-tolerance window is
refused at startup so operators discover the issue before consensus-time
outliers manifest.

## What it checks

On `amd64` and `arm64`:

1. A 200 ms self-bench measures sustained SHA-256 throughput on 16 KiB
   blocks. The result must be at least **800 MB/s** for startup to proceed.
2. Missing SHA-NI (amd64) or ARMv8 SHA2 (arm64) is logged as an
   informational `Warn` line — it is the most common cause of low SHA-256
   throughput, and noting it makes the resulting log actionable. Missing
   the flag never blocks startup on its own; only the bench does.
3. Missing AVX-512 IFMA on `amd64` is logged as a separate `Warn`
   (informational) — it indicates that the BLS pairing path is on the
   ~1.5× slower scalar fallback.

Other architectures are skipped — the preflight is a no-op on `riscv64`,
`386`, `ppc64le`, etc.

## Behavior modes

The preflight has two layers of failure handling, controlled by the
`preferences.enforceCpuPreflight` flag in the validator config.

| Flag value | On preflight failure |
|------------|----------------------|
| `true` (default) | Returns a non-zero exit code with a clear error message. The validator does not start. |
| `false` (escape hatch) | Logs the failure as a `Warn` and continues startup. Useful during a coordinated fleet migration when operators need to observe the issue without bricking running nodes. |

Every preflight run logs a single `Info` line with the measured throughput
(emitted on success, on warn-only failure, and as a precursor to a hard
failure error):

```text
INFO  validator CPU preflight measurement  arch=amd64 sha_accel=true avx512_ifma=true sha256_mbps=1742.3
```

## Override

For emergencies (CI, dev environments, intentional homogeneity tests),
the env var `KLEVER_SKIP_CPU_CHECK=1` bypasses the preflight entirely.
The exact value `1` is required — `true`, `yes`, etc. are not honored
(fail-closed: a typo leaves the preflight active). A loud warning is
logged on every startup so operators see the bypass in their logs:

```text
WARN  validator CPU preflight bypassed via env var  env=KLEVER_SKIP_CPU_CHECK risk=consensus latency may exceed peer median; not for production
```

Do not use this flag in production.

## Migration plan for SHA-NI-deficient hardware

If the preflight refuses to start your validator, migrate to a CPU with
SHA extensions. Note: the validator startup gate is **stricter** than
the standalone `klever-benchmark` tool — startup requires ≥ 800 MB/s
while the benchmark's SHA-256 hard-veto threshold is 500 MB/s. A host
that earns a non-`F` grade from the benchmark can still fail the
startup gate; always run the actual validator binary on a candidate
host before committing to it.

Recommended CPU classes:

- **AMD**: any Zen generation — EPYC Naples, Rome, Milan, Genoa, Turin, or
  Ryzen / Threadripper equivalents.
- **Intel**: Ice Lake-SP (3rd-gen Xeon Scalable) or newer. Skylake-X,
  Cascade Lake, and earlier consumer Skylake / Coffee Lake / Cooper Lake
  parts do not have SHA-NI.
- **ARM**: any ARMv8 chip exposing the SHA2 feature flag (i.e., effectively
  every datacenter ARM CPU since 2018, including AWS Graviton and Apple
  Silicon).

For Hetzner Cloud specifically (based on current fleet observations):
CCX (dedicated AMD EPYC) and CPX (shared AMD EPYC) instances typically
satisfy the preflight. The CX series is a mixed Intel/AMD pool, and
Skylake-class instances within it may not. Cloud SKUs and underlying
hardware can change over time, so always confirm by running
`klever-benchmark --skip-disk --skip-network --skip-kv --skip-memory \
--skip-goroutine --skip-bignum` on a candidate instance before
deploying as a validator.

## Related

- `cmd/node/preflight.go` — the preflight implementation.
- `cmd/benchmark/CLI.md` — the operator-facing benchmark, which applies a
  measured-throughput veto (SHA-256 < 500 MB/s) — not a SHA-NI feature-bit
  check — and produces a more detailed report.
- `config/prefsConfig.go` — `EnforceCPUPreflight` is the runtime flag.
