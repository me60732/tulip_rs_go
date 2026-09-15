# TulipRS Go Bindings

![Go](https://img.shields.io/badge/go-1.22+-blue.svg)
![Rust](https://img.shields.io/badge/rust-nightly-orange.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

High-performance Go bindings for the [TulipRS](https://github.com/me60732/tulip_rs) technical
analysis library, built on the [`tulip_rs_ffi`](https://github.com/me60732/tulip_rs_ffi) C ABI
via cgo. 📖 **[Full documentation for the tulip_rs project](https://me60732.github.io/tulip_rs/)**

## Features

- **94 technical indicators + 77+ candlestick patterns** — moving averages, oscillators,
  trend, volatility, volume, cycle/Ehlers, and single-pass candlestick detection
- **Zero-copy outputs** — `res.Rows` are read-only views into Rust-allocated memory
- **Streaming state** — `Batch()` continues from saved state with no reprocessing;
  states `Clone()` cheaply and `Serialize()` to portable TRFS blobs that resume in any
  tulip_rs language binding
- **SIMD evaluation** — `SimdByAssets` (N assets, one pass) and `SimdByOptions`
  (N option sets, one pass)
- **Generated, never hand-copied** — input/option arities and state ids come from the
  C headers that `build.rs` produces from the Rust core, so they cannot drift

## Installation

### From source (recommended)

Requirements: Rust nightly (pinned by the FFI repo's `rust-toolchain.toml`), a C
toolchain, and Go 1.22+.

```bash
git clone https://github.com/me60732/tulip_rs_go.git
cd tulip_rs_go
./bootstrap.sh --source
```

This clones the FFI repo as a sibling (if not present), then `cargo build`s it with
full `-C target-cpu=native` tuning — LLVM uses every instruction set your CPU
supports. This is the path the benchmarks measure.

### Prebuilt binary (no Rust required)

```bash
./bootstrap.sh --prebuilt   # downloads the GitHub-release cdylib for this
                            # GOOS/GOARCH into ffi/lib/
```

Prebuilds ship portable baselines only (`x86-64-v3` / aarch64 generic — CI CPUs
are not yours, and a native-flags artifact would `SIGILL` at runtime). Windows
gets the staticlib, so cgo links statically with no DLL to ship next to the exe.
Until the first tagged FFI release lands, `--source` is the only mode that works.

After either mode, `go build ./...` works. Everything under `ffi/` is generated
and gitignored; the cgo flags search `ffi/lib/` first, then
`../tulip_rs_ffi/target/{release,debug}`, with matching rpaths (binaries are not
relocatable outside this checkout). `go install` cannot run bootstrap hooks, so
cloning + bootstrap is the supported entry point.

### Verify

```bash
go build ./...
go run ./examples/adx     # expected tail: ALL CHECKS PASSED
```

## Quick start

```go
import (
    "tulip_rs_go/indicators"
    "tulip_rs_go/tulip"
)

options := []float64{14.0}
res, st, err := indicators.Adx.Indicator(high, low, close, options, []bool{true, true, true})
if err != nil { ... }
defer res.Close()
defer st.Close()

fmt.Println(res.Rows[0])              // adx line (zero-copy view, valid until Close)
```

## Advanced usage

### Streaming & persistence

```go
out, err := st.Batch(h2, l2, c2, nil)   // continue from saved state
defer out.Close()

blob, err := st.Serialize(tulip.FormatBincode) // TRFS blob: store anywhere
st2, err := indicators.Adx.DeserializeState(blob) // resume in any language binding
st3, err := st.Clone()                             // or snapshot in-process
```

### SIMD batch evaluation

```go
sim, err := indicators.Adx.SimdByAssets(assets, options, nil)  // N assets, one pass
defer sim.Close()

sim2, err := indicators.Adx.SimdByOptions(h, l, c,
    [][]float64{{3}, {5}, {7}, {10}}, nil)                    // N option sets, one pass
defer sim2.Close()
```

### Candlestick patterns

```go
cres, err := indicators.Candlestick.Indicator(open, high, low, close, options,
    indicators.ForecastNone) // or filter to one forecast class
defer cres.Close()

cres.Patterns(bar)  // pattern ids detected on bar `bar` (CSR-packed, zero-copy)
cres.Names(bar)     // same, as pattern names
```

## Indicators

Every indicator is a facade value in the `indicators` package (`indicators.Ema`,
`indicators.Rsi`, …) exposing `Info()`, `MinData()`, `Indicator()`, `Batch()` state,
serialization, and — where the core supports it — both SIMD modes. By family:

| Family | Indicators |
|---|---|
| Moving averages | sma, ema, wma, dema, tema, trima, hma, zlema, kama, vidya, vwma, wilders, smaenvelope |
| Oscillators | rsi, macd, stoch, stochrsi, willr, cci, cmo, ultosc, ao, fisher, fosc, msw, trix |
| Trend | adx, adxr, di, dm, dx, aroon, aroonosc, psar, ppo, apo, vortex, elderray, donchianchannel, ichimoku, supertrend, ef, mama |
| Volatility | bbands, atr, natr, tr, stddev, volatility, vhf, cvi, chandelierexit, keltnerchannel, trvi |
| Volume | ad, adosc, obv, mfi, nvi, pvi, vosc, kvo, emv, wad, marketfi, chaikinmf, vwap |
| Price & statistical | avgprice, medprice, typprice, wcprice, max, min, mom, roc, rocr, bop, linreg, tsf, dpo, mass, md, qstick, pivotpoint |
| Cycle & Ehlers | cybercycle, adaptivemsw, homodynediscriminator, instantaneoustrendline, trendmode, highpass, hilberttransform, roofingfilter, supersmoother, ccfisher |
| Candlestick | 77+ classical patterns, single-pass, per-bar match lists with names, Japanese names, bar counts, and forecast types |

The `examples/` directory has a runnable demo per indicator mirroring the C
examples (series generation, NaN-safe comparison against reference values,
full/streaming/SIMD checks, `ALL CHECKS PASSED`).

## Benchmarks

A nanosecond-precision benchmark suite lives in [`bench/`](bench/) and mirrors the
C, Python, and Rust harnesses (same tickers, option sets, and timing methodology).
It compares tulip_rs_go against pure-Go reference bindings (cinar) and logs to a
shared Postgres DB. Run:

```bash
cd bench && go run ../cmd/bench
```

## Layout

```
tulip_rs_go/
  go.mod
  bootstrap.sh      one-shot installer: --source or --prebuilt native lib
  tulip/            shared plumbing: errors, metadata, input marshalling,
                    Result / State / SimdResult ownership wrappers
  indicators/       one file per indicator (94 + candlestick + helpers), all
                    in one package, namespaced behind facade values
  ffi/              GENERATED & gitignored: headers + prebuilt lib land here
  examples/<name>/  per-indicator end-to-end demo (mirrors <name>_example.c)
  cmd/bench/        benchmark runner entry point
  bench/            indicator benchmark suite
```

## License

MIT — same as the rest of the tulip_rs project family.

## Credits

- Core: [tulip_rs](https://github.com/me60732/tulip_rs) (Rust)
- C ABI: [tulip_rs_ffi](https://github.com/me60732/tulip_rs_ffi)
- Inspired by the original [Tulip Indicators](https://tulipindicators.org/) library
