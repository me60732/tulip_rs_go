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

// Pvi is the namespaced entry point for the Positive Volume Index.
type pvi struct{}

var Pvi = pvi{}

// PviInputs / PviOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	PviInputs  = int(C.PVI_INPUTS)  // close, volume
	PviOptions = int(C.PVI_OPTIONS) // none
)

// PviID is the FFI indicator id for state serialisation: fnv1a32("pvi"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var PviID = uint32(C.C_INDICATOR_ID_PVI)

// PviState is a live PVI streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type PviState struct{ *tulip.State }

func pviFreeState(h uintptr) { C.pvi_state_free(C.u2p(C.uintptr_t(h))) }

func wrapPviState(h uintptr) *PviState {
	return &PviState{tulip.NewState(PviID, h, pviFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (pvi) Info() tulip.Info {
	ci := C.pvi_info()
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

// MinData returns the minimum number of bars PVI needs to produce output.
func (pvi) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.pvi_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes PVI over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [pvi].
func (pvi) Indicator(close, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *PviState, error) {
	if len(options) != PviOptions {
		return nil, nil, fmt.Errorf("pvi: Indicator needs %d option(s), got %d", PviOptions, len(options))
	}
	series := [][]float64{close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(close))
	if err != nil {
		return nil, nil, fmt.Errorf("pvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.pvi_indicator(
		(**C.double)(arr),
		C.size_t(len(close)),
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
		return nil, nil, fmt.Errorf("pvi: %w", err)
	}
	return res, wrapPviState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *PviState) Batch(close, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(close))
	if err != nil {
		return nil, fmt.Errorf("pvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.pvi_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(close)),
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
		return nil, fmt.Errorf("pvi: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *PviState) Clone() (*PviState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &PviState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (pvi) DeserializeState(blob []byte) (*PviState, error) {
	st, err := tulip.DeserializeState(PviID, blob, pviFreeState)
	if err != nil {
		return nil, fmt.Errorf("pvi: %w", err)
	}
	return &PviState{st}, nil
}

// SimdByAssets computes PVI for N assets in one CPU pass. Each asset
// carries the same series (close, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (pvi) SimdByAssets(assets [][PviInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != PviOptions {
		return nil, fmt.Errorf("pvi: SimdByAssets needs %d option(s), got %d", PviOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("pvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.pvi_simd_by_assets(
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
		PviID, pviFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("pvi: %w", err)
	}
	return res, nil
}

// SimdByOptions is not available for PVI (no options).
func (pvi) SimdByOptions(close, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("pvi: SimdByOptions not available (PVI has no options)")
}
