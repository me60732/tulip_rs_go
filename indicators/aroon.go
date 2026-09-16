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

// Aroon is the namespaced entry point for the Aroon indicator.
type aroon struct{}

var Aroon = aroon{}

// AroonInputs / AroonOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	AroonInputs  = int(C.AROON_INPUTS)  // high, low
	AroonOptions = int(C.AROON_OPTIONS) // period
)

// AroonID is the FFI indicator id for state serialisation: fnv1a32("aroon"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var AroonID = uint32(C.C_INDICATOR_ID_AROON)

// AroonState is a live AROON streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type AroonState struct{ *tulip.State }

func aroonFreeState(h uintptr) { C.aroon_state_free(C.u2p(C.uintptr_t(h))) }

func wrapAroonState(h uintptr) *AroonState {
	return &AroonState{tulip.NewState(AroonID, h, aroonFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (aroon) Info() tulip.Info {
	ci := C.aroon_info()
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

// MinData returns the minimum number of bars AROON needs to produce output.
func (aroon) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.aroon_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes AROON over the given bars.
//
// Outputs: Rows is [aroon_up, aroon_down].
func (aroon) Indicator(high, low []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *AroonState, error) {
	if len(options) != AroonOptions {
		return nil, nil, fmt.Errorf("aroon: Indicator needs %d option(s) [period], got %d", AroonOptions, len(options))
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("aroon: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.aroon_indicator(
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
		return nil, nil, fmt.Errorf("aroon: %w", err)
	}
	return res, wrapAroonState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *AroonState) Batch(high, low []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.aroon_batch(
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
		return nil, fmt.Errorf("aroon: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *AroonState) Clone() (*AroonState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &AroonState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (aroon) DeserializeState(blob []byte) (*AroonState, error) {
	st, err := tulip.DeserializeState(AroonID, blob, aroonFreeState)
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	return &AroonState{st}, nil
}

// SimdByAssets computes AROON for N assets in one CPU pass. Each asset
// carries the same series (high, low) of equal length; all lanes share one options set.
func (aroon) SimdByAssets(assets [][AroonInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != AroonOptions {
		return nil, fmt.Errorf("aroon: SimdByAssets needs %d option(s) [period], got %d", AroonOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.aroon_simd_by_assets(
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
		AroonID, aroonFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	return res, nil
}

// SimdByOptions computes AROON for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (aroon) SimdByOptions(high, low []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("aroon: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != AroonOptions {
			return nil, fmt.Errorf("aroon: option set %d has %d values, need %d", i, len(o), AroonOptions)
		}
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	defer release()
	sets := padSets(optionSets, AroonOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.aroon_simd_by_options(
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
		AroonID, aroonFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("aroon: %w", err)
	}
	return res, nil
}
