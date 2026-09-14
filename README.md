# tulip-rs-go

Go bindings for [tulip_rs](https://github.com/me60732/tulip_rs) technical
indicators, built on the `tulip_rs_ffi` C ABI via cgo.

Zero-copy indicator outputs (read-only views into Rust memory), streaming
state with cheap clone + opaque TRFS blobs for persistence, SIMD by-assets /
by-options evaluation, and automatic (but explicit-first) memory ownership in
idiomatic Go: everything the user gets from the C examples is here too.

See `../bindings_memory_model.md` for the ownership model this binding
implements — the short version: outputs are views valid until `Close`, every
handle's `Close` is idempotent, finalizers are leak nets not mechanisms, and
`defer res.Close()` / `defer st.Close()` is the way.

## Layout

```
tulip_rs_go/
  go.mod
  tulip/       shared plumbing: errors, metadata, input marshalling,
               Result / State / SimdResult ownership wrappers
  adx/         ADX indicator (the pattern all indicator packages follow)
  examples/adx end-to-end runnable demo (mirrors adx_example.c)
```

## Prerequisites

1. Rust nightly (pinned by the FFI repo's toolchain file) and a C toolchain.
2. The shared library built:

   ```bash
   cd ../tulip_rs_ffi
   cargo build --release   # or `cargo build` for a debug build
   ```

   The cgo LDFLAGS search `../tulip_rs_ffi/target/release` first and fall
   back to `target/debug`, with matching rpaths, so a local build just works;
   binaries built here are not relocatable outside this checkout (dev-mode
   caveat — revisit when packaging for distribution).

## Build & run

```bash
cd tulip_rs_go
go build ./...
go run ./examples/adx
```

Expected tail: `ALL CHECKS PASSED`.

## Usage sketch

```go
import (
    "tulip_rs_go/adx"
    "tulip_rs_go/tulip"
)

options := []float64{14.0}
res, st, err := adx.Indicator(high, low, close, options, []bool{true, true, true})
if err != nil { ... }
defer res.Close()
defer st.Close()

fmt.Println(res.Rows[0])           // adx line (zero-copy view, valid until Close)

out, err := st.Batch(h2, l2, c2, nil) // stream new bars
defer out.Close()

blob, err := st.Serialize(tulip.FormatBincode) // TRFS blob: store anywhere
st2, err := adx.DeserializeState(blob)         // resume in any language binding
st3, err := st.Clone()                          // or snapshot in-process

sim, err := adx.SimdByAssets(assets, options, nil) // N assets, one pass
defer sim.Close()                                  // frees lane states first

sim2, err := adx.SimdByOptions(h, l, c, [][]float64{{3}, {5}, {7}, {10}}, nil)
defer sim2.Close()
```

## Status

- `adx` is complete and verified; the other indicator packages will follow
  the same template (per-indicator package over the shared `tulip` core).
