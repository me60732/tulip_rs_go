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

// Nvi is the namespaced entry point for the Negative Volume Index indicator.
type nvi struct{}

var Nvi = nvi{}

// NviInputs / NviOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	NviInputs  = int(C.NVI_INPUTS)  // real, volume
	NviOptions = int(C.NVI_OPTIONS) // none
)

// NviID is the FFI indicator id for state serialisation: fnv1a32("nvi"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var NviID = uint32(C.C_INDICATOR_ID_NVI)

// NviState is a live NVI streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type NviState struct{ *tulip.State }

func nviFreeState(h uintptr) { C.nvi_state_free(C.u2p(C.uintptr_t(h))) }

func wrapNviState(h uintptr) *NviState {
	return &NviState{tulip.NewState(NviID, h, nviFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (nvi) Info() tulip.Info {
	ci := C.nvi_info()
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

// MinData returns the minimum number of bars NVI needs to produce output.
func (nvi) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.nvi_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes NVI over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [nvi]. NVI has no
// optional outputs.
func (nvi) Indicator(real, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *NviState, error) {
	if len(options) != NviOptions {
		return nil, nil, fmt.Errorf("nvi: Indicator needs %d option(s), got %d", NviOptions, len(options))
	}
	series := [][]float64{real, volume}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("nvi: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.nvi_indicator(
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
		return nil, nil, fmt.Errorf("nvi: %w", err)
	}
	return res, wrapNviState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *NviState) Batch(real, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real, volume}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("nvi: %w", err)
	}
	defer release()

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.nvi_batch(
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
		return nil, fmt.Errorf("nvi: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *NviState) Clone() (*NviState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &NviState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (nvi) DeserializeState(blob []byte) (*NviState, error) {
	st, err := tulip.DeserializeState(NviID, blob, nviFreeState)
	if err != nil {
		return nil, fmt.Errorf("nvi: %w", err)
	}
	return &NviState{st}, nil
}

// SimdByAssets computes NVI for N assets in one CPU pass. Each asset
// carries the same series (real, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (nvi) SimdByAssets(assets [][NviInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != NviOptions {
		return nil, fmt.Errorf("nvi: SimdByAssets needs %d option(s), got %d", NviOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("nvi: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.nvi_simd_by_assets(
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
		NviID, nviFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("nvi: %w", err)
	}
	return res, nil
}

// SimdByOptions is NOT available for NVI (NVI_OPTIONS=0).
func (nvi) SimdByOptions(real, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("nvi: SimdByOptions not available (zero options)")
}
