// Elderray example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/elderray_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{5.0} // period

	info := indicators.Elderray.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Elderray.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, close := series[0], series[1], series[2]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Elderray.Indicator(high, low, close, options, []bool{true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"bull", "bear"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullBull := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		fullBear := append([]float64(nil), tulip.AsFloat64(res.Rows[1])...)
		var fullEma []float64
		if len(res.Rows) > 2 {
			fullEma = append([]float64(nil), tulip.AsFloat64(res.Rows[2])...)
		}
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Elderray.Indicator(high[:partial], low[:partial], close[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], close[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				bullTail := fullBull[len(fullBull)-len(br.Rows[0]):]
				bearTail := fullBear[len(fullBear)-len(br.Rows[1]):]
				c.Match("partial+continued bull equals full recompute", demo.Same(tulip.AsCDoubles(bullTail), br.Rows[0]))
				c.Match("partial+continued bear equals full recompute", demo.Same(tulip.AsCDoubles(bearTail), br.Rows[1]))
				if len(br.Rows) > 2 && len(fullEma) > 0 {
					emaTail := fullEma[len(fullEma)-len(br.Rows[2]):]
					c.Match("partial+continued ema equals full recompute", demo.Same(tulip.AsCDoubles(emaTail), br.Rows[2]))
				}
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.ElderrayID)
			}
			rs, err := indicators.Elderray.DeserializeState(blob)
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
				c.Match("deserialized state bull continues identically", demo.Same(b1.Rows[0], b2.Rows[0]))
				c.Match("deserialized state bear continues identically", demo.Same(b1.Rows[1], b2.Rows[1]))
				if len(b1.Rows) > 2 && len(b2.Rows) > 2 {
					c.Match("deserialized state ema continues identically", demo.Same(b1.Rows[2], b2.Rows[2]))
				}
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state bull continues identically", demo.Same(b1.Rows[0], b3.Rows[0]))
				c.Match("cloned state bear continues identically", demo.Same(b1.Rows[1], b3.Rows[1]))
				if len(b1.Rows) > 2 && len(b3.Rows) > 2 {
					c.Match("cloned state ema continues identically", demo.Same(b1.Rows[2], b3.Rows[2]))
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
	assets := [][indicators.ElderrayInputs][]float64{
		{high, low, close},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2)},
	}
	sim, err := indicators.Elderray.SimdByAssets(assets, options, []bool{true})
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Elderray.Indicator(assets[i][0], assets[i][1], assets[i][2], options, []bool{true})
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d bull equals individual", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d bear equals individual", i+1), demo.SameTol(sim.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				if len(sim.Results[i]) > 2 && len(r.Rows) > 2 {
					c.Match(fmt.Sprintf("SIMD asset %d ema equals individual", i+1), demo.SameTol(sim.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				}
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Elderray.SimdByOptions(high, low, close, [][]float64{{3}, {5}, {7}, {10}}, []bool{true})
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{3}, {5}, {7}, {10}} {
			r, s2, err := indicators.Elderray.Indicator(high, low, close, o, []bool{true})
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d bull equals individual", i+1), demo.SameTol(sim2.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d bear equals individual", i+1), demo.SameTol(sim2.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				if len(sim2.Results[i]) > 2 && len(r.Rows) > 2 {
					c.Match(fmt.Sprintf("SIMD option set %d ema equals individual", i+1), demo.SameTol(sim2.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				}
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
