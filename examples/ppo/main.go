// PPO example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/ppo_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{12.0, 26.0} // fast_period, slow_period

	info := indicators.Ppo.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Ppo.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	real := series[0]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Ppo.Indicator(real, options, []bool{true, true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"ppo"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullPpo := append([]tulip.CDouble(nil), res.Rows[0]...)
		fullShortEma := append([]tulip.CDouble(nil), res.Rows[1]...)
		fullLongEma := append([]tulip.CDouble(nil), res.Rows[2]...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Ppo.Indicator(real[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(real[partial:], []bool{true, true})
			c.Err("batch", err)
			if err == nil {
				tailPpo := fullPpo[len(fullPpo)-len(br.Rows[0]):]
				tailShortEma := fullShortEma[len(fullShortEma)-len(br.Rows[1]):]
				tailLongEma := fullLongEma[len(fullLongEma)-len(br.Rows[2]):]
				c.Match("partial+continued equals full recompute (ppo)", demo.SameTol(tailPpo, br.Rows[0], 1e-6, 1e-9))
				c.Match("partial+continued equals full recompute (short_ema)", demo.SameTol(tailShortEma, br.Rows[1], 1e-6, 1e-9))
				c.Match("partial+continued equals full recompute (long_ema)", demo.SameTol(tailLongEma, br.Rows[2], 1e-6, 1e-9))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.PpoID)
			}
			rs, err := indicators.Ppo.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(real[partial:], []bool{true, true})
			b2, e2 := rs.Batch(real[partial:], []bool{true, true})
			b3, e3 := cl.Batch(real[partial:], []bool{true, true})
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically (ppo)", demo.SameTol(b1.Rows[0], b2.Rows[0], 1e-6, 1e-9))
				c.Match("deserialized state continues identically (short_ema)", demo.SameTol(b1.Rows[1], b2.Rows[1], 1e-6, 1e-9))
				c.Match("deserialized state continues identically (long_ema)", demo.SameTol(b1.Rows[2], b2.Rows[2], 1e-6, 1e-9))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically (ppo)", demo.SameTol(b1.Rows[0], b3.Rows[0], 1e-6, 1e-9))
				c.Match("cloned state continues identically (short_ema)", demo.SameTol(b1.Rows[1], b3.Rows[1], 1e-6, 1e-9))
				c.Match("cloned state continues identically (long_ema)", demo.SameTol(b1.Rows[2], b3.Rows[2], 1e-6, 1e-9))
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
	assets := [][indicators.PpoInputs][]float64{
		{real},
		{demo.Scale(real, 1.2)},
	}
	sim, err := indicators.Ppo.SimdByAssets(assets, options, []bool{true, true})
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Ppo.Indicator(assets[i][0], options, []bool{true, true})
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (ppo)", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (short_ema)", i+1), demo.SameTol(sim.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (long_ema)", i+1), demo.SameTol(sim.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Ppo.SimdByOptions(real, [][]float64{{10, 20}, {12, 26}, {15, 30}, {20, 40}}, []bool{true, true})
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{10, 20}, {12, 26}, {15, 30}, {20, 40}} {
			r, s2, err := indicators.Ppo.Indicator(real, o, []bool{true, true})
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (ppo)", i+1), demo.SameTol(sim2.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (short_ema)", i+1), demo.SameTol(sim2.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (long_ema)", i+1), demo.SameTol(sim2.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
