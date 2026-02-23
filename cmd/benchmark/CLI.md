
# Benchmark CLI

The **Klever Benchmark Tool** exposes the following Command Line Interface:

```text
$ benchmark --help

NAME:
   Klever Benchmark Tool - Measures hardware performance for Kleverchain validator nodes
                           across six categories and produces a pass/fail verdict and
                           a 0–1000 point score with a letter grade (S/A/B/C/D/F).

USAGE:
   benchmark [global options]

AUTHOR:
   KleverIO <contact@klever.org>

GLOBAL OPTIONS:

   -- Goroutine benchmark --
   --goroutines value    Maximum number of concurrent goroutines to test (default: NumCPU×4)
   --duration value      Duration in seconds for each concurrency level (default: 3)

   -- Disk I/O benchmark --
   --disk-dir value      Directory for disk I/O test files (default: auto temp dir next to binary)
   --disk-size value     Sequential test file size in MB (default: 256)

   -- Control --
   --skip-goroutine      Skip goroutine / CPU scalability benchmark
   --skip-disk           Skip disk I/O benchmark
   --skip-network        Skip network TCP loopback benchmark
   --skip-kv             Skip KV store benchmark
   --skip-memory         Skip memory bandwidth and latency benchmark
   --skip-bignum         Skip big-number / FPU benchmark

   --output value        Output format: text or json (default: "text")
   --verbose             Enable verbose logging
   --version             Print version and exit
   --help, -h            Show help
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
| Goroutine (CPU) | 200 |
| Disk I/O | 200 |
| KV Store | 200 |
| Network | 150 |
| Memory | 150 |
| BigNum / FPU | 100 |

| Grade | % of enabled max | Description |
|-------|-----------------|-------------|
| **S** | ≥ 90 % | Elite — top-tier validator hardware |
| **A** | ≥ 75 % | Excellent — production-ready for high-traffic networks |
| **B** | ≥ 60 % | Good — suitable for standard validator operation |
| **C** | ≥ 45 % | Acceptable — meets minimum validator requirements |
| **D** | ≥ 30 % | Marginal — several metrics below recommended levels |
| **F** | < 30 % | Insufficient — does not meet validator requirements |

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

