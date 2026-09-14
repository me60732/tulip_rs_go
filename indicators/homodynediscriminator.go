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

// Homodynediscriminator is the namespaced entry point for the Homodyne Discriminator.
type homodynediscriminator struct{}

var Homodynediscriminator = homodynediscriminator{}

// HomodynediscriminatorInputs / HomodynediscriminatorOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	HomodynediscriminatorInputs  = int(C.HOMODYNEDISCRIMINATOR_INPUTS)  // real
	HomodynediscriminatorOptions = int(C.HOMODYNEDISCRIMINATOR_OPTIONS) // none (recon counts header!)
)

// HomodynediscriminatorID is the FFI indicator id for state serialisation: fnv1a32("homodynediscriminator"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var HomodynediscriminatorID = uint32(C.C_INDICATOR_ID_HOMODYNEDISCRIMINATOR)

// HomodynediscriminatorState is a live HOMODYNEDISCRIMINATOR streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type HomodynediscriminatorState struct{ *tulip.State }

func homodynediscriminatorFreeState(h uintptr) {
	C.homodynediscriminator_state_free(C.u2p(C.uintptr_t(h)))
}

func wrapHomodynediscriminatorState(h uintptr) *HomodynediscriminatorState {
	return &HomodynediscriminatorState{tulip.NewState(HomodynediscriminatorID, h, homodynediscriminatorFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (homodynediscriminator) Info() tulip.Info {
	ci := C.homodynediscriminator_info()
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

// MinData returns the minimum number of bars HOMODYNEDISCRIMINATOR needs to produce output.
func (homodynediscriminator) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.homodynediscriminator_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes HOMODYNEDISCRIMINATOR over the given bars.
//
// Outputs: with optionalOutputs empty/nil, Rows is just [dc_period]. The returned HomodynediscriminatorState
// is live streaming state — closing the Result does NOT free it (and vice versa).
func (homodynediscriminator) Indicator(real []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *HomodynediscriminatorState, error) {
	if len(options) != HomodynediscriminatorOptions {
		return nil, nil, fmt.Errorf("homodynediscriminator: Indicator needs %d option(s), got %d", HomodynediscriminatorOptions, len(options))
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.homodynediscriminator_indicator(
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
		return nil, nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	return res, wrapHomodynediscriminatorState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *HomodynediscriminatorState) Batch(real []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{real}
	arr, release, err := tulip.MarshalSeries(series, len(real))
	if err != nil {
		return nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.homodynediscriminator_batch(
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
		return nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *HomodynediscriminatorState) Clone() (*HomodynediscriminatorState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &HomodynediscriminatorState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (homodynediscriminator) DeserializeState(blob []byte) (*HomodynediscriminatorState, error) {
	st, err := tulip.DeserializeState(HomodynediscriminatorID, blob, homodynediscriminatorFreeState)
	if err != nil {
		return nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	return &HomodynediscriminatorState{st}, nil
}

// SimdByAssets computes HOMODYNEDISCRIMINATOR for N assets in one CPU pass. Each asset
// carries the same series (real) of equal length; all lanes share one options set.
func (homodynediscriminator) SimdByAssets(assets [][HomodynediscriminatorInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != HomodynediscriminatorOptions {
		return nil, fmt.Errorf("homodynediscriminator: SimdByAssets needs %d option(s), got %d", HomodynediscriminatorOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.homodynediscriminator_simd_by_assets(
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
		HomodynediscriminatorID, homodynediscriminatorFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("homodynediscriminator: %w", err)
	}
	return res, nil
}
