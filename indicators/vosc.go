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

// Vosc is the namespaced entry point for the Volume Oscillator.
type vosc struct{}

var Vosc = vosc{}

// VoscInputs / VoscOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	VoscInputs  = int(C.VOSC_INPUTS)  // volume
	VoscOptions = int(C.VOSC_OPTIONS) // short_period, long_period
)

// VoscID is the FFI indicator id for state serialisation: fnv1a32("vosc"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var VoscID = uint32(C.C_INDICATOR_ID_VOSC)

// VoscState is a live VOSC streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type VoscState struct{ *tulip.State }

func voscFreeState(h uintptr) { C.vosc_state_free(C.u2p(C.uintptr_t(h))) }

func wrapVoscState(h uintptr) *VoscState {
	return &VoscState{tulip.NewState(VoscID, h, voscFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (vosc) Info() tulip.Info {
	ci := C.vosc_info()
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

// MinData returns the minimum number of bars VOSC needs to produce output.
func (vosc) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.vosc_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes VOSC over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [vosc]; with two
// flags, Rows is [vosc, short_sma, long_sma]. The returned VoscState is live
// streaming state — closing the Result does NOT free it (and vice versa).
func (vosc) Indicator(volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *VoscState, error) {
	if len(options) != VoscOptions {
		return nil, nil, fmt.Errorf("vosc: Indicator needs %d option(s) [short_period, long_period], got %d", VoscOptions, len(options))
	}
	series := [][]float64{volume}
	arr, release, err := tulip.MarshalSeries(series, len(volume))
	if err != nil {
		return nil, nil, fmt.Errorf("vosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.vosc_indicator(
		(**C.double)(arr),
		C.size_t(len(volume)),
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
		return nil, nil, fmt.Errorf("vosc: %w", err)
	}
	return res, wrapVoscState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *VoscState) Batch(volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{volume}
	arr, release, err := tulip.MarshalSeries(series, len(volume))
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.vosc_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(volume)),
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
		return nil, fmt.Errorf("vosc: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *VoscState) Clone() (*VoscState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &VoscState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (vosc) DeserializeState(blob []byte) (*VoscState, error) {
	st, err := tulip.DeserializeState(VoscID, blob, voscFreeState)
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	return &VoscState{st}, nil
}

// SimdByAssets computes VOSC for N assets in one CPU pass. Each asset
// carries the same series (volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (vosc) SimdByAssets(assets [][VoscInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != VoscOptions {
		return nil, fmt.Errorf("vosc: SimdByAssets needs %d option(s) [short_period, long_period], got %d", VoscOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.vosc_simd_by_assets(
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
		VoscID, voscFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	return res, nil
}

// SimdByOptions computes VOSC for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (vosc) SimdByOptions(volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("vosc: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != VoscOptions {
			return nil, fmt.Errorf("vosc: option set %d has %d values, need %d", i, len(o), VoscOptions)
		}
	}
	series := [][]float64{volume}
	arr, release, err := tulip.MarshalSeries(series, len(volume))
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	defer release()
	sets := padSets(optionSets, VoscOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.vosc_simd_by_options(
		(**C.double)(arr),
		C.size_t(len(volume)),
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
		VoscID, voscFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("vosc: %w", err)
	}
	return res, nil
}
