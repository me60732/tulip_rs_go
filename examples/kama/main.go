// KAMA example: full compute, streaming continuation, state persistence
// (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/kama_example.c.
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

	info := indicators.Kama.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Kama.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	real := series[0]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Kama.Indicator(real, options, []bool{true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"kama"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-16s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullKama := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Kama.Indicator(real[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(real[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tailLen := len(br.Rows[0])
				tailStart := len(fullKama) - tailLen
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("partial+continued kama[%d] equals full recompute", i),
						demo.SameTol(tulip.AsCDoubles([]float64{fullKama[tailStart+i]}), br.Rows[0][i:i+1], 1e-6, 1e-9))
				}
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.KamaID)
			}
			rs, err := indicators.Kama.DeserializeState(blob)
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
				tailLen := len(b1.Rows[0])
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("deserialized state continues kama[%d] identically", i),
						demo.SameTol(b1.Rows[0][i:i+1], b2.Rows[0][i:i+1], 1e-6, 1e-9))
				}
			}
			if e1 == nil && e3 == nil {
				tailLen := len(b1.Rows[0])
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("cloned state continues kama[%d] identically", i),
						demo.SameTol(b1.Rows[0][i:i+1], b3.Rows[0][i:i+1], 1e-6, 1e-9))
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
	assets := [][indicators.KamaInputs][]float64{
		{real},
		{demo.Scale(real, 1.2)},
	}
	sim, err := indicators.Kama.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Kama.Indicator(assets[i][0], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				tailLen := len(sim.Results[i][0])
				for j := 0; j < tailLen; j++ {
					c.Match(fmt.Sprintf("SIMD asset %d kama[%d] equals individual", i+1, j),
						demo.SameTol(sim.Results[i][0][j:j+1], r.Rows[0][j:j+1], 1e-6, 1e-9))
				}
				r.Close()
				s2.Close()
			}
		}
		sim.Close() // frees lane states first, then the SIMD buffers
	}

	// ---- SIMD by options --------------------------------------------------
	fmt.Println("\n=== SIMD by options (N=4) ===")
	// Tile to ensure longer-period option sets have enough data.
	tiledLen := n * 3
	realTiled := make([]float64, tiledLen)
	for i := 0; i < 3; i++ {
		copy(realTiled[i*n:(i+1)*n], real)
	}
	sim2, err := indicators.Kama.SimdByOptions(realTiled,
		[][]float64{{3.0}, {5.0}, {7.0}, {10.0}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{3.0}, {5.0}, {7.0}, {10.0}} {
			r, s2, err := indicators.Kama.Indicator(realTiled, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				tailLen := len(sim2.Results[i][0])
				for j := 0; j < tailLen; j++ {
					c.Match(fmt.Sprintf("SIMD option set %d kama[%d] equals individual", i+1, j),
						demo.SameTol(sim2.Results[i][0][j:j+1], r.Rows[0][j:j+1], 1e-6, 1e-9))
				}
				r.Close()
				s2.Close()
			}
		}
		sim2.Close()
	}

	c.Done()
}
