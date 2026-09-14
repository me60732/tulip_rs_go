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

// Pivotpoint is the namespaced entry point for the Pivot Point indicator.
type pivotpoint struct{}

var Pivotpoint = pivotpoint{}

// PivotpointInputs / PivotpointOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	PivotpointInputs  = int(C.PIVOTPOINT_INPUTS)  // high, low, close
	PivotpointOptions = int(C.PIVOTPOINT_OPTIONS) // period
)

// PivotpointID is the FFI indicator id for state serialisation: fnv1a32("pivotpoint"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var PivotpointID = uint32(C.C_INDICATOR_ID_PIVOTPOINT)

// PivotpointState is a live Pivot Point streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type PivotpointState struct{ *tulip.State }

func pivotpointFreeState(h uintptr) { C.pivotpoint_state_free(C.u2p(C.uintptr_t(h))) }

func wrapPivotpointState(h uintptr) *PivotpointState {
	return &PivotpointState{tulip.NewState(PivotpointID, h, pivotpointFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (pivotpoint) Info() tulip.Info {
	ci := C.pivotpoint_info()
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

// MinData returns the minimum number of bars Pivot Point needs to produce output.
func (pivotpoint) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.pivotpoint_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes Pivot Point over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [s3,s2,s1,pp,r1,r2,r3].
func (pivotpoint) Indicator(high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *PivotpointState, error) {
	if len(options) != PivotpointOptions {
		return nil, nil, fmt.Errorf("pivotpoint: Indicator needs %d option(s) [period], got %d", PivotpointOptions, len(options))
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("pivotpoint: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.pivotpoint_indicator(
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
		return nil, nil, fmt.Errorf("pivotpoint: %w", err)
	}
	return res, wrapPivotpointState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *PivotpointState) Batch(high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.pivotpoint_batch(
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
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *PivotpointState) Clone() (*PivotpointState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &PivotpointState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (pivotpoint) DeserializeState(blob []byte) (*PivotpointState, error) {
	st, err := tulip.DeserializeState(PivotpointID, blob, pivotpointFreeState)
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	return &PivotpointState{st}, nil
}

// SimdByAssets computes Pivot Point for N assets in one CPU pass. Each asset
// carries the same series (high, low, close) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (pivotpoint) SimdByAssets(assets [][PivotpointInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != PivotpointOptions {
		return nil, fmt.Errorf("pivotpoint: SimdByAssets needs %d option(s) [period], got %d", PivotpointOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.pivotpoint_simd_by_assets(
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
		PivotpointID, pivotpointFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	return res, nil
}

// SimdByOptions computes Pivot Point for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (pivotpoint) SimdByOptions(high, low, close []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("pivotpoint: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != PivotpointOptions {
			return nil, fmt.Errorf("pivotpoint: option set %d has %d values, need %d", i, len(o), PivotpointOptions)
		}
	}
	series := [][]float64{high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	defer release()
	sets := padSets(optionSets, PivotpointOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.pivotpoint_simd_by_options(
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
		PivotpointID, pivotpointFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("pivotpoint: %w", err)
	}
	return res, nil
}
