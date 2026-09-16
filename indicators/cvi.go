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

// Cvi is the namespaced entry point for the Chaikin Volatility Index.
type cvi struct{}

var Cvi = cvi{}

// CviInputs / CviOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	CviInputs  = int(C.CVI_INPUTS)  // high, low
	CviOptions = int(C.CVI_OPTIONS) // period
)

// CviID is the FFI indicator id for state serialisation: fnv1a32("cvi"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var CviID = uint32(C.C_INDICATOR_ID_CVI)

// CviState is a live CVI streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type CviState struct{ *tulip.State }

func cviFreeState(h uintptr) { C.cvi_state_free(C.u2p(C.uintptr_t(h))) }

func wrapCviState(h uintptr) *CviState {
	return &CviState{tulip.NewState(CviID, h, cviFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (cvi) Info() tulip.Info {
	ci := C.cvi_info()
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

// MinData returns the minimum number of bars CVI needs to produce output.
func (cvi) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.cvi_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes CVI over the given bars.
//
// Outputs: Rows is [cvi_line]. The returned CviState is live
// streaming state — closing the Result does NOT free it (and vice versa).
func (cvi) Indicator(high, low []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *CviState, error) {
	if len(options) != CviOptions {
		return nil, nil, fmt.Errorf("cvi: Indicator needs %d option(s) [period], got %d", CviOptions, len(options))
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("cvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.cvi_indicator(
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
		return nil, nil, fmt.Errorf("cvi: %w", err)
	}
	return res, wrapCviState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *CviState) Batch(high, low []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.cvi_batch(
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
		return nil, fmt.Errorf("cvi: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *CviState) Clone() (*CviState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &CviState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (cvi) DeserializeState(blob []byte) (*CviState, error) {
	st, err := tulip.DeserializeState(CviID, blob, cviFreeState)
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	return &CviState{st}, nil
}

// SimdByAssets computes CVI for N assets in one CPU pass. Each asset
// carries the same series (high, low) of equal length; all lanes
// share one options set. The SimdResult owns all lane states — Close it,
// not the individual States.
func (cvi) SimdByAssets(assets [][CviInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != CviOptions {
		return nil, fmt.Errorf("cvi: SimdByAssets needs %d option(s) [period], got %d", CviOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.cvi_simd_by_assets(
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
		CviID, cviFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	return res, nil
}

// SimdByOptions computes CVI for N option sets over one shared series of
// bars in one CPU pass. Each lane's resumable state lives in
// SimdResult.States.
func (cvi) SimdByOptions(high, low []float64, optionSets [][]float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(optionSets) == 0 {
		return nil, fmt.Errorf("cvi: no option sets")
	}
	for i, o := range optionSets {
		if len(o) != CviOptions {
			return nil, fmt.Errorf("cvi: option set %d has %d values, need %d", i, len(o), CviOptions)
		}
	}
	series := [][]float64{high, low}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	defer release()
	sets := padSets(optionSets, CviOptions)
	optsArr, optsRelease, err := tulip.MarshalSeries(sets, len(sets[0]))
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	defer optsRelease()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.cvi_simd_by_options(
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
		CviID, cviFreeState,
	)
	runtime.KeepAlive(series)
	runtime.KeepAlive(sets)
	if err != nil {
		return nil, fmt.Errorf("cvi: %w", err)
	}
	return res, nil
}
