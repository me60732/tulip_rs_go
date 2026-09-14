package tulip

import (
	"runtime"
	"unsafe"
)

// SimdResult owns one CSimdResult: N lanes of output rows plus N state
// handles (one resumable streaming state per lane — asset or option set).
//
// Close frees every lane state FIRST and then the outer SIMD buffers: that
// order is contractual (tulip_ffi_simd_result_free walks states[i], they
// must still be valid). The lane States are returned closed-after-SimdResult
// Close; do not also close them individually.
//
// Results lanes, like Result.Rows, are zero-copy views valid until Close.
type SimdResult struct {
	// Results is [lane][output][]CDouble: each lane's output rows in the
	// indicator's output order (zero-copy views; see Result's docs).
	Results [][][]CDouble

	// States are the per-lane streaming handles (no finalizers; owned).
	States []*State

	outputsAddr uintptr
	lensAddr    uintptr
	statesAddr  uintptr
	numOutputs  int
	numResults  int
	closed      bool
}

// NewSimdResult wraps the raw fields of a CSimdResult (plain addresses; the
// indicator package supplies its id + state-free function for the lanes).
func NewSimdResult(errCode int32, outputsAddr, lensAddr, statesAddr unsafe.Pointer,
	numResults, numOutputs int, id uint32, stateFree func(uintptr)) (*SimdResult, error) {

	if err := checkErr(errCode); err != nil {
		return nil, err
	}
	o, l, s := uintptr(outputsAddr), uintptr(lensAddr), uintptr(statesAddr)
	results, handles := simdRowViews(o, l, s, numResults, numOutputs)
	states := make([]*State, numResults)
	for i, h := range handles {
		states[i] = newStateNoFinalizer(id, h, stateFree)
	}
	sr := &SimdResult{
		Results:     results,
		States:      states,
		outputsAddr: o,
		lensAddr:    l,
		statesAddr:  s,
		numOutputs:  numOutputs,
		numResults:  numResults,
	}
	runtime.SetFinalizer(sr, (*SimdResult).Close)
	return sr, nil
}

// NumResults returns the number of SIMD lanes.
func (s *SimdResult) NumResults() int { return len(s.Results) }

// Close releases lane states (first) and the SIMD buffers, then the lane
// handles. Idempotent.
func (s *SimdResult) Close() error {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	runtime.SetFinalizer(s, nil)
	for _, st := range s.States {
		st.Close() // marks closed; frees the boxed lane state
	}
	ffiSimdFree(s.outputsAddr, s.lensAddr, s.statesAddr, s.numOutputs, s.numResults)
	return nil
}
