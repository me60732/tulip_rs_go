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

// Msw is the namespaced entry point for the Moving Slope Weighted indicator.
type msw struct{}

var Msw = msw{}

// MswInputs / MswOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	MswInputs  = int(C.MSW_INPUTS)  // close
	MswOptions = int(C.MSW_OPTIONS) // period
)

// MswID is the FFI indicator id for state serialisation: fnv1a32("msw"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var MswID = uint32(C.C_INDICATOR_ID_MSW)

// MswState is a live MSW streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type MswState struct{ *tulip.State }

func mswFreeState(h uintptr) { C.msw_state_free(C.u2p(C.uintptr_t(h))) }

func wrapMswState(h uintptr) *MswState {
	return &MswState{tulip.NewState(MswID, h, mswFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (msw) Info() tulip.Info {
	ci := C.msw_info()
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

// MinData returns the minimum number of bars MSW needs to produce output.
func (msw) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.msw_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes MSW over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is [slope, lead]. MSW has no
// optional outputs.
func (msw) Indicator(close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *MswState, error) {
	if len(options) != MswOptions {
		return nil, nil, fmt.Errorf("msw: Indicator needs %d option(s) [period], got %d", MswOptions, len(options))
	}
	series := [][]float64{close}
	arr, release, err := tulip.MarshalSeries(series, len(close))
	if err != nil {
		return nil, nil, fmt.Errorf("msw: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.msw_indicator(
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
		return nil, nil, fmt.Errorf("msw: %w", err)
	}
	return res, wrapMswState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *MswState) Batch(close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{close}
	arr, release, err := tulip.MarshalSeries(series, len(close))
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	defer release()

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.msw_batch(
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
		return nil, fmt.Errorf("msw: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *MswState) Clone() (*MswState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &MswState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (msw) DeserializeState(blob []byte) (*MswState, error) {
	st, err := tulip.DeserializeState(MswID, blob, mswFreeState)
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	return &MswState{st}, nil
}

// SimdByAssets computes MSW for N assets in one CPU pass. Each asset
// carries the same series (close) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (msw) SimdByAssets(assets [][MswInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != MswOptions {
		return nil, fmt.Errorf("msw: SimdByAssets needs %d option(s) [period], got %d", MswOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.msw_simd_by_assets(
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
		MswID, mswFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	return res, nil
}

// SimdByOptions computes MSW for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (msw) SimdByOptions(close []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("msw: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != MswOptions {
			return nil, fmt.Errorf("msw: option set %d has %d values, need %d", i, len(o), MswOptions)
		}
	}
	series := [][]float64{close}
	arr, release, err := tulip.MarshalSeries(series, len(close))
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	defer release()
	sets := padSets(optionSets, MswOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	defer optsRelease()

	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.msw_simd_by_options(
		(**C.double)(arr),
		C.size_t(len(close)),
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
		MswID, mswFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("msw: %w", err)
	}
	return res, nil
}
