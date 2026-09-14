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

// Mfi is the namespaced entry point for the Money Flow Index indicator.
type mfi struct{}

var Mfi = mfi{}

// MfiInputs / MfiOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	MfiInputs  = int(C.MFI_INPUTS)  // high, low, close, volume
	MfiOptions = int(C.MFI_OPTIONS) // period
)

// MfiID is the FFI indicator id for state serialisation: fnv1a32("mfi"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var MfiID = uint32(C.C_INDICATOR_ID_MFI)

// MfiState is a live MFI streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type MfiState struct{ *tulip.State }

func mfiFreeState(h uintptr) { C.mfi_state_free(C.u2p(C.uintptr_t(h))) }

func wrapMfiState(h uintptr) *MfiState {
	return &MfiState{tulip.NewState(MfiID, h, mfiFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (mfi) Info() tulip.Info {
	ci := C.mfi_info()
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

// MinData returns the minimum number of bars MFI needs to produce output.
func (mfi) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.mfi_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes MFI over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [mfi]; with one flag,
// Rows is [mfi, typprice].
func (mfi) Indicator(high, low, close, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *MfiState, error) {
	if len(options) != MfiOptions {
		return nil, nil, fmt.Errorf("mfi: Indicator needs %d option(s) [period], got %d", MfiOptions, len(options))
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("mfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.mfi_indicator(
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
		return nil, nil, fmt.Errorf("mfi: %w", err)
	}
	return res, wrapMfiState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *MfiState) Batch(high, low, close, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.mfi_batch(
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
		return nil, fmt.Errorf("mfi: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *MfiState) Clone() (*MfiState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &MfiState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (mfi) DeserializeState(blob []byte) (*MfiState, error) {
	st, err := tulip.DeserializeState(MfiID, blob, mfiFreeState)
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	return &MfiState{st}, nil
}

// SimdByAssets computes MFI for N assets in one CPU pass. Each asset
// carries the same series (high, low, close, volume) of equal length; all lanes share one options set.
func (mfi) SimdByAssets(assets [][MfiInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != MfiOptions {
		return nil, fmt.Errorf("mfi: SimdByAssets needs %d option(s) [period], got %d", MfiOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.mfi_simd_by_assets(
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
		MfiID, mfiFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	return res, nil
}

// SimdByOptions computes MFI for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (mfi) SimdByOptions(high, low, close, volume []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("mfi: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != MfiOptions {
			return nil, fmt.Errorf("mfi: option set %d has %d values, need %d", i, len(o), MfiOptions)
		}
	}
	series := [][]float64{high, low, close, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	defer release()
	sets := padSets(optionSets, MfiOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.mfi_simd_by_options(
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
		MfiID, mfiFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("mfi: %w", err)
	}
	return res, nil
}
