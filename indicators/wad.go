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

// Wad is the namespaced entry point for the Wave/Demo indicator (accumulation).
type wad struct{}

var Wad = wad{}

// WadInputs / WadOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	WadInputs  = int(C.WAD_INPUTS)  // high, low, close
	WadOptions = int(C.WAD_OPTIONS) // none
)

// WadID is the FFI indicator id for state serialisation: fnv1a32("wad"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var WadID = uint32(C.C_INDICATOR_ID_WAD)

// WadState is a live WAD streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type WadState struct{ *tulip.State }

func wadFreeState(h uintptr) { C.wad_state_free(C.u2p(C.uintptr_t(h))) }

func wrapWadState(h uintptr) *WadState {
	return &WadState{tulip.NewState(WadID, h, wadFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (wad) Info() tulip.Info {
	ci := C.wad_info()
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

// MinData returns the minimum number of bars WAD needs to produce output.
func (wad) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.wad_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes WAD over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [wad]; no optional
// outputs exist for WAD. The returned WadState is live streaming state —
// closing the Result does NOT free it (and vice versa).
func (wad) Indicator(high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *WadState, error) {
	if len(options) != WadOptions {
		return nil, nil, fmt.Errorf("wad: Indicator needs %d option(s), got %d", WadOptions, len(options))
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("wad: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.wad_indicator(
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
		return nil, nil, fmt.Errorf("wad: %w", err)
	}
	return res, wrapWadState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *WadState) Batch(high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("wad: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.wad_batch(
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
		return nil, fmt.Errorf("wad: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *WadState) Clone() (*WadState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &WadState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (wad) DeserializeState(blob []byte) (*WadState, error) {
	st, err := tulip.DeserializeState(WadID, blob, wadFreeState)
	if err != nil {
		return nil, fmt.Errorf("wad: %w", err)
	}
	return &WadState{st}, nil
}

// SimdByAssets computes WAD for N assets in one CPU pass. Each asset
// carries the same series (high, low, close) of equal length; all lanes share
// one options set. The SimdResult owns all lane states — Close it, not the
// individual States.
func (wad) SimdByAssets(assets [][WadInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != WadOptions {
		return nil, fmt.Errorf("wad: SimdByAssets needs %d option(s), got %d", WadOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("wad: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.wad_simd_by_assets(
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
		WadID, wadFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("wad: %w", err)
	}
	return res, nil
}

// SimdByOptions is not available for WAD (0 options).
func (wad) SimdByOptions(high, low, close []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("wad: SimdByOptions not available (0 options)")
}
