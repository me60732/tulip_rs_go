// BOP example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/bop_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{} // no options for BOP

	info := indicators.Bop.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Bop.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	open, high, low, close := series[0], series[1], series[2], series[3]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation ===")
	res, st, err := indicators.Bop.Indicator(open, high, low, close, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range info.Outputs {
			if i < len(res.Rows) {
				fmt.Printf("  %-16s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullBop := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Bop.Indicator(open[:partial], high[:partial], low[:partial], close[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(open[partial:], high[partial:], low[partial:], close[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tail := fullBop[len(fullBop)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute", demo.SameTol(tulip.AsCDoubles(tail), br.Rows[0], 1e-6, 1e-9))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  serialize failed: %v\n", err)
			} else {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.BopID)
			}
			rs, err := indicators.Bop.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(open[partial:], high[partial:], low[partial:], close[partial:], nil)
			b2, e2 := rs.Batch(open[partial:], high[partial:], low[partial:], close[partial:], nil)
			b3, e3 := cl.Batch(open[partial:], high[partial:], low[partial:], close[partial:], nil)
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically", demo.SameTol(b1.Rows[0], b2.Rows[0], 1e-6, 1e-9))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically", demo.SameTol(b1.Rows[0], b3.Rows[0], 1e-6, 1e-9))
			}
			for _, r := range []*tulip.Result{b1, b2, b3} {
				if r != nil {
					r.Close()
				}
			}
			if rs != nil {
				rs.Close()
			}
			if cl != nil {
				cl.Close()
			}
			st.Close()
		}
	}

	// ---- SIMD by assets ---------------------------------------------------
	fmt.Println("\n=== SIMD by assets (N=2) ===")
	assets := [][indicators.BopInputs][]float64{
		{open, high, low, close},
		{demo.Scale(open, 1.2), demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2)},
	}
	sim, err := indicators.Bop.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Bop.Indicator(assets[i][0], assets[i][1], assets[i][2], assets[i][3], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options skipped (no options) ===")
	c.Done()
}
