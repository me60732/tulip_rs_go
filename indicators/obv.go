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

	"github.com/me60732/tulip_rs_go/tulip"
)

// Obv is the namespaced entry point for the On Balance Volume indicator.
type obv struct{}

var Obv = obv{}

// ObvInputs / ObvOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	ObvInputs  = int(C.OBV_INPUTS)  // real, volume
	ObvOptions = int(C.OBV_OPTIONS) // none
)

// ObvID is the FFI indicator id for state serialisation: fnv1a32("obv"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var ObvID = uint32(C.C_INDICATOR_ID_OBV)

// ObvState is a live OBV streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type ObvState struct{ *tulip.State }

func obvFreeState(h uintptr) { C.obv_state_free(C.u2p(C.uintptr_t(h))) }

func wrapObvState(h uintptr) *ObvState {
	return &ObvState{tulip.NewState(ObvID, h, obvFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (obv) Info() tulip.Info {
	ci := C.obv_info()
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

// MinData returns the minimum number of bars OBV needs to produce output.
func (obv) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.obv_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes OBV over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [obv]. OBV has no
// optional outputs.
func (obv) Indicator(real, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *ObvState, error) {
	if len(options) != ObvOptions {
		return nil, nil, fmt.Errorf("obv: Indicator needs %d option(s), got %d", ObvOptions, len(options))
	}
	series := [][]float64{real, volume}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("obv: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.obv_indicator(
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
		return nil, nil, fmt.Errorf("obv: %w", err)
	}
	return res, wrapObvState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *ObvState) Batch(real, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real, volume}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("obv: %w", err)
	}
	defer release()

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.obv_batch(
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
		return nil, fmt.Errorf("obv: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *ObvState) Clone() (*ObvState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &ObvState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (obv) DeserializeState(blob []byte) (*ObvState, error) {
	st, err := tulip.DeserializeState(ObvID, blob, obvFreeState)
	if err != nil {
		return nil, fmt.Errorf("obv: %w", err)
	}
	return &ObvState{st}, nil
}

// SimdByAssets computes OBV for N assets in one CPU pass. Each asset
// carries the same series (real, volume) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (obv) SimdByAssets(assets [][ObvInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != ObvOptions {
		return nil, fmt.Errorf("obv: SimdByAssets needs %d option(s), got %d", ObvOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("obv: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.obv_simd_by_assets(
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
		ObvID, obvFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("obv: %w", err)
	}
	return res, nil
}

// SimdByOptions is NOT available for OBV (OBV_OPTIONS=0).
func (obv) SimdByOptions(real, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	return nil, fmt.Errorf("obv: SimdByOptions not available (zero options)")
}
