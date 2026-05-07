
# Benchmark CLI

The **Klever Benchmark Tool** exposes the following Command Line Interface:

```text
$ benchmark -help

Usage of benchmark:
  -goroutines int     Maximum number of concurrent goroutines to test (default: NumCPU×4)
  -duration int       Duration in seconds for each concurrency level (default: 3)
  -disk-dir string    Directory for disk I/O test files (default: auto temp dir next to binary)
  -disk-size int      Sequential test file size in MB (default: 256)
  -skip-goroutine     Skip goroutine / CPU scalability benchmark
  -skip-disk          Skip disk I/O benchmark
  -skip-network       Skip network TCP loopback benchmark
  -skip-kv            Skip KV store benchmark
  -skip-memory        Skip memory bandwidth and latency benchmark
  -skip-bignum        Skip big-number / FPU benchmark
  -skip-crypto        Skip crypto benchmark (SHA-256/Blake2b/Keccak/Ed25519)
  -output string      Output format: text or json (default: "text")
  -verbose            Enable verbose logging
  -version            Print version and exit
```

---

## Benchmarks

| Category | What it measures | Key metrics |
|----------|-----------------|-------------|
| **Goroutine** | CPU scalability via parallel SHA-256 hashing | Efficiency at NumCPU workers |
| **Disk I/O** | Sequential and random read/write throughput | MB/s, IOPS |
| **Network** | TCP loopback latency and streaming throughput | P50/P99 µs, MB/s |
| **KV Store** | In-memory state-access patterns (80/20 read-write) | ops/s |
| **Memory** | DRAM bandwidth, random latency, allocator speed | GB/s, ns, M allocs/s |
| **BigNum / FPU** | 2048-bit modexp/modmul and float64 transcendentals | ops/s |
| **Crypto / Hashing** | SHA-256 / Blake2b / Keccak-256 / Ed25519 throughput + CPU feature flags | MB/s, ops/s |

---

## Verdicts

Each metric is evaluated against two thresholds:

| Icon | Label | Meaning |
|------|-------|---------|
| `[OK]` | PASS | Meets production validator requirements |
| `[!!]` | WARN | Minimum requirements met but below recommended |
| `[XX]` | FAIL | Does not meet validator requirements |

---

## Scoring

Each category contributes weighted points (total: 1,000). Scores are computed via linear
interpolation between a *fail floor* (0 pts) and an *excellent ceiling* (100 pts) per metric.
Skipped categories are excluded from the denominator so the grade stays fair.

| Category | Weight |
|----------|--------|
| Disk I/O | 200 |
| KV Store | 200 |
| Crypto / Hashing | 200 |
| Goroutine (CPU) | 150 |
| Network | 100 |
| Memory | 100 |
| BigNum / FPU | 50 |

| Grade | % of enabled max | Description |
|-------|-----------------|-------------|
| **S** | ≥ 90 % | Elite — top-tier validator hardware |
| **A** | ≥ 75 % | Excellent — production-ready for high-traffic networks |
| **B** | ≥ 60 % | Good — suitable for standard validator operation |
| **C** | ≥ 45 % | Acceptable — meets minimum validator requirements; consider a hardware upgrade |
| **D** | ≥ 30 % | Marginal — several metrics below recommended levels |
| **F** | < 30 % | Insufficient — does not meet validator requirements |

### Hard veto: SHA-256 throughput floor

Klever's TX hot path hashes SHA-256 per-TX, per-header, and per-state-entry.
The protocol tolerates some hardware variance — a leader has a 500 ms
baseline timeout with a 425 ms lower bound below which validators
attribute leader failure to weak hardware. To prevent operators from
deploying nodes that cannot consistently process TXs as leader within
that window, the Crypto category applies a **hard veto** on the measured
SHA-256 throughput: hosts whose 16 KiB SHA-256 throughput is below
**500 MB/s** have their overall grade capped at **F** regardless of total
points.

The veto is grounded in the measurement, not in any specific CPU feature
flag. SHA-NI absence is the most common cause of low throughput in
practice (Skylake-X / Cascade Lake / Haswell on amd64), and the report
calls this out informationally. The text report highlights the reason;
the JSON report sets `score.grade = "F"`, populates `score.vetoed=true`
and `score.veto_reason`, and exposes the underlying CPU flags under
`crypto.cpu_features`.

**Note on validator startup:** the `klever-node` validator binary applies
a *stricter* CPU preflight at startup, requiring ≥ 800 MB/s on a 200 ms
self-bench (vs the 500 MB/s benchmark veto). A host can pass this
benchmark with a non-`F` grade and still be refused by validator
startup. See `cmd/node/PREFLIGHT.md` for the rationale.

---

## Examples

```bash
# Run all benchmarks with defaults
./benchmark

# Run only disk and network benchmarks
./benchmark --skip-goroutine --skip-kv --skip-memory --skip-bignum

# Increase goroutine concurrency and duration
./benchmark --goroutines 128 --duration 5

# Use a specific disk (e.g. a mounted NVMe)
./benchmark --disk-dir /mnt/nvme0 --disk-size 512

# Output as JSON and pipe to jq
./benchmark --output json | jq '.score'

# Save JSON report to file
./benchmark --output json > report-$(hostname)-$(date +%Y%m%d).json
```

