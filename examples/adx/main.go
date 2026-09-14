// Command adx demonstrates the tulip_rs_go ADX binding end-to-end, mirroring
// tulip_rs_ffi/examples/adx_example.c: full compute, streaming continuation,
// state persistence (serialize/deserialize/clone), and both SIMD modes — with
// every step verified against a reference and every native handle closed.
package main

import (
	"fmt"
	"os"

	"tulip_rs_go/adx"
	"tulip_rs_go/tulip"
)

var (
	high   = []float64{82.15, 81.89, 83.03, 83.30, 83.85, 83.90, 83.33, 84.30, 84.84, 85.00, 85.90, 86.58, 86.98, 88.00, 87.87}
	low    = []float64{81.29, 80.64, 81.31, 82.65, 83.07, 83.11, 82.49, 82.30, 84.15, 84.11, 84.03, 85.39, 85.76, 87.17, 87.01}
	close_ = []float64{81.59, 81.06, 82.87, 83.00, 83.61, 83.15, 82.84, 83.99, 84.55, 84.36, 85.53, 86.54, 86.89, 87.77, 87.29}
)

const total = 15

var failed bool

func require(cond bool, msg string) {
	if !cond {
		fmt.Println("  FAIL:", msg)
		failed = true
	}
}

func rowPrefix(name string) string { return name }

func main() {
	options := []float64{5.0} // period

	// ---- metadata -------------------------------------------------------
	info := adx.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %d %v, Options: %d %v, Optional outputs: %v\n",
		adx.Inputs, info.Inputs, adx.Options, info.Options, info.OptionalOutputs)
	fmt.Printf("Type: %s, min bars for period 5: %d\n", info.Type, adx.MinData(options))

	// keep a reference copy of the full adx row for later verification
	res, st, err := adx.Indicator(high, low, close_, options, []bool{true, true, true})
	require(err == nil, fmt.Sprint("full indicator:", err))
	fullAdx := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...) // copy: rows are views
	fmt.Printf("\n=== full calculation (all optional outputs) ===\n")
	for i, name := range []string{"adx", "dx", "atr", "tr"} {
		fmt.Printf("  %-6s %d values: %.4f .. %.4f\n", rowPrefix(name), len(res.Rows[i]), res.Rows[i][0], res.Rows[i][len(res.Rows[i])-1])
	}
	res.Close()
	st.Close()

	// ---- partial + batch continuation ------------------------------------
	fmt.Println("\n=== partial calculation + batch continuation ===")
	partial := total - 5
	res, st, err = adx.Indicator(high[:partial], low[:partial], close_[:partial], options, nil)
	require(err == nil, fmt.Sprint("partial indicator:", err))
	fmt.Printf("  partial adx: %.4f (%d values)\n", res.Rows[0][0], len(res.Rows[0]))
	res.Close()

	br, err := st.Batch(high[partial:], low[partial:], close_[partial:], nil)
	require(err == nil, fmt.Sprint("batch:", err))
	fmt.Printf("  continued adx: %.4f (%d values)\n", br.Rows[0][0], len(br.Rows[0]))
	tail := fullAdx[len(fullAdx)-len(br.Rows[0]):]
	require(eqFloats(tail, br.Rows[0]), "partial+continued != full recompute")
	if !failed {
		fmt.Println("  MATCH: partial+continued equals full recompute")
	}
	br.Close()
	st.Close()

	// ---- persistence: serialize / deserialize / clone ---------------------
	fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
	res, st, err = adx.Indicator(high[:partial], low[:partial], close_[:partial], options, nil)
	require(err == nil, fmt.Sprint("persist-partial indicator:", err))
	res.Close() // outputs released; st keeps the streaming state alive

	blob, err := st.Serialize(tulip.FormatBincode)
	require(err == nil, fmt.Sprint("serialize:", err))
	fmt.Printf("  bincode blob: %d bytes (TRFS header, indicator id 0x%08x)\n", len(blob), adx.ID)

	rs, err := adx.DeserializeState(blob)
	require(err == nil, fmt.Sprint("deserialize:", err))

	jsonBlob, err := st.Serialize(tulip.FormatJSON)
	require(err == nil, fmt.Sprint("serialize json:", err))
	fmt.Printf("  json blob (%d bytes): %.60s...\n", len(jsonBlob), jsonBlob)

	// Clone BEFORE continuing the stream: the clone snapshots the state at
	// the current bar, so all three handles continue from the same point.
	cl, err := st.Clone()
	require(err == nil, fmt.Sprint("clone:", err))

	b1, err := st.Batch(high[partial:], low[partial:], close_[partial:], nil)
	require(err == nil, fmt.Sprint("batch original:", err))
	b2, err := rs.Batch(high[partial:], low[partial:], close_[partial:], nil)
	require(err == nil, fmt.Sprint("batch restored:", err))
	b3, err := cl.Batch(high[partial:], low[partial:], close_[partial:], nil)
	require(err == nil, fmt.Sprint("batch clone:", err))

	require(eq(b1.Rows[0], b2.Rows[0]), "deserialized state diverges")
	fmt.Println("  MATCH: deserialized state continues identically")
	require(eq(b1.Rows[0], b3.Rows[0]), "cloned state diverges")
	fmt.Println("  MATCH: cloned state continues identically")
	require(eqFloats(fullAdx[len(fullAdx)-len(b1.Rows[0]):], b1.Rows[0]), "continuation != full recompute")
	fmt.Println("  MATCH: persisted continuations equal the full recompute tail")
	b1.Close()
	b2.Close()
	b3.Close()
	rs.Close()
	cl.Close()
	st.Close()

	// ---- SIMD by assets ----------------------------------------------------
	fmt.Println("\n=== SIMD by assets (N=4) ===")
	assets := buildAssets()
	sim, err := adx.SimdByAssets(assets, options, nil)
	require(err == nil, fmt.Sprint("simd by assets:", err))
	for i := range sim.Results {
		fmt.Printf("  asset %d: %d values\n", i+1, len(sim.Results[i][0]))
	}
	fmt.Println("  verification vs individual:")
	for i, a := range assets {
		r, s2, err := adx.Indicator(a[0], a[1], a[2], options, nil)
		require(err == nil, fmt.Sprint("individual asset:", err))
		require(eq(sim.Results[i][0], r.Rows[0]), fmt.Sprintf("SIMD asset %d != individual", i+1))
		fmt.Printf("    asset %d: MATCH\n", i+1)
		r.Close()
		s2.Close()
	}
	sim.Close() // frees all lane states first, then the SIMD buffers

	// ---- SIMD by options ---------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	hl20, ll20, c20 := tile(20)
	sets := [][]float64{{3}, {5}, {7}, {10}}
	sim2, err := adx.SimdByOptions(hl20, ll20, c20, sets, nil)
	require(err == nil, fmt.Sprint("simd by options:", err))
	for i := range sim2.Results {
		fmt.Printf("  option set %d: %d values\n", i+1, len(sim2.Results[i][0]))
	}
	for i, o := range sets {
		r, s2, err := adx.Indicator(hl20, ll20, c20, o, nil)
		require(err == nil, fmt.Sprint("individual option set:", err))
		require(eq(sim2.Results[i][0], r.Rows[0]), fmt.Sprintf("SIMD option set %d != individual", i+1))
		fmt.Printf("    option set %d: MATCH\n", i+1)
		r.Close()
		s2.Close()
	}
	sim2.Close()

	if failed {
		fmt.Println("\nSOME CHECKS FAILED")
		os.Exit(1)
	}
	fmt.Println("\nALL CHECKS PASSED")
}

