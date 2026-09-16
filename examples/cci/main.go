// CCI example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/cci_example.c, with every step verified.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{20.0} // period

	info := indicators.Cci.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Cci.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, close := series[0], series[1], series[2]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Cci.Indicator(high, low, close, options, []bool{true, true, true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"cci"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-16s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullCci := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		fullSma := append([]float64(nil), tulip.AsFloat64(res.Rows[1])...)
		fullMd := append([]float64(nil), tulip.AsFloat64(res.Rows[2])...)
		fullTypprice := append([]float64(nil), tulip.AsFloat64(res.Rows[3])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Cci.Indicator(high[:partial], low[:partial], close[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], close[partial:], []bool{true, true, true})
			c.Err("batch", err)
			if err == nil {
				tailCci := fullCci[len(fullCci)-len(br.Rows[0]):]
				tailSma := fullSma[len(fullSma)-len(br.Rows[0]):]
				tailMd := fullMd[len(fullMd)-len(br.Rows[0]):]
				tailTypprice := fullTypprice[len(fullTypprice)-len(br.Rows[0]):]
				c.Match("partial+continued cci equals full recompute", demo.SameTol(tulip.AsCDoubles(tailCci), br.Rows[0], 1e-6, 1e-9))
				c.Match("partial+continued sma equals full recompute", demo.SameTol(tulip.AsCDoubles(tailSma), br.Rows[1], 1e-6, 1e-9))
				c.Match("partial+continued md equals full recompute", demo.SameTol(tulip.AsCDoubles(tailMd), br.Rows[2], 1e-6, 1e-9))
				c.Match("partial+continued typprice equals full recompute", demo.SameTol(tulip.AsCDoubles(tailTypprice), br.Rows[3], 1e-6, 1e-9))
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err != nil {
				fmt.Printf("  serialize failed: %v\n", err)
			} else {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.CciID)
			}
			rs, err := indicators.Cci.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(high[partial:], low[partial:], close[partial:], []bool{true, true, true})
			b2, e2 := rs.Batch(high[partial:], low[partial:], close[partial:], []bool{true, true, true})
			b3, e3 := cl.Batch(high[partial:], low[partial:], close[partial:], []bool{true, true, true})
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state continues identically cci", demo.SameTol(b1.Rows[0], b2.Rows[0], 1e-6, 1e-9))
				c.Match("deserialized state continues identically sma", demo.SameTol(b1.Rows[1], b2.Rows[1], 1e-6, 1e-9))
				c.Match("deserialized state continues identically md", demo.SameTol(b1.Rows[2], b2.Rows[2], 1e-6, 1e-9))
				c.Match("deserialized state continues identically typprice", demo.SameTol(b1.Rows[3], b2.Rows[3], 1e-6, 1e-9))
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state continues identically cci", demo.SameTol(b1.Rows[0], b3.Rows[0], 1e-6, 1e-9))
				c.Match("cloned state continues identically sma", demo.SameTol(b1.Rows[1], b3.Rows[1], 1e-6, 1e-9))
				c.Match("cloned state continues identically md", demo.SameTol(b1.Rows[2], b3.Rows[2], 1e-6, 1e-9))
				c.Match("cloned state continues identically typprice", demo.SameTol(b1.Rows[3], b3.Rows[3], 1e-6, 1e-9))
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
	assets := [][indicators.CciInputs][]float64{
		{high, low, close},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2)},
	}
	sim, err := indicators.Cci.SimdByAssets(assets, options, []bool{true, true, true})
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Cci.Indicator(assets[i][0], assets[i][1], assets[i][2], options, []bool{true, true, true})
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d cci equals individual", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d sma equals individual", i+1), demo.SameTol(sim.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d md equals individual", i+1), demo.SameTol(sim.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD asset %d typprice equals individual", i+1), demo.SameTol(sim.Results[i][3], r.Rows[3], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	sim2, err := indicators.Cci.SimdByOptions(high, low, close, [][]float64{{15}, {20}, {25}, {30}}, []bool{true, true, true})
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{15}, {20}, {25}, {30}} {
			r, s2, err := indicators.Cci.Indicator(high, low, close, o, []bool{true, true, true})
			c.Err("individual option set", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD option set %d cci equals individual", i+1), demo.SameTol(sim2.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d sma equals individual", i+1), demo.SameTol(sim2.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d md equals individual", i+1), demo.SameTol(sim2.Results[i][2], r.Rows[2], 1e-6, 1e-9))
				c.Match(fmt.Sprintf("SIMD option set %d typprice equals individual", i+1), demo.SameTol(sim2.Results[i][3], r.Rows[3], 1e-6, 1e-9))
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
