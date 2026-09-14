// EMV example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and SIMD by assets — emv has no simd_by_options.
// The Go mirror of tulip_rs_ffi/examples/emv_example.c, with every step verified.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

func main() {
	var c demo.Check

	info := indicators.Emv.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Emv.MinData([]float64{})) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, volume := series[0], series[1], series[2]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Emv.Indicator(high, low, volume, []float64{}, []bool{true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"emv"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-6s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullEmv := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		var fullMedprice []float64
		if len(res.Rows) > 1 {
			fullMedprice = append([]float64(nil), tulip.AsFloat64(res.Rows[1])...)
		}
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Emv.Indicator(high[:partial], low[:partial], volume[:partial], []float64{}, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], volume[partial:], []bool{true})
			c.Err("batch", err)
			if err == nil {
				emvTail := fullEmv[len(fullEmv)-len(br.Rows[0]):]
				c.Match("partial+continued emv equals full recompute", demo.Same(tulip.AsCDoubles(emvTail), br.Rows[0]))
				if len(br.Rows) > 1 && len(fullMedprice) > 0 {
					medpriceTail := fullMedprice[len(fullMedprice)-len(br.Rows[1]):]
					c.Match("partial+continued medprice equals full recompute", demo.Same(tulip.AsCDoubles(medpriceTail), br.Rows[1]))
				}
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.EmvID)
			}
			rs, err := indicators.Emv.DeserializeState(blob)
			c.Err("deserialize", err)
			cl, err := st.Clone()
			c.Err("clone", err)

			b1, e1 := st.Batch(high[partial:], low[partial:], volume[partial:], []bool{true})
			b2, e2 := rs.Batch(high[partial:], low[partial:], volume[partial:], []bool{true})
			b3, e3 := cl.Batch(high[partial:], low[partial:], volume[partial:], []bool{true})
			c.Err("batch original", e1)
			c.Err("batch restored", e2)
			c.Err("batch clone", e3)
			if e1 == nil && e2 == nil {
				c.Match("deserialized state emv continues identically", demo.Same(b1.Rows[0], b2.Rows[0]))
				if len(b1.Rows) > 1 && len(b2.Rows) > 1 {
					c.Match("deserialized state medprice continues identically", demo.Same(b1.Rows[1], b2.Rows[1]))
				}
			}
			if e1 == nil && e3 == nil {
				c.Match("cloned state emv continues identically", demo.Same(b1.Rows[0], b3.Rows[0]))
				if len(b1.Rows) > 1 && len(b3.Rows) > 1 {
					c.Match("cloned state medprice continues identically", demo.Same(b1.Rows[1], b3.Rows[1]))
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
	assets := [][indicators.EmvInputs][]float64{
		{high, low, volume},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(volume, 1.2)},
	}
	sim, err := indicators.Emv.SimdByAssets(assets, []float64{}, []bool{true})
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Emv.Indicator(assets[i][0], assets[i][1], assets[i][2], []float64{}, []bool{true})
			c.Err("individual asset", err)
			if err == nil {
				c.Match(fmt.Sprintf("SIMD asset %d emv equals individual", i+1), demo.SameTol(sim.Results[i][0], r.Rows[0], 1e-6, 1e-9))
				if len(sim.Results[i]) > 1 && len(r.Rows) > 1 {
					c.Match(fmt.Sprintf("SIMD asset %d medprice equals individual", i+1), demo.SameTol(sim.Results[i][1], r.Rows[1], 1e-6, 1e-9))
				}
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	c.Done()
}
