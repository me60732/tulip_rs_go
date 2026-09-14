// Pivotpoint example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/pivotpoint_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{10.0} // period

	info := indicators.Pivotpoint.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Pivotpoint.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, close := series[0], series[1], series[2]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Pivotpoint.Indicator(high, low, close, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for _, name := range info.Outputs {
			fmt.Printf("  %-6s %d values\n", name, len(res.Rows[0]))
		}
		// pivotpoint stores all 7 outputs (s3,s2,s1,pp,r1,r2,r3) in a single row
		fullS3 := append([]tulip.CDouble(nil), res.Rows[0]...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Pivotpoint.Indicator(high[:partial], low[:partial], close[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], close[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				brLen := len(br.Rows[0])
				fullLen := len(fullS3)
				tailStart := fullLen - brLen
				c.Match("partial+continued equals full recompute (all values)", demo.Same(fullS3[tailStart:], br.Rows[0]))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.PivotpointID)
			}
			rs, err := indicators.Pivotpoint.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(high[partial:], low[partial:], close[partial:], nil)
			b2, e2 := rs.Batch(high[partial:], low[partial:], close[partial:], nil)
			b3, e3 := cl.Batch(high[partial:], low[partial:], close[partial:], nil)
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				b2Len := len(b2.Rows[0])
				b1Len := len(b1.Rows[0])
				if b1Len == b2Len {
					c.Match("deserialized state continues identically (all values)", demo.Same(b1.Rows[0], b2.Rows[0]))
				}
			}
			if e1 == nil && e3 == nil {
				b3Len := len(b3.Rows[0])
				b1Len := len(b1.Rows[0])
				if b1Len == b3Len {
					c.Match("cloned state continues identically (all values)", demo.Same(b1.Rows[0], b3.Rows[0]))
				}
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
	assets := [][indicators.PivotpointInputs][]float64{
		{high, low, close},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2)},
	}
	sim, err := indicators.Pivotpoint.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Pivotpoint.Indicator(assets[i][0], assets[i][1], assets[i][2], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (all values)", i+1), demo.Same(sim.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Pivotpoint.SimdByOptions(high, low, close, [][]float64{{5}, {10}, {15}, {20}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{5}, {10}, {15}, {20}} {
			r, s2, err := indicators.Pivotpoint.Indicator(high, low, close, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (all values)", i+1), demo.Same(sim2.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
