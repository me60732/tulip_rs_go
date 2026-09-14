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

// Kvo is the namespaced entry point for the Klinger Volume Oscillator.
type kvo struct{}

var Kvo = kvo{}

// KvoInputs / KvoOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	KvoInputs  = int(C.KVO_INPUTS)  // high, low, close, volume
	KvoOptions = int(C.KVO_OPTIONS) // short_period, long_period
)

// KvoID is the FFI indicator id for state serialisation: fnv1a32("kvo"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var KvoID = uint32(C.C_INDICATOR_ID_KVO)

// KvoState is a live KVO streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type KvoState struct{ *tulip.State }

func kvoFreeState(h uintptr) { C.kvo_state_free(C.u2p(C.uintptr_t(h))) }

func wrapKvoState(h uintptr) *KvoState {
	return &KvoState{tulip.NewState(KvoID, h, kvoFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (kvo) Info() tulip.Info {
	ci := C.kvo_info()
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

// MinData returns the minimum number of bars KVO needs to produce output.
func (kvo) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.kvo_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes KVO over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is [kvo]; with short_ema and
// long_ema flags, Rows adds those rows in that fixed order. Note that volume
// series is required as an input (SeriesFor provides).
// The returned KvoState is live streaming state — closing the Result does
// NOT free it (and vice versa).
func (kvo) Indicator(high, low, close, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *KvoState, error) {
	if len(options) != KvoOptions {
		return nil, nil, fmt.Errorf("kvo: Indicator needs %d option(s) [short_period, long_period], got %d", KvoOptions, len(options))
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("kvo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.kvo_indicator(
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
		return nil, nil, fmt.Errorf("kvo: %w", err)
	}
	return res, wrapKvoState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *KvoState) Batch(high, low, close, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.kvo_batch(
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
		return nil, fmt.Errorf("kvo: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *KvoState) Clone() (*KvoState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &KvoState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (kvo) DeserializeState(blob []byte) (*KvoState, error) {
	st, err := tulip.DeserializeState(KvoID, blob, kvoFreeState)
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	return &KvoState{st}, nil
}

// SimdByAssets computes KVO for N assets in one CPU pass. Each asset
// carries the same series (high, low, close, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (kvo) SimdByAssets(assets [][KvoInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != KvoOptions {
		return nil, fmt.Errorf("kvo: SimdByAssets needs %d option(s) [short_period, long_period], got %d", KvoOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.kvo_simd_by_assets(
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
		KvoID, kvoFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	return res, nil
}

// SimdByOptions computes KVO for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (kvo) SimdByOptions(high, low, close, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("kvo: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != KvoOptions {
			return nil, fmt.Errorf("kvo: option set %d has %d values, need %d", i, len(o), KvoOptions)
		}
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	defer release()
	sets := padSets(optionSets, KvoOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.kvo_simd_by_options(
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
		KvoID, kvoFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("kvo: %w", err)
	}
	return res, nil
}
