// Package adx provides Go bindings for the ADX (Average Directional Index)
// indicator, built on the tulip_rs_ffi C API via the shared tulip package.
//
// It mirrors the full FFI surface for ADX:
//
//	Inputs, Options   — series/option arities (from the generated C defines)
//	ID                — the CIndicatorId used by the state-serialisation FFI
//	Info()            — indicator metadata (Go copies, nothing to free)
//	MinData(options)  — minimum bars needed to produce any output
//	Indicator(...)    — full or partial compute; returns outputs + state
//	State.Batch(...)  — stream new bars without reprocessing history
//	State.Serialize/DeserializeState/Clone — TRFS blob persistence + snapshot
//	SimdByAssets(...) — N assets in one CPU pass
//	SimdByOptions(...)- N option sets in one CPU pass
//
// Memory model: outputs are zero-copy read-only views into Rust memory,
// valid until the owning wrapper's Close; every wrapper (Result, State,
// SimdResult) must be closed exactly once (defer it) and carries a
// finalizer as a leak net. See the repo-root bindings_memory_model.md.
package adx

/*
#cgo CFLAGS: -I${SRCDIR}/../../tulip_rs_ffi/include
#include <stdbool.h>
#include <stdint.h>
// cgo prologue defines a `CBytes` helper; rename the FFI typedef to avoid
// the clash (must match the rename in the tulip package's preamble).
#define CBytes TulipBytes
#include "tulip_rs_ffi.h"

static inline void *u2p(uintptr_t p) { return (void *)p; }
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"tulip_rs_go/tulip"
)

// Indicator arities, read from the generated C defines so they can never
// drift from the core crate (see tulip_rs_ffi/build.rs).
const (
	Inputs  = int(C.ADX_INPUTS)  // high, low, close
	Options = int(C.ADX_OPTIONS) // period
)

// ID is the FFI indicator id for state serialisation: fnv1a32("adx"),
// defined in the generated tulip_rs_ffi_state_ids.h. Never recompute.
var ID = uint32(C.C_INDICATOR_ID_ADX)

// State is a live ADX streaming-state handle: the state returned by
// Indicator, resumable via Batch and closeable exactly once via Close.
type State struct{ *tulip.State }

func freeState(h uintptr) { C.adx_state_free(C.u2p(C.uintptr_t(h))) }

func wrapState(h uintptr) *State {
	return &State{tulip.NewState(ID, h, freeState)}
}

// optionArg returns a non-NULL pointer to the options array. The FFI
// always receives a live pointer (even 0-option indicators must not get
// NULL), so empty options are padded with one harmless slot. The caller
// must runtime.KeepAlive the returned slice across the cgo call.
func optionArg(options []float64) (*C.double, []float64) {
	pad := options
	if len(pad) == 0 {
		pad = []float64{0}
	}
	return (*C.double)(unsafe.Pointer(&pad[0])), pad
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func Info() tulip.Info {
	ci := C.adx_info()
	return tulip.NewInfo(
		int32(ci.indicator_type),
		C.GoString(ci.name),
		C.GoString(ci.full_name),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.inputs.ptr)), int(ci.inputs.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.options.ptr)), int(ci.options.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.outputs.ptr)), int(ci.outputs.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.optional_outputs.ptr)), int(ci.optional_outputs.len)),
	)
}

// MinData returns the minimum number of bars ADX needs to produce any
// output for the given options ([period]).
func MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.adx_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes ADX over the given bars.
//
// Outputs: with optionalOutputs empty/nil, result.Rows is just [adx]; with
// three flags, Rows is [adx, dx, atr, tr] (rows may start with NaN during
// warmup). The returned State is live streaming state and must be closed
// when done; closing the Result does NOT free it (and vice versa).
func Indicator(high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *State, error) {
	if len(options) != Options {
		return nil, nil, fmt.Errorf("adx: Indicator needs %d option(s) [period], got %d", Options, len(options))
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("adx: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.adx_indicator(
		(**C.double)(arr),
		C.size_t(len(high)),
		opts,
		(*C.bool)(optArr),
		C.size_t(len(optionalOutputs)),
	)
	res, err := tulip.NewResult(
		int32(raw.error),
		unsafe.Pointer(raw.outputs),
		unsafe.Pointer(raw.output_lens),
		int(raw.num_outputs),
		uintptr(raw.state),
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, nil, fmt.Errorf("adx: %w", err)
	}
	return res, wrapState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars from this state. Inputs need not
// re-send history; outputs follow the same shape rules as Indicator.
func (s *State) Batch(high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.adx_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(high)),
		(*C.bool)(optArr),
		C.size_t(len(optionalOutputs)),
	)
	res, err := tulip.NewBatchResult(
		int32(raw.error),
		unsafe.Pointer(raw.outputs),
		unsafe.Pointer(raw.output_lens),
		int(raw.num_outputs),
	)
	runtime.KeepAlive(series)
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *State) Clone() (*State, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &State{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func DeserializeState(blob []byte) (*State, error) {
	st, err := tulip.DeserializeState(ID, blob, freeState)
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	return &State{st}, nil
}

// SimdByAssets computes ADX for N assets in a single CPU pass.
//
// Each asset carries the same [Inputs] series (high, low, close) of equal
// length; all assets must share the same options. Results/States are
// indexed by lane. The SimdResult owns all lane states — Close it, not the
// individual States.
func SimdByAssets(assets [][Inputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != Options {
		return nil, fmt.Errorf("adx: SimdByAssets needs %d option(s) [period], got %d", Options, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.adx_simd_by_assets(
		(***C.double)(outer),
		C.size_t(len(assets)),
		C.size_t(len(assets[0][0])),
		opts,
		(*C.bool)(optArr),
		C.size_t(len(optionalOutputs)),
	)
	res, err := tulip.NewSimdResult(
		int32(raw.error),
		unsafe.Pointer(raw.outputs),
		unsafe.Pointer(raw.output_lens),
		unsafe.Pointer(raw.states),
		int(raw.num_results),
		int(raw.num_outputs),
		ID, freeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	return res, nil
}

// SimdByOptions computes ADX for N option sets over one shared series of
// bars in a single CPU pass (e.g. several periods at once). Each option set
// has [Options] values ([period]); lanes need not relate to each other.
// Each lane's resumable state lives in SimdResult.States.
func SimdByOptions(high, low, close []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("adx: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != Options {
			return nil, fmt.Errorf("adx: option set %d has %d values, need %d", i, len(o), Options)
		}
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	defer release()
	sets := padSets(optionSets)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.adx_simd_by_options(
		(**C.double)(arr),
		C.size_t(len(high)),
		(**C.double)(optsArr),
		C.size_t(len(optionSets)),
		(*C.bool)(optArr),
		C.size_t(len(optionalOutputs)),
	)
	res, err := tulip.NewSimdResult(
		int32(raw.error),
		unsafe.Pointer(raw.outputs),
		unsafe.Pointer(raw.output_lens),
		unsafe.Pointer(raw.states),
		int(raw.num_results),
		int(raw.num_outputs),
		ID, freeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("adx: %w", err)
	}
	return res, nil
}

// padSets guarantees every option set has at least one slot, so
// zero-option indicators still pass a non-NULL array to the FFI. For
// Options >= 1 it is the identity.
func padSets(sets [][]float64) [][]float64 {
	if Options > 0 {
		return sets
	}
	out := make([][]float64, len(sets))
	for i := range out {
		out[i] = []float64{0}
	}
	return out
}
