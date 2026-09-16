// Package indicators holds the Go bindings for every tulip_rs indicator,
// one file per indicator, mirroring the FFI crate's own layout (one crate,
// N indicator modules).
//
// Each indicator is namespaced behind an exported facade value so the whole
// family shares one import:
//
//	import "github.com/me60732/tulip_rs_go/indicators"
//
//	res, st, err := indicators.Adx.Indicator(high, low, close, opts, nil)
//	defer res.Close()
//	defer st.Close()
//
// The FFI surface exposed per indicator mirrors the C API completely:
// Info, MinData, Indicator, State.Batch, Serialize/DeserializeState/Clone,
// SimdByAssets, SimdByOptions, plus <X>Inputs/<X>Options arity constants
// and the <X>ID serialisation id — all read from generated headers, never
// hand-copied.
//
// Memory model (repo-root bindings_memory_model.md): outputs are zero-copy
// read-only views valid until the owning wrapper's Close; Result/State/
// SimdResult close idempotently and carry finalizer leak nets.
package indicators

/*
#include <stdlib.h>
*/
import "C"

import "unsafe"

// optionArg returns a non-NULL pointer to the options array for the FFI.
// Zero-option indicators must still not receive NULL (the C side reads an
// empty [f64; 0] through it), so empty options are padded with one
// harmless slot. Callers must runtime.KeepAlive the returned slice across
// the cgo call.
func optionArg(options []float64) (*C.double, []float64) {
	pad := options
	if len(pad) == 0 {
		pad = []float64{0}
	}
	return (*C.double)(unsafe.Pointer(&pad[0])), pad
}

// padSets guarantees every SIMD option set has at least one slot, so
// zero-option indicators still pass a non-NULL array to the FFI. For
// option counts >= 1 it is the identity; n is the indicator's Options arity.
func padSets(sets [][]float64, n int) [][]float64 {
	if n > 0 {
		return sets
	}
	out := make([][]float64, len(sets))
	for i := range out {
		out[i] = []float64{0}
	}
	return out
}
