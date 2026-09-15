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
static inline uint32_t *u32_u(uintptr_t p) { return (uint32_t *)(void *)p; }

// Rebuild the CSR result structs from stored raw parts and release the
// buffers (never the streaming state — candlestick_state_free owns that).
static inline void ffi_candle_result_free(uintptr_t offs, uintptr_t ids,
                                          uintptr_t num_bars, uintptr_t total) {
	CCandleStickResult r;
	r.error = C_INDICATOR_ERROR_OK;
	r.num_bars = num_bars;
	r.total_patterns = total;
	r.bar_offsets = (uint32_t *)offs;
	r.pattern_ids = (uint32_t *)ids;
	r.state = NULL;
	candlestick_result_free(r);
}

static inline void ffi_candle_batch_free(uintptr_t offs, uintptr_t ids,
                                         uintptr_t num_bars, uintptr_t total) {
	CCandleStickBatchResult r;
	r.error = C_INDICATOR_ERROR_OK;
	r.num_bars = num_bars;
	r.total_patterns = total;
	r.bar_offsets = (uint32_t *)offs;
	r.pattern_ids = (uint32_t *)ids;
	candlestick_batch_result_free(r);
}
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"

	"tulip_rs_go/tulip"
)

// Candlestick is the namespaced entry point for pattern detection.
//
// Unlike every other indicator here, candlestick output is NOT f64 series:
// detection results are CSR-packed integer pattern ids per bar
// (Patterns(bar) below), resolved against a process-wide 77-entry pattern
// table exposed via NumPatterns / PatternInfo / PatternNames. It is also
// scalar-only: no optional outputs, no SIMD entry points. State
// serialisation (Serialize/DeserializeState/Clone) works like everywhere
// else.
type candlestick struct{}

var Candlestick = candlestick{}

const (
	CandlestickInputs  = int(C.CANDLESTICK_INPUTS)  // open, high, low, close
	CandlestickOptions = int(C.CANDLESTICK_OPTIONS) // candle, trend, trend-signal periods
)

// CandlestickID is the FFI serialisation id: fnv1a32("candlestick").
var CandlestickID = uint32(C.C_INDICATOR_ID_CANDLESTICK)

// Forecast filters detected patterns by their forecast direction.
type Forecast int32

const (
	ForecastNone Forecast = -1 // no filter (any other out-of-range value behaves the same)

	ForecastBearishReversal               Forecast = 0
	ForecastBullishReversal               Forecast = 1
	ForecastBearishContinuation           Forecast = 2
	ForecastBullishContinuation           Forecast = 3
	ForecastBearishReversalOrContinuation Forecast = 4
	ForecastBullishReversalOrContinuation Forecast = 5
)

func (f Forecast) String() string {
	switch f {
	case ForecastBearishReversal:
		return "BearishReversal"
	case ForecastBullishReversal:
		return "BullishReversal"
	case ForecastBearishContinuation:
		return "BearishContinuation"
	case ForecastBullishContinuation:
		return "BullishContinuation"
	case ForecastBearishReversalOrContinuation:
		return "BearishReversalOrContinuation"
	case ForecastBullishReversalOrContinuation:
		return "BullishReversalOrContinuation"
	default:
		return "None"
	}
}

// CandlePatternInfo describes one entry of the stable pattern table
// (ids are sorted by short name, so they never shift across builds).
type CandlePatternInfo struct {
	ID           uint32
	Name         string
	FullName     string
	JapaneseName string
	Forecast     Forecast
	Bars         int
}

// CandleResult owns the Rust-allocated CSR buffers of one candlestick run:
// patterns detected on bar i are IDs[Offsets[i]:Offsets[i+1]]. Like every
// other result, the views are valid until Close; Close never touches the
// streaming state.
type CandleResult struct {
	// NumBars is the number of bars the result covers.
	NumBars int
	// TotalPatterns is the number of detections across all bars.
	TotalPatterns int

	offsets    []C.uint32_t
	ids        []C.uint32_t
	offsAddr   uintptr
	idsAddr    uintptr
	numBarsRaw int
	total      int
	isBatch    bool
	closed     bool
}

func candleErr(code int32) error {
	if code == 0 { // C_INDICATOR_ERROR_OK
		return nil
	}
	return tulip.Error(code)
}

func newCandleResult(err int32, offs, ids unsafe.Pointer, numBars, total int, isBatch bool) (*CandleResult, error) {
	if e := candleErr(err); e != nil {
		return nil, e
	}
	o, i := uintptr(offs), uintptr(ids)
	return &CandleResult{
		NumBars:       numBars,
		TotalPatterns: total,
		offsets:       unsafe.Slice(C.u32_u(C.uintptr_t(o)), numBars+1),
		ids:           unsafe.Slice(C.u32_u(C.uintptr_t(i)), total),
		offsAddr:      o,
		idsAddr:       i,
		numBarsRaw:    numBars,
		total:         total,
		isBatch:       isBatch,
	}, nil
}

// Patterns returns the pattern ids detected on a bar (empty: no pattern).
func (r *CandleResult) Patterns(bar int) []uint32 {
	if bar < 0 || bar+1 >= len(r.offsets) {
		return nil
	}
	slice := r.ids[r.offsets[bar]:r.offsets[bar+1]]
	out := make([]uint32, len(slice))
	for i, v := range slice {
		out[i] = uint32(v)
	}
	return out
}

// Names returns the pattern short names detected on a bar.
func (r *CandleResult) Names(bar int) []string {
	s := r.Patterns(bar)
	if len(s) == 0 {
		return nil
	}
	table := patternNames()
	out := make([]string, len(s))
	for i, id := range s {
		if int(id) < len(table) {
			out[i] = table[id]
		} else {
			out[i] = fmt.Sprintf("pattern#%d", id)
		}
	}
	return out
}

