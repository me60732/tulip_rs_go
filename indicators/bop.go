package indicators

/*
#cgo CFLAGS: -I${SRCDIR}/../ffi/include
#include <stdbool.h>
#include <stdint.h>
// cgo prologue defines a `CBytes` helper; rename the FFI typedef to avoid
// the clash (must match the rename in every cgo file of this package and in
// the tulip package's preamble).
#define CBytes TulipBytes
#include "tulip_rs_ffi.h"

static inline void *u2p(uintptr_t p) { return (void *)p; }
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/me60732/tulip_rs_go/tulip"
)

// Bop is the namespaced entry point for the Balance of Power.
type bop struct{}

var Bop = bop{}

// BopInputs / BopOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	BopInputs  = int(C.BOP_INPUTS)  // open, high, low, close
	BopOptions = int(C.BOP_OPTIONS) // none
)

// BopID is the FFI indicator id for state serialisation: fnv1a32("bop"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var BopID = uint32(C.C_INDICATOR_ID_BOP)

// BopState is a live BOP streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type BopState struct{ *tulip.State }

func bopFreeState(h uintptr) { C.bop_state_free(C.u2p(C.uintptr_t(h))) }

func wrapBopState(h uintptr) *BopState {
	return &BopState{tulip.NewState(BopID, h, bopFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (bop) Info() tulip.Info {
	ci := C.bop_info()
	return tulip.NewInfo(
		int32(ci.indicator_type),
		C.GoString(ci.name),
		C.GoString(ci.full_name),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.inputs.ptr)), int(ci.inputs.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.options.ptr)), int(ci.options.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.outputs.ptr)), int(ci.outputs.len)),
		tulip.CopyStringArray(uintptr(unsafe.Pointer(ci.optional_outputs.ptr)), int(ci.optional_outputs.len)),
		tulip.CopyDisplayGroups(uintptr(unsafe.Pointer(ci.display_groups.ptr)), int(ci.display_groups.len)),
	)
}

// MinData returns the minimum number of bars BOP needs to produce output.
func (bop) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.bop_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes BOP over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is [bop].
func (bop) Indicator(open, high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *BopState, error) {
	if len(options) != BopOptions {
		return nil, nil, fmt.Errorf("bop: Indicator needs %d option(s), got %d", BopOptions, len(options))
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, nil, fmt.Errorf("bop: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.bop_indicator(
		(**C.double)(arr),
		C.size_t(len(open)),
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
		return nil, nil, fmt.Errorf("bop: %w", err)
	}
	return res, wrapBopState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *BopState) Batch(open, high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, fmt.Errorf("bop: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.bop_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(open)),
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
		return nil, fmt.Errorf("bop: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *BopState) Clone() (*BopState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &BopState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (bop) DeserializeState(blob []byte) (*BopState, error) {
	st, err := tulip.DeserializeState(BopID, blob, bopFreeState)
	if err != nil {
		return nil, fmt.Errorf("bop: %w", err)
	}
	return &BopState{st}, nil
}

// SimdByAssets computes BOP for N assets in one CPU pass. Each asset
// carries the same series (open, high, low, close) of equal length; all lanes
// share one options set.
func (bop) SimdByAssets(assets [][BopInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != BopOptions {
		return nil, fmt.Errorf("bop: SimdByAssets needs %d option(s), got %d", BopOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("bop: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.bop_simd_by_assets(
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
		BopID, bopFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("bop: %w", err)
	}
	return res, nil
}

// SimdByOptions is not implemented for BOP (no options, no simd_by_options FFI).
func (bop) SimdByOptions(open, high, low, close []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("bop: SimdByOptions not implemented (no options)")
}
