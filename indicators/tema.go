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

// Tema is the namespaced entry point for the Triple Exponential Moving Average.
type tema struct{}

var Tema = tema{}

// TemaInputs / TemaOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	TemaInputs  = int(C.TEMA_INPUTS)  // real
	TemaOptions = int(C.TEMA_OPTIONS) // period
)

// TemaID is the FFI indicator id for state serialisation: fnv1a32("tema"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var TemaID = uint32(C.C_INDICATOR_ID_TEMA)

// TemaState is a live TEMA streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type TemaState struct{ *tulip.State }

func temaFreeState(h uintptr) { C.tema_state_free(C.u2p(C.uintptr_t(h))) }

func wrapTemaState(h uintptr) *TemaState {
	return &TemaState{tulip.NewState(TemaID, h, temaFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (tema) Info() tulip.Info {
	ci := C.tema_info()
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

// MinData returns the minimum number of bars TEMA needs to produce output.
func (tema) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.tema_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes TEMA over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [tema]; no optional outputs.
// The returned TemaState is live streaming state — closing the Result does NOT free it (and vice versa).
func (tema) Indicator(real []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *TemaState, error) {
	if len(options) != TemaOptions {
		return nil, nil, fmt.Errorf("tema: Indicator needs %d option(s) [period], got %d", TemaOptions, len(options))
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("tema: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.tema_indicator(
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
		return nil, nil, fmt.Errorf("tema: %w", err)
	}
	return res, wrapTemaState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *TemaState) Batch(real []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.tema_batch(
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
		return nil, fmt.Errorf("tema: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *TemaState) Clone() (*TemaState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &TemaState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (tema) DeserializeState(blob []byte) (*TemaState, error) {
	st, err := tulip.DeserializeState(TemaID, blob, temaFreeState)
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	return &TemaState{st}, nil
}

// SimdByAssets computes TEMA for N assets in one CPU pass. Each asset
// carries the same series (real) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (tema) SimdByAssets(assets [][TemaInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != TemaOptions {
		return nil, fmt.Errorf("tema: SimdByAssets needs %d option(s) [period], got %d", TemaOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.tema_simd_by_assets(
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
		TemaID, temaFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	return res, nil
}

// SimdByOptions computes TEMA for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (tema) SimdByOptions(real []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("tema: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != TemaOptions {
			return nil, fmt.Errorf("tema: option set %d has %d values, need %d", i, len(o), TemaOptions)
		}
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	defer release()
	sets := padSets(optionSets, TemaOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.tema_simd_by_options(
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
		TemaID, temaFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("tema: %w", err)
	}
	return res, nil
}
