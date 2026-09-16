// TRIMA example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/trima_example.c, with every step verified.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{5.0} // period

	info := indicators.Trima.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Trima.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	real := series[0]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation ===")
	res, st, err := indicators.Trima.Indicator(real, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range []string{"trima"} {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullTrima := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Trima.Indicator(real[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(real[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tail := fullTrima[len(fullTrima)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute", demo.SameTol(tulip.AsCDoubles(tail), br.Rows[0], 1e-6, 1e-9))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.TrimaID)
			}
			rs, err := indicators.Trima.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(real[partial:], nil)
			b2, e2 := rs.Batch(real[partial:], nil)
			b3, e3 := cl.Batch(real[partial:], nil)
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
	assets := [][indicators.TrimaInputs][]float64{
		{real},
		{demo.Scale(real, 1.2)},
	}
	sim, err := indicators.Trima.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Trima.Indicator(assets[i][0], options, nil)
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
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Trima.SimdByOptions(real, [][]float64{{3}, {5}, {7}, {10}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{3}, {5}, {7}, {10}} {
			r, s2, err := indicators.Trima.Indicator(real, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual", i+1), demo.SameTol(sim2.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
