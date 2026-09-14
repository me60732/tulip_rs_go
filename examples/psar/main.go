// PSAR example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/psar_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{0.02, 0.2} // acceleration_factor, maximum

	info := indicators.Psar.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Psar.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low := series[0], series[1]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Psar.Indicator(high, low, options, []bool{true})
	c.Err("full indicator", err)
	if err == nil {
		for _, name := range info.Outputs {
			fmt.Printf("  %-6s %d values\n", name, len(res.Rows[0]))
		}
		fullPsar := append([]tulip.CDouble(nil), res.Rows[0]...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Psar.Indicator(high[:partial], low[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], []bool{true})
			c.Err("batch", err)
			if err == nil {
				tailPsar := fullPsar[len(fullPsar)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute (psar)", demo.SameTol(tailPsar, br.Rows[0], 1e-6, 1e-9))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.PsarID)
			}
			rs, err := indicators.Psar.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(high[partial:], low[partial:], []bool{true})
			b2, e2 := rs.Batch(high[partial:], low[partial:], []bool{true})
			b3, e3 := cl.Batch(high[partial:], low[partial:], []bool{true})
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically (psar)", demo.SameTol(b1.Rows[0], b2.Rows[0], 1e-6, 1e-9))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically (psar)", demo.SameTol(b1.Rows[0], b3.Rows[0], 1e-6, 1e-9))
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
	assets := [][indicators.PsarInputs][]float64{
		{high, low},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2)},
	}
	sim, err := indicators.Psar.SimdByAssets(assets, options, []bool{true})
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Psar.Indicator(assets[i][0], assets[i][1], options, []bool{true})
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (psar)", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Psar.SimdByOptions(high, low, [][]float64{{0.01, 0.2}, {0.02, 0.2}, {0.03, 0.2}, {0.05, 0.2}}, []bool{true})
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{0.01, 0.2}, {0.02, 0.2}, {0.03, 0.2}, {0.05, 0.2}} {
			r, s2, err := indicators.Psar.Indicator(high, low, o, []bool{true})
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (psar)", i+1), demo.SameTol(sim2.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
