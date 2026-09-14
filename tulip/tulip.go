// Package tulip is the shared low-level plumbing for the Go bindings to the
// tulip_rs_ffi C API: error mapping, indicator metadata, input marshalling,
// and the memory-ownership wrappers (Result, State, SimdResult) that every
// indicator package reuses.
//
// It implements the ownership contract documented in the repo-root
// bindings_memory_model.md:
//
//   - Indicator calls return rows allocated by Rust. Result.Rows are
//     zero-copy read-only views into that memory, valid until Close, which
//     releases the buffers back to Rust. Close never touches the streaming
//     state handle.
//   - State handles are long-lived and are released exactly once via Close.
//   - SimdResult.Close frees every lane state FIRST, then the outer SIMD
//     buffers — that order is contractual.
//   - Serialized blobs are copied into Go memory and the native buffer is
//     freed inside the same call (copy-then-free: cold path, no handle).
//   - Info strings are process-lifetime on the Rust side: copied once,
//     never freed.
//
// Finalizers on Result/State/SimdResult are leak nets, not mechanisms —
// prefer `defer x.Close()`.
//
// All uintptr <-> C-pointer casts are confined to the static-inline helpers
// in the cgo preamble below, so no unsafe.Pointer(uintptr) conversion ever
// appears in Go code, and raw C-memory addresses are only ever stored in
// uintptr fields, which the GC does not scan. (cgo rules: each file may
// only reference C entities declared in ITS OWN preamble, so this is the
// single cgo file of the package; result.go/state.go/simd.go call the
// package-private cgo-backed primitives defined at the bottom.)
package tulip

