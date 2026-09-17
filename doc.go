// Package tulip_rs_go provides high-performance Go bindings for the tulip_rs
// technical analysis library (https://github.com/me60732/tulip_rs), built on
// the tulip_rs_ffi C ABI via cgo.
//
// The actual library code lives in two packages:
//
//   - github.com/me60732/tulip_rs_go/indicators — one facade value per
//     indicator (indicators.Ema, indicators.Rsi, ...), exposing Info(),
//     MinData(), Indicator(), streaming Batch() state, serialization, and
//     SIMD evaluation (SimdByAssets / SimdByOptions).
//   - github.com/me60732/tulip_rs_go/tulip — shared low-level plumbing:
//     error mapping, indicator metadata, input marshalling, and the
//     memory-ownership wrappers (Result, State, SimdResult) that every
//     indicator package reuses.
//
// # Quick start
//
//	import (
//		"github.com/me60732/tulip_rs_go/indicators"
//		"github.com/me60732/tulip_rs_go/tulip"
//	)
//
//	options := []float64{14.0}
//	res, st, err := indicators.Adx.Indicator(high, low, close, options, []bool{true, true, true})
//	if err != nil {
//		// handle err
//	}
//	defer res.Close()
//	defer st.Close()
//
// See the README at https://github.com/me60732/tulip_rs_go for installation
// (source build vs. prebuilt binary), streaming/persistence, SIMD batch
// evaluation, and candlestick pattern usage.
//
// Full documentation for the wider tulip_rs project:
// https://me60732.github.io/tulip_rs/
package tulip_rs_go
