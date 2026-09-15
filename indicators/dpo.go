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

// Dpo is the namespaced entry point for the Detrended Price Oscillator.
type dpo struct{}

var Dpo = dpo{}

// DpoInputs / DpoOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	DpoInputs  = int(C.DPO_INPUTS)  // real
	DpoOptions = int(C.DPO_OPTIONS) // period
)

// DpoID is the FFI indicator id for state serialisation: fnv1a32("dpo"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var DpoID = uint32(C.C_INDICATOR_ID_DPO)

// DpoState is a live Dpo streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type DpoState struct{ *tulip.State }

func dpoFreeState(h uintptr) { C.dpo_state_free(C.u2p(C.uintptr_t(h))) }

func wrapDpoState(h uintptr) *DpoState {
	return &DpoState{tulip.NewState(DpoID, h, dpoFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (dpo) Info() tulip.Info {
	ci := C.dpo_info()
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

// MinData returns the minimum number of bars Dpo needs to produce output.
func (dpo) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.dpo_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes Dpo over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [dpo]; with one
// flag, Rows is [dpo]. The returned DpoState is live
// streaming state — closing the Result does NOT free it (and vice versa).
func (dpo) Indicator(real []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *DpoState, error) {
	if len(options) != DpoOptions {
		return nil, nil, fmt.Errorf("dpo: Indicator needs %d option(s) [period], got %d", DpoOptions, len(options))
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("dpo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.dpo_indicator(
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
		return nil, nil, fmt.Errorf("dpo: %w", err)
	}
	return res, wrapDpoState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *DpoState) Batch(real []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.dpo_batch(
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
		return nil, fmt.Errorf("dpo: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *DpoState) Clone() (*DpoState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &DpoState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (dpo) DeserializeState(blob []byte) (*DpoState, error) {
	st, err := tulip.DeserializeState(DpoID, blob, dpoFreeState)
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	return &DpoState{st}, nil
}

// SimdByAssets computes Dpo for N assets in one CPU pass. Each asset
// carries the same series (real) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (dpo) SimdByAssets(assets [][DpoInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != DpoOptions {
		return nil, fmt.Errorf("dpo: SimdByAssets needs %d option(s) [period], got %d", DpoOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.dpo_simd_by_assets(
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
		DpoID, dpoFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	return res, nil
}

// SimdByOptions computes Dpo for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (dpo) SimdByOptions(real []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("dpo: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != DpoOptions {
			return nil, fmt.Errorf("dpo: option set %d has %d values, need %d", i, len(o), DpoOptions)
		}
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	defer release()
	sets := padSets(optionSets, DpoOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.dpo_simd_by_options(
		(**C.double)(arr),
		C.size_t(len(real)),
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
		DpoID, dpoFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("dpo: %w", err)
	}
	return res, nil
}
