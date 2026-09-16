// Package demo holds the tiny shared harness every indicator example uses:
// synthetic series generation, NaN-safe output comparison, and pass/fail
// bookkeeping. (internal/: importable only from the examples subtree.)
package demo

import (
	"fmt"
	"math"
	"os"

	"github.com/me60732/tulip_rs_go/tulip"
)

// Check accumulates assertions; Done prints the verdict and exits non-zero
// on any failure.
type Check struct {
	failed bool
}

func (c *Check) Require(cond bool, msg string) {
	if !cond {
		fmt.Println("  FAIL:", msg)
		c.failed = true
	}
}

func (c *Check) Match(what string, cond bool) {
	if cond {
		fmt.Printf("  MATCH: %s\n", what)
	} else {
		fmt.Printf("  MISMATCH: %s\n", what)
		c.failed = true
	}
}

func (c *Check) Err(context string, err error) {
	if err != nil {
		c.Require(false, fmt.Sprintf("%s: %v", context, err))
	}
}

func (c *Check) Done() {
	if c.failed {
		fmt.Println("\nSOME CHECKS FAILED")
		os.Exit(1)
	}
	fmt.Println("\nALL CHECKS PASSED")
}

// Same compares two zero-copy output rows NaN-safely, bit-exact (NaN==NaN
// counts). Use only where the core guarantees bit-identical results.
func Same(a, b []tulip.CDouble) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		if x != y && !(math.IsNaN(x) && math.IsNaN(y)) {
			return false
		}
	}
	return true
}

// SameTol compares rows with |a-b| <= abs + rel*max(|a|,|b|). SIMD-vs-scalar
// and other approx-equal code paths (reciprocal division, vectorized
// accumulation) legitimately differ by ulps — the core's own tests allow
// this (e.g. ad: approx_eq 1e-2), so examples comparing SIMD output to the
// scalar reference must use a tolerance, not bit-equality.
func SameTol(a, b []tulip.CDouble, rel, abs float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		if math.IsNaN(x) && math.IsNaN(y) {
			continue
		}
		xa, ya := math.Abs(x), math.Abs(y)
		tol := abs + rel*math.Max(xa, ya)
		if math.Abs(x-y) > tol {
			return false
		}
	}
	return true
}

// SeriesFor builds plausible, non-degenerate input series for the named
// FFI inputs (names come from Info().Inputs, e.g. "high", "low", "close",
// "volume", "open", "real"). Oscillating + mild drift keeps filter math
// (atan/sin, division) away from NaN traps that constant or linear data
// would hit.
func SeriesFor(names []string, n int) [][]float64 {
	cs := make([]float64, n)
	for i := range cs {
		t := float64(i)
		cs[i] = 100 + 10*math.Sin(t*0.3) + t*0.05
	}
	off := func(k float64) []float64 {
		o := make([]float64, n)
		for i := range o {
			o[i] = cs[i] + k
		}
		return o
	}
	vol := make([]float64, n)
	for i := range vol {
		vol[i] = 1e6 + float64(i)*137
	}
	lag := append([]float64{cs[0]}, cs[:n-1]...)
	out := make([][]float64, len(names))
	for i, nm := range names {
		switch nm {
		case "high":
			out[i] = off(2)
		case "low":
			out[i] = off(-2)
		case "open":
			out[i] = lag
		case "volume":
			out[i] = vol
		default: // "close", "real"
			out[i] = cs
		}
	}
	return out
}

// Scale multiplies a series by k (for SIMD asset variants).
func Scale(s []float64, k float64) []float64 {
	out := make([]float64, len(s))
	for i, v := range s {
		out[i] = v * k
	}
	return out
}
