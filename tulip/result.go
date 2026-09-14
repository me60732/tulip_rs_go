package tulip

import (
	"runtime"
	"unsafe"
)

// CDouble (see tulip.go) is cgo's C.double — a defined type sharing
// float64's memory layout. Zero-copy output views use it; AsFloat64 below
// reinterprets a row as plain []float64 when that's wanted.

// Result owns the Rust-allocated output buffers of one indicator or batch
// call. Rows are zero-copy read-only views into that memory: they are valid
// exactly until Close returns — treat a Result like an open file (the GC
// cannot protect reads-after-Close, see bindings_memory_model.md §5).
//
// Close releases the output buffers only. The streaming state handle
// (StateHandle, set for indicator calls, 0 for batch calls) is deliberately
// NOT freed: it is long-lived and released via its own State.Close.
type Result struct {
	// Rows holds one read-only view per output series, in the indicator's
	// output order (mandatory outputs first, then enabled optional ones).
	Rows [][]CDouble

	// StateHandle is the raw streaming-state address returned alongside
	// this result (0 for batch results). Indicator packages wrap it.
	StateHandle uintptr

	outputsAddr uintptr
	lensAddr    uintptr
	numOutputs  int
	isBatch     bool
	closed      bool
}

// NewResult wraps the raw fields of a CIndicatorResult (passed as plain
// addresses so indicator packages never move cgo struct types across the
// package boundary). A non-OK error code frees nothing (error results carry
// null buffers) and returns the error.
func NewResult(errCode int32, outputsAddr, lensAddr unsafe.Pointer, numOutputs int, stateHandle uintptr) (*Result, error) {
	return newResult(errCode, outputsAddr, lensAddr, numOutputs, stateHandle, false)
}

// NewBatchResult wraps the raw fields of a CBatchResult (no state handle).
func NewBatchResult(errCode int32, outputsAddr, lensAddr unsafe.Pointer, numOutputs int) (*Result, error) {
	return newResult(errCode, outputsAddr, lensAddr, numOutputs, 0, true)
}

func newResult(errCode int32, outputsAddr, lensAddr unsafe.Pointer, numOutputs int, stateHandle uintptr, isBatch bool) (*Result, error) {
	if err := checkErr(errCode); err != nil {
		return nil, err
	}
	o, l := uintptr(outputsAddr), uintptr(lensAddr)
	r := &Result{
		Rows:        rowViews(o, l, numOutputs),
		StateHandle: stateHandle,
		outputsAddr: o,
		lensAddr:    l,
		numOutputs:  numOutputs,
		isBatch:     isBatch,
	}
	runtime.SetFinalizer(r, (*Result).Close)
	return r, nil
}

// NumOutputs returns the number of output series.
func (r *Result) NumOutputs() int { return len(r.Rows) }

// AsFloat64 reinterprets a zero-copy output row as []float64 (identical
// memory; C.double is a defined type, not an alias). The result stays a
// view into Rust memory with the same validity window as the row argument.
func AsFloat64(row []CDouble) []float64 {
	if len(row) == 0 {
		return nil
	}
	return unsafe.Slice((*float64)(unsafe.Pointer(&row[0])), len(row))
}

// AsCDoubles is the inverse of AsFloat64, for comparing Go-owned series
// against zero-copy output views.
func AsCDoubles(row []float64) []CDouble {
	if len(row) == 0 {
		return nil
	}
	return unsafe.Slice((*CDouble)(unsafe.Pointer(&row[0])), len(row))
}

// Close releases the output buffers back to Rust. Idempotent.
func (r *Result) Close() error {
	if r == nil || r.closed {
		return nil
	}
	r.closed = true
	runtime.SetFinalizer(r, nil)
	if r.isBatch {
		ffiBatchFree(r.outputsAddr, r.lensAddr, r.numOutputs)
	} else {
		ffiResultFree(r.outputsAddr, r.lensAddr, r.numOutputs)
	}
	return nil
}
