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

// Avgprice is the namespaced entry point for the Average Price.
type avgprice struct{}

var Avgprice = avgprice{}

// AvgpriceInputs / AvgpriceOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	AvgpriceInputs  = int(C.AVGPRICE_INPUTS)  // open, high, low, close
	AvgpriceOptions = int(C.AVGPRICE_OPTIONS) // none
)

// AvgpriceID is the FFI indicator id for state serialisation: fnv1a32("avgprice"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var AvgpriceID = uint32(C.C_INDICATOR_ID_AVGPRICE)

// AvgpriceState is a live AVGPRICE streaming-state handle: the state returned by
// Indicator, resumable via Batch, closeable exactly once via Close.
type AvgpriceState struct{ *tulip.State }

func avgpriceFreeState(h uintptr) { C.avgprice_state_free(C.u2p(C.uintptr_t(h))) }

func wrapAvgpriceState(h uintptr) *AvgpriceState {
	return &AvgpriceState{tulip.NewState(AvgpriceID, h, avgpriceFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (avgprice) Info() tulip.Info {
	ci := C.avgprice_info()
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

// MinData returns the minimum number of bars AVGPRICE needs to produce output.
func (avgprice) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.avgprice_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes AVGPRICE over the given bars.
//
// Outputs: Rows is just [avgprice].
func (avgprice) Indicator(open, high, low, close []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *AvgpriceState, error) {
	if len(options) != AvgpriceOptions {
		return nil, nil, fmt.Errorf("avgprice: Indicator expects 0 option(s), got %d", len(options))
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, nil, fmt.Errorf("avgprice: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.avgprice_indicator(
		(**C.double)(arr),
		C.size_t(len(open)),
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
		return nil, nil, fmt.Errorf("avgprice: %w", err)
	}
	return res, wrapAvgpriceState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *AvgpriceState) Batch(open, high, low, close []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, fmt.Errorf("avgprice: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.avgprice_batch(
		s.Handle(),
		(**C.double)(arr),
		C.size_t(len(open)),
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
		return nil, fmt.Errorf("avgprice: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *AvgpriceState) Clone() (*AvgpriceState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &AvgpriceState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (avgprice) DeserializeState(blob []byte) (*AvgpriceState, error) {
	st, err := tulip.DeserializeState(AvgpriceID, blob, avgpriceFreeState)
	if err != nil {
		return nil, fmt.Errorf("avgprice: %w", err)
	}
	return &AvgpriceState{st}, nil
}

// SimdByAssets computes AVGPRICE for N assets in one CPU pass. Each asset
// carries the same series (open, high, low, close) of equal length; all lanes share one options set.
func (avgprice) SimdByAssets(assets [][AvgpriceInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != AvgpriceOptions {
		return nil, fmt.Errorf("avgprice: SimdByAssets expects 0 option(s), got %d", len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2], a[3]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("avgprice: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.avgprice_simd_by_assets(
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
		AvgpriceID, avgpriceFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("avgprice: %w", err)
	}
	return res, nil
}
