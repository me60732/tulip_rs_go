// PVI example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and SIMD by assets — the Go mirror of
// tulip_rs_ffi/examples/pvi_example.c, with every step verified.
// Note: PVI has NO options (PVI_OPTIONS = 0), so it does NOT have simd_by_options.
// Only the SIMD by-assets section is included.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{} // no options for PVI

	info := indicators.Pvi.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Pvi.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	close, volume := series[0], series[1]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Pvi.Indicator(close, volume, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range info.Outputs {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullPvi := append([]tulip.CDouble(nil), res.Rows[0]...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Pvi.Indicator(close[:partial], volume[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(close[partial:], volume[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tailPvi := fullPvi[len(fullPvi)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute (pvi)", demo.Same(tailPvi, br.Rows[0]))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.PviID)
			}
			rs, err := indicators.Pvi.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(close[partial:], volume[partial:], nil)
			b2, e2 := rs.Batch(close[partial:], volume[partial:], nil)
			b3, e3 := cl.Batch(close[partial:], volume[partial:], nil)
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically (pvi)", demo.Same(b1.Rows[0], b2.Rows[0]))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically (pvi)", demo.Same(b1.Rows[0], b3.Rows[0]))
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
	assets := [][indicators.PviInputs][]float64{
		{close, volume},
		{demo.Scale(close, 1.2), demo.Scale(volume, 1.2)},
	}
	sim, err := indicators.Pvi.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Pvi.Indicator(assets[i][0], assets[i][1], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (pvi)", i+1), demo.Same(sim.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	c.Done()
}