// Close releases the CSR buffers. Idempotent.
func (r *CandleResult) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	if r.isBatch {
		C.ffi_candle_batch_free(C.uintptr_t(r.offsAddr), C.uintptr_t(r.idsAddr),
			C.uintptr_t(r.numBarsRaw), C.uintptr_t(r.total))
	} else {
		C.ffi_candle_result_free(C.uintptr_t(r.offsAddr), C.uintptr_t(r.idsAddr),
			C.uintptr_t(r.numBarsRaw), C.uintptr_t(r.total))
	}
	return nil
}

// CandlestickState is the live streaming state (resumable via Batch).
type CandlestickState struct{ *tulip.State }

func candlestickFreeState(h uintptr) { C.candlestick_state_free(C.u2p(C.uintptr_t(h))) }

// patternNames caches the process-lifetime name table (copy; never freed).
var (
	patternNamesOnce  sync.Once
	patternNamesTable []string
)

func patternNames() []string {
	patternNamesOnce.Do(func() {
		cn := C.candlestick_pattern_names()
		patternNamesTable = tulip.CopyStringArray(uintptr(unsafe.Pointer(cn.ptr)), int(cn.len))
	})
	return patternNamesTable
}

// Info returns the indicator metadata (Go copies; nothing to free).
func (candlestick) Info() tulip.Info {
	ci := C.candlestick_info()
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

// MinData: bars needed before any detection occurs.
func (candlestick) MinData(options []float64) uint64 {
	opts, keep := optionArg(options)
	n := uint64(C.candlestick_min_data(opts))
	runtime.KeepAlive(keep)
	return n
}

// NumPatterns returns the size of the stable pattern table (77).
func (candlestick) NumPatterns() int { return int(C.candlestick_num_patterns()) }

// PatternInfo looks up one pattern's metadata by id.
func (candlestick) PatternInfo(id uint32) (CandlePatternInfo, bool) {
	if int(id) >= int(C.candlestick_num_patterns()) {
		return CandlePatternInfo{}, false
	}
	pi := C.candlestick_pattern_info(C.uint32_t(id))
	return CandlePatternInfo{
		ID:           uint32(pi.id),
		Name:         C.GoString(pi.name),
		FullName:     C.GoString(pi.full_name),
		JapaneseName: C.GoString(pi.japanese_name),
		Forecast:     Forecast(pi.forecast),
		Bars:         int(pi.bars),
	}, true
}

// PatternNames returns the id-ordered table of pattern short names.
func (candlestick) PatternNames() []string { return patternNames() }

// Indicator detects candlestick patterns over the given bars. forecast
// selects a direction filter (ForecastNone for all). The returned state
// continues streaming; closing the result does not free it.
func (candlestick) Indicator(open, high, low, close []float64, options []float64, forecast Forecast) (*CandleResult, *CandlestickState, error) {
	if len(options) != CandlestickOptions {
		return nil, nil, fmt.Errorf("candlestick: Indicator needs %d option(s), got %d", CandlestickOptions, len(options))
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, nil, fmt.Errorf("candlestick: %w", err)
	}
	defer release()
	opts, optsKeep := optionArg(options)

	raw := C.candlestick_indicator(
		(**C.double)(arr),
		C.uintptr_t(len(open)),
		opts,
		C.int32_t(forecast),
	)
	res, err := newCandleResult(int32(raw.error), unsafe.Pointer(raw.bar_offsets), unsafe.Pointer(raw.pattern_ids),
		int(raw.num_bars), int(raw.total_patterns), false)
	runtime.KeepAlive(series)
	runtime.KeepAlive(optsKeep)
	if err != nil {
		return nil, nil, fmt.Errorf("candlestick: %w", err)
	}
	st := &CandlestickState{tulip.NewState(CandlestickID, uintptr(unsafe.Pointer(raw.state)), candlestickFreeState)}
	return res, st, nil
}

// Batch continues detection over new bars, mutating the state.
func (s *CandlestickState) Batch(open, high, low, close []float64, forecast Forecast) (*CandleResult, error) {
	if s.Closed() {
		return nil, tulip.ErrClosed
	}
	series := [][]float64{open, high, low, close}
	arr, release, err := tulip.MarshalSeries(series, len(open))
	if err != nil {
		return nil, fmt.Errorf("candlestick: %w", err)
	}
	defer release()

	raw := C.candlestick_batch(s.Handle(), (**C.double)(arr), C.uintptr_t(len(open)), C.int32_t(forecast))
	res, err := newCandleResult(int32(raw.error), unsafe.Pointer(raw.bar_offsets), unsafe.Pointer(raw.pattern_ids),
		int(raw.num_bars), int(raw.total_patterns), true)
	runtime.KeepAlive(series)
	if err != nil {
		return nil, fmt.Errorf("candlestick: %w", err)
	}
	return res, nil
}

// Clone returns an independent snapshot of the state.
func (s *CandlestickState) Clone() (*CandlestickState, error) {
	c, err := s.State.Clone()
	if err != nil {
		return nil, err
	}
	return &CandlestickState{c}, nil
}

// DeserializeState restores streaming state from a TRFS blob.
func (candlestick) DeserializeState(blob []byte) (*CandlestickState, error) {
	st, err := tulip.DeserializeState(CandlestickID, blob, candlestickFreeState)
	if err != nil {
		return nil, fmt.Errorf("candlestick: %w", err)
	}
	return &CandlestickState{st}, nil
}
