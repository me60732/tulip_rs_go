// QStick example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/qstick_example.c, with every step verified.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{10.0} // period

	info := indicators.Qstick.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Qstick.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	open, close := series[0], series[1]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Qstick.Indicator(open, close, options, nil)
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range info.Outputs {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullQstick := append([]tulip.CDouble(nil), res.Rows[0]...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Qstick.Indicator(open[:partial], close[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(open[partial:], close[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tailQstick := fullQstick[len(fullQstick)-len(br.Rows[0]):]
				c.Match("partial+continued equals full recompute (qstick)", demo.Same(tailQstick, br.Rows[0]))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.QstickID)
			}
			rs, err := indicators.Qstick.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(open[partial:], close[partial:], nil)
			b2, e2 := rs.Batch(open[partial:], close[partial:], nil)
			b3, e3 := cl.Batch(open[partial:], close[partial:], nil)
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically (qstick)", demo.Same(b1.Rows[0], b2.Rows[0]))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically (qstick)", demo.Same(b1.Rows[0], b3.Rows[0]))
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
	assets := [][indicators.QstickInputs][]float64{
		{open, close},
		{demo.Scale(open, 1.2), demo.Scale(close, 1.2)},
	}
	sim, err := indicators.Qstick.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Qstick.Indicator(assets[i][0], assets[i][1], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d equals individual (qstick)", i+1), demo.Same(sim.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Qstick.SimdByOptions(open, close, [][]float64{{5}, {10}, {15}, {20}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{5}, {10}, {15}, {20}} {
			r, s2, err := indicators.Qstick.Indicator(open, close, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d equals individual (qstick)", i+1), demo.Same(sim2.Results[i][0], r.Rows[0]))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
