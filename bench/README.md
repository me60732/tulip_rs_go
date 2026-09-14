# Go Benchmark Suite

This directory contains Go benchmarks that mirror the Python, Rust, and C benchmark suites.

## Purpose

Run performance benchmarks for all tulip.rs indicators using:
- **4 stocks**: BHP_ASX, CBA_ASX, AAPL_NYSE, MSFT_NYSE (6705 bars each)
- **Same option sets** as the Python/Rust benchmarks
- **nanosecond timing methodology** (mirrors Criterion in Rust)

Results are logged to the `indicator_benchmark` Postgres database via `.env`.

## Implementation Type Strings

The benchmark runner writes results with these implementation type strings:
- `"tulip_rs_go"` — native tulip.rs Go bindings
- `"cinar"` — cinar reference (when param-compatible)
- `"tulip_rs_go_simd_by_assets"` — SIMD across assets (same series, same options)
- `"tulip_rs_go_simd_by_options"` — SIMD across option sets (one stock, all options)

## How to Run

```bash
# Copy the environment template
cp .env.example .env

# Edit .env if needed (DATABASE_URL and BENCHMARK_DATABASE_URL)

# Run benchmarks (no DB logging)
BENCHMARK_LOG_TO_DB=0 go run ./cmd/bench

# Or with DB logging enabled
go run ./cmd/bench
```

## How to Add an Indicator

1. Copy the pattern from an existing `bench_*.go` file
2. Ensure options match the Python twin (`tulip-rs-python/bench/tulip_rs_bench/indicators/bench_<name>.py`)
3. Wire `CinarFn` **only if** genuinely param-compatible (same period parameter sweep)
4. For SIMD functions, use the input counts from `indicators/<name>.go`:
   - `SimdByAssets`: `[N][]float64` where N = `<Indicator>Inputs`
   - `SimdOptions`: pass options as `[][]float64` (option sets)

## Audit Status

- **92 bench files** covering 94 indicators (avgprice, dx missing)
- **No duplicate registrations**
- **All CinarFn wires verified** for param compatibility
- `go build ./bench` and `go vet ./bench` pass