/*
#cgo CFLAGS: -I${SRCDIR}/../../tulip_rs_ffi/include
#cgo LDFLAGS: -L${SRCDIR}/../../tulip_rs_ffi/target/release -L${SRCDIR}/../../tulip_rs_ffi/target/debug -ltulip_rs_ffi -Wl,-rpath,${SRCDIR}/../../tulip_rs_ffi/target/release -Wl,-rpath,${SRCDIR}/../../tulip_rs_ffi/target/debug -lm -ldl -lpthread
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
// cgo's builtin prologue declares its own `CBytes` helper, which clashes
// with the FFI typedef of that name; rename it for this translation unit
// (seen as C.TulipBytes from Go).
#define CBytes TulipBytes
#include "tulip_rs_ffi.h"

// ---- cast helpers (see package doc) -------------------------------------

static inline void *u2p(uintptr_t p) { return (void *)p; }

// typed reads of the leaked C arrays behind stored addresses
static inline double       **dd_u(uintptr_t p) { return (double **)(void *)p; }
static inline double      ***d3_u(uintptr_t p) { return (double ***)(void *)p; }
static inline size_t        *sz_u(uintptr_t p) { return (size_t *)(void *)p; }
static inline size_t       **sz2_u(uintptr_t p) { return (size_t **)(void *)p; }
static inline const char   **cs_u(uintptr_t p) { return (const char **)(void *)p; }
static inline uintptr_t      slot_u(uintptr_t arr, size_t i) { return ((uintptr_t *)arr)[i]; }

// one-shot free shims: rebuild the C result struct from stored raw parts
static inline void ffi_result_free(uintptr_t outputs, uintptr_t lens, uintptr_t num) {
	CIndicatorResult r;
	r.error = C_INDICATOR_ERROR_OK;
	r.outputs = (double **)outputs;
	r.output_lens = (size_t *)lens;
	r.num_outputs = (size_t)num;
	r.state = NULL;
	tulip_ffi_result_free(r);
}

static inline void ffi_batch_free(uintptr_t outputs, uintptr_t lens, uintptr_t num) {
	CBatchResult r;
	r.error = C_INDICATOR_ERROR_OK;
	r.outputs = (double **)outputs;
	r.output_lens = (size_t *)lens;
	r.num_outputs = (size_t)num;
	tulip_ffi_batch_result_free(r);
}

static inline void ffi_simd_free(uintptr_t outputs, uintptr_t lens, uintptr_t states,
                                 uintptr_t numOut, uintptr_t numRes) {
	CSimdResult r;
	r.error = C_INDICATOR_ERROR_OK;
	r.outputs = (double ***)outputs;
	r.output_lens = (size_t **)lens;
	r.num_outputs = (size_t)numOut;
	r.states = (void **)states;
	r.num_results = (size_t)numRes;
	tulip_ffi_simd_result_free(r);
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// cgo's C.double is a defined type (not an alias) with float64's memory
// layout; export it as CDouble so pure-Go files in this package (and
// indicator packages via tulip.CDouble) can name the view element type.
type CDouble = C.double

// Error mirrors the FFI's CIndicatorError codes.
type Error int32

const (
	ErrInvalidInputs         Error = 1
	ErrNotEnoughData         Error = 2
	ErrInvalidOptions        Error = 3
	ErrInvalidIndicatorState Error = 4
)

func (e Error) Error() string {
	switch e {
	case ErrInvalidInputs:
		return "tulip: invalid inputs (nil/empty series or mismatched lengths)"
	case ErrNotEnoughData:
		return "tulip: not enough data"
	case ErrInvalidOptions:
		return "tulip: invalid options"
	case ErrInvalidIndicatorState:
		return "tulip: invalid indicator state"
	default:
		return fmt.Sprintf("tulip: ffi error %d", int32(e))
	}
}

func checkError(code C.CIndicatorError) error {
	if code == C.C_INDICATOR_ERROR_OK {
		return nil
	}
	return Error(int32(code))
}

// Format selects the state serialisation encoding (FFI CStateFormat).
type Format uint32

const (
	// FormatBincode: compact, handles NaN/Inf — recommended for persistence.
	FormatBincode Format = 0
	// FormatJSON: human-readable for debugging; the FFI rejects non-finite
	// f64s in this format.
	FormatJSON Format = 1
)

// ErrClosed is returned by operations on an already-closed handle.
var ErrClosed = fmt.Errorf("tulip: operation on closed state handle")

// IndicatorType mirrors CIndicatorType as a readable string.
type IndicatorType string

func typeName(code int32) IndicatorType {
	switch code {
	case 0:
		return "Trend"
	case 1:
		return "Momentum"
	case 2:
		return "Volume"
	case 3:
		return "Volatility"
	case 4:
		return "Price"
	case 5:
		return "Cycle"
	case 6:
		return "CandleStick"
	case 7:
		return "Math"
	case 8:
		return "Other"
	default:
		return IndicatorType(fmt.Sprintf("Unknown(%d)", code))
	}
}

// Info is an immutable Go copy of the FFI's CIndicatorInfo. The underlying
// C strings are leaked process-lifetime on the Rust side (copy at init,
// never free) — this type makes the C struct entirely moot.
type Info struct {
	Name            string
	FullName        string
	Type            IndicatorType
	Inputs          []string
	Options         []string
	Outputs         []string
	OptionalOutputs []string
}

// NewInfo assembles Info from primitives, so indicator packages never pass
// cgo struct types across the package boundary (they are per-package).
func NewInfo(typeCode int32, name, fullName string, inputs, options, outputs, optionalOutputs []string) Info {
	return Info{
		Name:            name,
		FullName:        fullName,
		Type:            typeName(typeCode),
		Inputs:          inputs,
		Options:         options,
		Outputs:         outputs,
		OptionalOutputs: optionalOutputs,
	}
}

// CopyStringArray reads a CStringArray (given as a raw address + length)
// into Go strings. addr 0 yields nil. The strings are process-lifetime on
// the Rust side; we copy.
func CopyStringArray(addr uintptr, n int) []string {
	if addr == 0 || n == 0 {
		return nil
	}
	ptrs := unsafe.Slice(C.cs_u(C.uintptr_t(addr)), n)
	out := make([]string, n)
	for i, p := range ptrs {
		out[i] = C.GoString(p)
	}
	return out
}

// MarshalSeries copies one pointer per input series into a C-allocated
// `const double *const *` array suitable for the FFI inputs argument.
//
// The returned release func must run after the FFI call returns; the caller
// must also keep the underlying slices alive across the call (each series
// must be at least dataLen long). Pointers to Go memory are stored in the C
// array only for the duration of the call — standard cgo practice and safe
// with Go's non-moving GC; runtime.KeepAlive at the call site makes the
// lifetime explicit.
func MarshalSeries(series [][]float64, dataLen int) (unsafe.Pointer, func(), error) {
	if dataLen <= 0 {
		return nil, nil, fmt.Errorf("tulip: no input data")
	}
	for i, s := range series {
		if len(s) < dataLen {
			return nil, nil, fmt.Errorf("tulip: input series %d has %d bars, need %d", i, len(s), dataLen)
		}
	}
	n := len(series)
	ptrSize := C.size_t(unsafe.Sizeof(uintptr(0)))
	arr := C.malloc(C.size_t(n) * ptrSize)
	if arr == nil {
		return nil, nil, fmt.Errorf("tulip: out of memory marshalling inputs")
	}
	ptrs := unsafe.Slice((**C.double)(arr), n)
	for i, s := range series {
		ptrs[i] = (*C.double)(unsafe.Pointer(&s[0]))
	}
	return arr, func() { C.free(arr) }, nil
}

// MarshalSimdInputs builds the `const double *const *const *` asset array
// for by-assets SIMD calls: one inner `const double *const *` per asset
// (laid out contiguously) plus an outer array of them. Every asset must
// carry the same number of series, each at least dataLen bars long.
func MarshalSimdInputs(assets [][][]float64, dataLen int) (unsafe.Pointer, func(), error) {
	if len(assets) == 0 {
		return nil, nil, fmt.Errorf("tulip: no SIMD assets")
	}
	k := len(assets[0])
	if k == 0 {
		return nil, nil, fmt.Errorf("tulip: SIMD asset has no series")
	}
	for i, asset := range assets {
		if len(asset) != k {
			return nil, nil, fmt.Errorf("tulip: asset %d has %d series, first has %d", i, len(asset), k)
		}
		for j, s := range asset {
			if len(s) < dataLen {
				return nil, nil, fmt.Errorf("tulip: asset %d series %d has %d bars, need %d", i, j, len(s), dataLen)
			}
		}
	}
	n := len(assets)
	ptrSize := C.size_t(unsafe.Sizeof(uintptr(0)))
	block := C.malloc(C.size_t(n) * C.size_t(k) * ptrSize)
	outer := C.malloc(C.size_t(n) * ptrSize)
	if block == nil || outer == nil {
		if block != nil {
			C.free(block)
		}
		if outer != nil {
			C.free(outer)
		}
		return nil, nil, fmt.Errorf("tulip: out of memory marshalling SIMD inputs")
	}
	ip := unsafe.Slice((**C.double)(block), n*k)
	for i, asset := range assets {
		for j, s := range asset {
			ip[i*k+j] = (*C.double)(unsafe.Pointer(&s[0]))
		}
	}
	op := unsafe.Slice((*unsafe.Pointer)(outer), n)
	base := uintptr(block)
	stride := uintptr(k) * uintptr(ptrSize)
	for i := range op {
		op[i] = C.u2p(C.uintptr_t(base + uintptr(i)*stride))
	}
	return outer, func() { C.free(outer); C.free(block) }, nil
}

// MarshalBools allocates a C bool array for an optional_outputs argument.
// Empty input yields a nil address (the FFI treats NULL as "none").
func MarshalBools(flags []bool) (unsafe.Pointer, func()) {
	if len(flags) == 0 {
		return nil, func() {}
	}
	arr := C.malloc(C.size_t(len(flags)) * C.size_t(unsafe.Sizeof(C.bool(false))))
	ptrs := unsafe.Slice((*C.bool)(arr), len(flags))
	for i, f := range flags {
		ptrs[i] = C.bool(f)
	}
	return arr, func() { C.free(arr) }
}

// ---------------------------------------------------------------------------
// cgo-backed primitives for result.go / state.go / simd.go (the rest of this
// package is pure Go; see the preamble note above).
// ---------------------------------------------------------------------------

func ffiResultFree(outputsAddr, lensAddr uintptr, numOutputs int) {
	C.ffi_result_free(C.uintptr_t(outputsAddr), C.uintptr_t(lensAddr), C.uintptr_t(numOutputs))
}

func ffiBatchFree(outputsAddr, lensAddr uintptr, numOutputs int) {
	C.ffi_batch_free(C.uintptr_t(outputsAddr), C.uintptr_t(lensAddr), C.uintptr_t(numOutputs))
}

func ffiSimdFree(outputsAddr, lensAddr, statesAddr uintptr, numOutputs, numResults int) {
	C.ffi_simd_free(C.uintptr_t(outputsAddr), C.uintptr_t(lensAddr), C.uintptr_t(statesAddr),
		C.uintptr_t(numOutputs), C.uintptr_t(numResults))
}

// checkErr converts an FFI error code (as int32) to a Go error.
func checkErr(code int32) error { return checkError(C.CIndicatorError(code)) }

// rowViews builds the zero-copy [][]C.double view of one result's rows.
// cgo's C.double is a defined type with the same memory layout as float64,
// so rows are byte-compatible with a plain []float64 — cast with
// unsafe.SliceData/len or re-slice if you need float64 explicitly.
func rowViews(outputsAddr, lensAddr uintptr, numOutputs int) [][]CDouble {
	ptrs := unsafe.Slice(C.dd_u(C.uintptr_t(outputsAddr)), numOutputs)
	lens := unsafe.Slice(C.sz_u(C.uintptr_t(lensAddr)), numOutputs)
	rows := make([][]CDouble, numOutputs)
	for i := range rows {
		rows[i] = unsafe.Slice(ptrs[i], int(lens[i]))
	}
	return rows
}

// simdRowViews builds [lane][output][]C.double views, plus the raw per-lane
// state handle addresses.
func simdRowViews(outputsAddr, lensAddr, statesAddr uintptr, numResults, numOutputs int) ([][][]CDouble, []uintptr) {
	laneOuts := unsafe.Slice(C.d3_u(C.uintptr_t(outputsAddr)), numResults)
	laneLens := unsafe.Slice(C.sz2_u(C.uintptr_t(lensAddr)), numResults)
	results := make([][][]CDouble, numResults)
	for i := range results {
		ptrs := unsafe.Slice(laneOuts[i], numOutputs)
		lens := unsafe.Slice(laneLens[i], numOutputs)
		rows := make([][]CDouble, numOutputs)
		for j := range rows {
			rows[j] = unsafe.Slice(ptrs[j], int(lens[j]))
		}
		results[i] = rows
	}
	handles := make([]uintptr, numResults)
	for i := range handles {
		handles[i] = uintptr(C.slot_u(C.uintptr_t(statesAddr), C.size_t(i)))
	}
	return results, handles
}

// --- state operations (cgo calls confined here) ----------------------------

func stateSerialize(id uint32, format uint32, handle uintptr) ([]byte, error) {
	b := C.tulip_state_serialize(C.uint32_t(id), C.uint32_t(format), C.u2p(C.uintptr_t(handle)))
	if b.ptr == nil {
		return nil, fmt.Errorf("tulip: serialize failed (bad state?)")
	}
	out := C.GoBytes(unsafe.Pointer(b.ptr), C.int(C.size_t(b.len)))
	C.tulip_ffi_bytes_free(b)
	return out, nil
}

func stateClone(id uint32, handle uintptr) (uintptr, error) {
	h := C.tulip_state_clone(C.uint32_t(id), C.u2p(C.uintptr_t(handle)))
	if h == nil {
		return 0, fmt.Errorf("tulip: clone failed")
	}
	return uintptr(h), nil
}

func stateDeserialize(blob []byte) (uintptr, error) {
	h := C.tulip_state_deserialize((*C.uint8_t)(unsafe.Pointer(&blob[0])), C.uintptr_t(len(blob)))
	if h == nil {
		return 0, fmt.Errorf("tulip: deserialize failed (corrupt, stale, or foreign blob)")
	}
	return uintptr(h), nil
}

func stateAsArg(handle uintptr) unsafe.Pointer { return C.u2p(C.uintptr_t(handle)) }
