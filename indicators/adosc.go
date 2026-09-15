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

// Adosc is the namespaced entry point for the Accumulation/Distribution Oscillator.
type adosc struct{}

var Adosc = adosc{}

// AdoscInputs / AdoscOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	AdoscInputs  = int(C.ADOSC_INPUTS)  // high, low, close, volume
	AdoscOptions = int(C.ADOSC_OPTIONS) // short_period, long_period
)

// AdoscID is the FFI indicator id for state serialisation: fnv1a32("adosc"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var AdoscID = uint32(C.C_INDICATOR_ID_ADOSC)

// AdoscState is a live ADOSC streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type AdoscState struct{ *tulip.State }

func adoscFreeState(h uintptr) { C.adosc_state_free(C.u2p(C.uintptr_t(h))) }

func wrapAdoscState(h uintptr) *AdoscState {
	return &AdoscState{tulip.NewState(AdoscID, h, adoscFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (adosc) Info() tulip.Info {
	ci := C.adosc_info()
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

// MinData returns the minimum number of bars ADOSC needs to produce output.
func (adosc) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.adosc_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes ADOSC over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [adosc].
func (adosc) Indicator(high, low, close, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *AdoscState, error) {
	if len(options) != AdoscOptions {
		return nil, nil, fmt.Errorf("adosc: Indicator needs %d option(s) [short_period, long_period], got %d", AdoscOptions, len(options))
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("adosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.adosc_indicator(
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
		return nil, nil, fmt.Errorf("adosc: %w", err)
	}
	return res, wrapAdoscState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *AdoscState) Batch(high, low, close, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.adosc_batch(
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
		return nil, fmt.Errorf("adosc: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *AdoscState) Clone() (*AdoscState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &AdoscState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (adosc) DeserializeState(blob []byte) (*AdoscState, error) {
	st, err := tulip.DeserializeState(AdoscID, blob, adoscFreeState)
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	return &AdoscState{st}, nil
}

// SimdByAssets computes ADOSC for N assets in one CPU pass. Each asset
// carries the same series (high, low, close, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (adosc) SimdByAssets(assets [][AdoscInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != AdoscOptions {
		return nil, fmt.Errorf("adosc: SimdByAssets needs %d option(s) [short_period, long_period], got %d", AdoscOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.adosc_simd_by_assets(
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
		AdoscID, adoscFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	return res, nil
}

// SimdByOptions computes ADOSC for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (adosc) SimdByOptions(high, low, close, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("adosc: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != AdoscOptions {
			return nil, fmt.Errorf("adosc: option set %d has %d values, need %d", i, len(o), AdoscOptions)
		}
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	defer release()
	sets := padSets(optionSets, AdoscOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.adosc_simd_by_options(
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
		AdoscID, adoscFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("adosc: %w", err)
	}
	return res, nil
}
