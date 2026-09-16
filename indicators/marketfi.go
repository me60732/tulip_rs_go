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

// Marketfi is the namespaced entry point for Market Force Index.
type marketfi struct{}

var Marketfi = marketfi{}

// MarketfiInputs / MarketfiOptions: series/option arities, from the generated C
// defines (tulip_rs_ffi_counts.h) so they can never drift from the core.
const (
	MarketfiInputs  = int(C.MARKETFI_INPUTS)  // high, low, volume
	MarketfiOptions = int(C.MARKETFI_OPTIONS) // none
)

// MarketfiID is the FFI indicator id for state serialisation: fnv1a32("marketfi"),
// from the generated tulip_rs_ffi_state_ids.h. Never recompute it here.
var MarketfiID = uint32(C.C_INDICATOR_ID_MARKETFI)

// MarketfiState is a live Market Force Index streaming-state handle:
// the state returned by Indicator, resumable via Batch, closeable exactly once via Close.
type MarketfiState struct{ *tulip.State }

func marketfiFreeState(h uintptr) { C.marketfi_state_free(C.u2p(C.uintptr_t(h))) }

func wrapMarketfiState(h uintptr) *MarketfiState {
	return &MarketfiState{tulip.NewState(MarketfiID, h, marketfiFreeState)}
}

// Info returns the indicator's metadata as pure Go values. The underlying
// C strings are process-lifetime; they are copied and never freed.
func (marketfi) Info() tulip.Info {
	ci := C.marketfi_info()
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

// MinData returns the minimum number of bars Marketfi needs to produce output.
func (marketfi) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.marketfi_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// Indicator computes Market Force Index over the given bars.
//
// Outputs: Rows is just [marketfi] (no optional outputs). The returned MarketfiState
// is live streaming state — closing the Result does NOT free it (and vice versa).
func (marketfi) Indicator(high, low, volume []float64, options []float64, optionalOutputs []bool) (*tulip.Result, *MarketfiState, error) {
	if len(options) != MarketfiOptions {
		return nil, nil, fmt.Errorf("marketfi: Indicator needs %d option(s), got %d", MarketfiOptions, len(options))
	}
	series := [][]float64{high, low, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, nil, fmt.Errorf("marketfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.marketfi_indicator(
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
		return nil, nil, fmt.Errorf("marketfi: %w", err)
	}
	return res, wrapMarketfiState(uintptr(raw.state)), nil
}

// Batch continues the stream with new bars; no need to re-send history.
func (s *MarketfiState) Batch(high, low, volume []float64, optionalOutputs []bool) (*tulip.Result, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{high, low, volume}
	arr, release, err := tulip.MarshalSeries(series, len(high))
	if err != nil {
		return nil, fmt.Errorf("marketfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()

	raw := C.marketfi_batch(
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
		return nil, fmt.Errorf("marketfi: %w", err)
	}
	return res, nil
}

// Clone returns an independent deep copy of the state (no serde involved).
func (s *MarketfiState) Clone() (*MarketfiState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &MarketfiState{c}, nil
}

// DeserializeState restores a streaming state from a TRFS blob produced by
// State.Serialize (from this or any other tulip_rs_ffi binding).
func (marketfi) DeserializeState(blob []byte) (*MarketfiState, error) {
	st, err := tulip.DeserializeState(MarketfiID, blob, marketfiFreeState)
	if err != nil {
		return nil, fmt.Errorf("marketfi: %w", err)
	}
	return &MarketfiState{st}, nil
}

// SimdByAssets computes Market Force Index for N assets in one CPU pass. Each asset
// carries the same series (high, low, volume) of equal length; all lanes share one options set.
// The SimdResult owns all lane states — Close it, not the individual States.
func (marketfi) SimdByAssets(assets [][MarketfiInputs][]float64, options []float64, optionalOutputs []bool) (*tulip.SimdResult, error) {
	if len(options) != MarketfiOptions {
		return nil, fmt.Errorf("marketfi: SimdByAssets needs %d option(s), got %d", MarketfiOptions, len(options))
	}
	groups := make([][][]float64, len(assets))
	for i, a := range assets {
		groups[i] = [][]float64{a[0], a[1], a[2]}
	}
	outer, release, err := tulip.MarshalSimdInputs(groups, len(assets[0][0]))
	if err != nil {
		return nil, fmt.Errorf("marketfi: %w", err)
	}
	defer release()
	optArr, optRelease := tulip.MarshalBools(optionalOutputs)
	defer optRelease()
	opts, optsKeep := optionArg(options)

	raw := C.marketfi_simd_by_assets(
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
		MarketfiID, marketfiFreeState,
	)
	runtime.KeepAlive(assets)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, fmt.Errorf("marketfi: %w", err)
	}
	return res, nil
}

// NOTE: SimdByOptions is not available for Marketfi because the FFI does not
// expose marketfi_simd_by_options (zero-option indicators have no option sets to vary).
