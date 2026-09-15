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

// Ao is the namespaced entry point for the Awesome Oscillator.
type ao struct{}

var Ao = ao{}

// AoInputs / AoOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	AoInputs  = int(C.AO_INPUTS)  // high, low
	AoOptions = int(C.AO_OPTIONS) // none
)

// AoID is the FFI indicator id for state serialisation: fnv1a32("ao"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var AoID = uint32(C.C_INDICATOR_ID_AO)

// AoState is a live AO streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type AoState struct{ *tulip.State }

func aoFreeState(h uintptr) { C.ao_state_free(C.u2p(C.uintptr_t(h))) }

func wrapAoState(h uintptr) *AoState {
	return &AoState{tulip.NewState(AoID, h, aoFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (ao) Info() tulip.Info {
	ci := C.ao_info()
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

// MinData returns the minimum number of bars AO needs to produce output.
func (ao) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.ao_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes AO over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [ao].
func (ao) Indicator(high, low []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *AoState, error) {
	if len(options) != AoOptions {
		return nil, nil, fmt.Errorf("ao: Indicator needs %d option(s), got %d", AoOptions, len(options))
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("ao: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.ao_indicator(
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
		return nil, nil, fmt.Errorf("ao: %w", err)
	}
	return res, wrapAoState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *AoState) Batch(high, low []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("ao: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.ao_batch(
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
		return nil, fmt.Errorf("ao: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *AoState) Clone() (*AoState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &AoState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (ao) DeserializeState(blob []byte) (*AoState, error) {
	st, err := tulip.DeserializeState(AoID, blob, aoFreeState)
	if err != nil {
		return nil, fmt.Errorf("ao: %w", err)
	}
	return &AoState{st}, nil
}

// SimdByAssets computes AO for N assets in one CPU pass. Each asset
// carries the same series (high, low) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (ao) SimdByAssets(assets [][AoInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != AoOptions {
		return nil, fmt.Errorf("ao: SimdByAssets needs %d option(s), got %d", AoOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("ao: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.ao_simd_by_assets(
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
		AoID, aoFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("ao: %w", err)
	}
	return res, nil
}