func eq(a, b []tulip.CDouble) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] { // bit-identical expected: same inputs, same code
			return false
		}
	}
	return true
}

func eqFloats(a []float64, b []tulip.CDouble) bool {
	return eq(tulip.AsCDoubles(a), b)
}

func buildAssets() [][adx.Inputs][]float64 {
	h2 := scale(high, 1.2)
	l2 := scale(low, 1.2)
	c2 := scale(close_, 1.2)
	h3, l3, c3 := trend(90.0, 0.5, 0.1)
	h4, l4, c4 := trend(100.0, -0.3, 0.05)
	return [][adx.Inputs][]float64{{high, low, close_}, {h2, l2, c2}, {h3, l3, c3}, {h4, l4, c4}}
}

func scale(xs []float64, k float64) []float64 {
	out := make([]float64, len(xs))
	for i, x := range xs {
		out[i] = x * k
	}
	return out
}

func trend(base, slope, mix float64) ([]float64, []float64, []float64) {
	h := make([]float64, total)
	l := make([]float64, total)
	c := make([]float64, total)
	for i := 0; i < total; i++ {
		b := base + float64(i)*slope
		h[i] = b + high[i]*mix
		l[i] = b + low[i]*mix
		c[i] = b + close_[i]*mix
	}
	return h, l, c
}

func tile(n int) ([]float64, []float64, []float64) {
	h := make([]float64, 0, total*n)
	l := make([]float64, 0, total*n)
	c := make([]float64, 0, total*n)
	for i := 0; i < n; i++ {
		h = append(h, high...)
		l = append(l, low...)
		c = append(c, close_...)
	}
	return h, l, c
}
