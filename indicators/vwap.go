package indicators

/*
#cgo CFLAGS: -I${SRCDIR}/../../tulip_rs_ffi/include
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

// Vwap is the namespaced entry point for the Volume Weighted Average Price.
type vwap struct{}

var Vwap = vwap{}

// VwapInputs / VwapOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	VwapInputs  = int(C.VWAP_INPUTS)  // high, low, close, volume
	VwapOptions = int(C.VWAP_OPTIONS) // none
)

// VwapID is the FFI indicator id for state serialisation: fnv1a32("vwap"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var VwapID = uint32(C.C_INDICATOR_ID_VWAP)

// VwapState is a live VWAP streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type VwapState struct{ *tulip.State }

func vwapFreeState(h uintptr) { C.vwap_state_free(C.u2p(C.uintptr_t(h))) }

func wrapVwapState(h uintptr) *VwapState {
	return &VwapState{tulip.NewState(VwapID, h, vwapFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (vwap) Info() tulip.Info {
	ci := C.vwap_info()
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

// MinData returns the minimum number of bars VWAP needs to produce output.
func (vwap) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.vwap_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes VWAP over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [vwap]; with one
// flag, Rows is [vwap, typprice]. The returned VwapState is live
// streaming state — closing the Result does NOT free it (and vice versa).
func (vwap) Indicator(high, low, close, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *VwapState, error) {
	if len(options) != VwapOptions {
		return nil, nil, fmt.Errorf("vwap: Indicator needs %d option(s), got %d", VwapOptions, len(options))
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("vwap: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.vwap_indicator(
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
		return nil, nil, fmt.Errorf("vwap: %w", err)
	}
	return res, wrapVwapState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *VwapState) Batch(high, low, close, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("vwap: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.vwap_batch(
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
		return nil, fmt.Errorf("vwap: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *VwapState) Clone() (*VwapState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &VwapState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (vwap) DeserializeState(blob []byte) (*VwapState, error) {
	st, err := tulip.DeserializeState(VwapID, blob, vwapFreeState)
	if err != nil {
		return nil, fmt.Errorf("vwap: %w", err)
	}
	return &VwapState{st}, nil
}

// SimdByAssets computes VWAP for N assets in one CPU pass. Each asset
// carries the same series (high, low, close, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (vwap) SimdByAssets(assets [][VwapInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != VwapOptions {
		return nil, fmt.Errorf("vwap: SimdByAssets needs %d option(s), got %d", VwapOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("vwap: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.vwap_simd_by_assets(
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
		VwapID, vwapFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("vwap: %w", err)
	}
	return res, nil
}

// SimdByOptions is not available for VWAP (0 options).
func (vwap) SimdByOptions(high, low, close, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("vwap: SimdByOptions not available (0 options)")
}
