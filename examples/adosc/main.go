// ADOSC example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/adosc_example.c, with every step verified.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{3.0, 10.0} // short_period, long_period

	info := indicators.Adosc.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Adosc.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, close, volume := series[0], series[1], series[2], series[3]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (no optional outputs) ===")
	res, st, err := indicators.Adosc.Indicator(high, low, close, volume, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range info.Outputs {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullAdosc := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Adosc.Indicator(high[:partial], low[:partial], close[:partial], volume[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], close[partial:], volume[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tail := fullAdosc[len(fullAdosc)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute", demo.Same(tulip.AsCDoubles(tail), br.Rows[0]))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.AdoscID)
			}
			rs, err := indicators.Adosc.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(high[partial:], low[partial:], close[partial:], volume[partial:], nil)
			b2, e2 := rs.Batch(high[partial:], low[partial:], close[partial:], volume[partial:], nil)
			b3, e3 := cl.Batch(high[partial:], low[partial:], close[partial:], volume[partial:], nil)
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically", demo.Same(b1.Rows[0], b2.Rows[0]))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically", demo.Same(b1.Rows[0], b3.Rows[0]))
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
	assets := [][indicators.AdoscInputs][]float64{
		{high, low, close, volume},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2), demo.Scale(volume, 1.2)},
	}
	sim, err := indicators.Adosc.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Adosc.Indicator(assets[i][0], assets[i][1], assets[i][2], assets[i][3], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				// SIMD vs scalar legitimately differ by ulps here (the core
				// tests for ad/adosc allow approx_eq): compare with tolerance.
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (tol)", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Adosc.SimdByOptions(high, low, close, volume, [][]float64{{3.0, 10.0}, {5.0, 15.0}, {7.0, 20.0}, {10.0, 25.0}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{3.0, 10.0}, {5.0, 15.0}, {7.0, 20.0}, {10.0, 25.0}} {
			r, s2, err := indicators.Adosc.Indicator(high, low, close, volume, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual", i+1), demo.Same(sim2.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
