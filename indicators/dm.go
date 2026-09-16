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

// Dm is the namespaced entry point for the Directional Movement.
type dm struct{}

var Dm = dm{}

// DmInputs / DmOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	DmInputs  = int(C.DM_INPUTS)  // high, low
	DmOptions = int(C.DM_OPTIONS) // period
)

// DmID is the FFI indicator id for state serialisation: fnv1a32("dm"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var DmID = uint32(C.C_INDICATOR_ID_DM)

// DmState is a live DM streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type DmState struct{ *tulip.State }

func dmFreeState(h uintptr) { C.dm_state_free(C.u2p(C.uintptr_t(h))) }

func wrapDmState(h uintptr) *DmState {
	return &DmState{tulip.NewState(DmID, h, dmFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (dm) Info() tulip.Info {
	ci := C.dm_info()
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

// MinData returns the minimum number of bars DM needs to produce output.
func (dm) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.dm_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes DM over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [+dm, -dm]; with two
// flags, Rows is [+dm, -dm]. The returned DmState is live
// streaming state — closing the Result does NOT free it (and vice versa).
func (dm) Indicator(high, low []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *DmState, error) {
	if len(options) != DmOptions {
		return nil, nil, fmt.Errorf("dm: Indicator needs %d option(s) [period], got %d", DmOptions, len(options))
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("dm: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.dm_indicator(
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
		return nil, nil, fmt.Errorf("dm: %w", err)
	}
	return res, wrapDmState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *DmState) Batch(high, low []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.dm_batch(
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
		return nil, fmt.Errorf("dm: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *DmState) Clone() (*DmState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &DmState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (dm) DeserializeState(blob []byte) (*DmState, error) {
	st, err := tulip.DeserializeState(DmID, blob, dmFreeState)
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	return &DmState{st}, nil
}

// SimdByAssets computes DM for N assets in one CPU pass. Each asset
// carries the same series (high, low) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (dm) SimdByAssets(assets [][DmInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != DmOptions {
		return nil, fmt.Errorf("dm: SimdByAssets needs %d option(s) [period], got %d", DmOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.dm_simd_by_assets(
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
		DmID, dmFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	return res, nil
}

// SimdByOptions computes DM for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (dm) SimdByOptions(high, low []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("dm: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != DmOptions {
			return nil, fmt.Errorf("dm: option set %d has %d values, need %d", i, len(o), DmOptions)
		}
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	defer release()
	sets := padSets(optionSets, DmOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.dm_simd_by_options(
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
		DmID, dmFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("dm: %w", err)
	}
	return res, nil
}
