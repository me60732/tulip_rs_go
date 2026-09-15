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

	"tulip_rs_go/tulip"
)

// Tr is the namespaced entry point for the True Range indicator.
type tr struct{}

var Tr = tr{}

// TrInputs / TrOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	TrInputs  = int(C.TR_INPUTS)  // high, low, close
	TrOptions = int(C.TR_OPTIONS) // none (0)
)

// TrID is the FFI indicator id for state serialisation: fnv1a32("tr"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var TrID = uint32(C.C_INDICATOR_ID_TR)

// TrState is a live TR streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type TrState struct{ *tulip.State }

func trFreeState(h uintptr) { C.tr_state_free(C.u2p(C.uintptr_t(h))) }

func wrapTrState(h uintptr) *TrState {
	return &TrState{tulip.NewState(TrID, h, trFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (tr) Info() tulip.Info {
	ci := C.tr_info()
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

// MinData returns the minimum number of bars TR needs to produce output.
func (tr) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.tr_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes TR over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [tr]; with flags,
// Rows is [tr, atr, medprice]. The returned TrState is live streaming state — closing the Result does NOT free it (and vice versa).
func (tr) Indicator(high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *TrState, error) {
	if len(options) != TrOptions {
		return nil, nil, fmt.Errorf("tr: Indicator needs %d option(s), got %d", TrOptions, len(options))
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("tr: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.tr_indicator(
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
		return nil, nil, fmt.Errorf("tr: %w", err)
	}
	return res, wrapTrState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *TrState) Batch(high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("tr: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.tr_batch(
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
		return nil, fmt.Errorf("tr: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *TrState) Clone() (*TrState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &TrState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (tr) DeserializeState(blob []byte) (*TrState, error) {
	st, err := tulip.DeserializeState(TrID, blob, trFreeState)
	if err != nil {
		return nil, fmt.Errorf("tr: %w", err)
	}
	return &TrState{st}, nil
}

// SimdByAssets computes TR for N assets in one CPU pass. Each asset
// carries the same series (high, low, close) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (tr) SimdByAssets(assets [][TrInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != TrOptions {
		return nil, fmt.Errorf("tr: SimdByAssets needs %d option(s), got %d", TrOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("tr: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.tr_simd_by_assets(
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
		TrID, trFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("tr: %w", err)
	}
	return res, nil
}
