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

// Instantaneoustrendline is the namespaced entry point for the Ehlers Instantaneous Trendline.
type instantaneoustrendline struct{}

var Instantaneoustrendline = instantaneoustrendline{}

// InstantaneoustrendlineInputs / InstantaneoustrendlineOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	InstantaneoustrendlineInputs  = int(C.INSTANTANEOUSTRENDLINE_INPUTS)  // real
	InstantaneoustrendlineOptions = int(C.INSTANTANEOUSTRENDLINE_OPTIONS) // none (fully adaptive)
)

// InstantaneoustrendlineID is the FFI indicator id for state serialisation: fnv1a32("instantaneoustrendline"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var InstantaneoustrendlineID = uint32(C.C_INDICATOR_ID_INSTANTANEOUSTRENDLINE)

// InstantaneoustrendlineState is a live InstantaneousTrendline streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type InstantaneoustrendlineState struct{ *tulip.State }

func instantaneoustrendlineFreeState(h uintptr) {
	C.instantaneoustrendline_state_free(C.u2p(C.uintptr_t(h)))
}

func wrapInstantaneoustrendlineState(h uintptr) *InstantaneoustrendlineState {
	return &InstantaneoustrendlineState{tulip.NewState(InstantaneoustrendlineID, h, instantaneoustrendlineFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (instantaneoustrendline) Info() tulip.Info {
	ci := C.instantaneoustrendline_info()
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

// MinData returns the minimum number of bars InstantaneousTrendline needs to produce output.
func (instantaneoustrendline) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.instantaneoustrendline_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes InstantaneousTrendline over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is [trendline]; with flags,
// Rows adds trigger, dc_period, alpha in that fixed order.
// The returned InstantaneoustrendlineState is live streaming state — closing the Result does
// NOT free it (and vice versa).
func (instantaneoustrendline) Indicator(real []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *InstantaneoustrendlineState, error) {
	if len(options) != InstantaneoustrendlineOptions {
		return nil, nil, fmt.Errorf("instantaneoustrendline: Indicator needs %d option(s), got %d", InstantaneoustrendlineOptions, len(options))
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.instantaneoustrendline_indicator(
		(**C.double)(arr),
		C.size_t(len(real)),
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
		return nil, nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	return res, wrapInstantaneoustrendlineState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *InstantaneoustrendlineState) Batch(real []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.instantaneoustrendline_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(real)),
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
		return nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *InstantaneoustrendlineState) Clone() (*InstantaneoustrendlineState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &InstantaneoustrendlineState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (instantaneoustrendline) DeserializeState(blob []byte) (*InstantaneoustrendlineState, error) {
	st, err := tulip.DeserializeState(InstantaneoustrendlineID, blob, instantaneoustrendlineFreeState)
	if err != nil {
		return nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	return &InstantaneoustrendlineState{st}, nil
}

// SimdByAssets computes InstantaneousTrendline for N assets in one CPU pass. Each asset
// carries the same series (real) of equal length; all lanes share one options set.
// The SimdResult owns all lane states — Close it, not the individual States.
func (instantaneoustrendline) SimdByAssets(assets [][InstantaneoustrendlineInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != InstantaneoustrendlineOptions {
		return nil, fmt.Errorf("instantaneoustrendline: SimdByAssets needs %d option(s), got %d", InstantaneoustrendlineOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.instantaneoustrendline_simd_by_assets(
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
		InstantaneoustrendlineID, instantaneoustrendlineFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("instantaneoustrendline: %w", err)
	}
	return res, nil
}
