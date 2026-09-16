// KVO (Klinger Volume Oscillator) example: full compute, streaming continuation,
// state persistence (serialize/deserialize/clone), and both SIMD modes — the Go mirror of
// tulip_rs_ffi/examples/kvo_example.c.
package main

import (
	"fmt"

	"github.com/me60732/tulip_rs_go/examples/internal/demo"
	"github.com/me60732/tulip_rs_go/indicators"
	"github.com/me60732/tulip_rs_go/tulip"
)

func main() {
	var c demo.Check
	options := []float64{2.0, 5.0} // short_period, long_period

	info := indicators.Kvo.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Optional: %v, Type: %s\n",
		info.Inputs, info.Options, info.OptionalOutputs, info.Type)

	// Size the synthetic series from the indicator's own min_data so the
	// partial (n-50) slice is always big enough.
	n := 2*int(indicators.Kvo.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	high, low, close, volume := series[0], series[1], series[2], series[3]

	// ---- full compute -----------------------------------------------------
	fmt.Println("\n=== full calculation (all optional outputs) ===")
	res, st, err := indicators.Kvo.Indicator(high, low, close, volume, options, []bool{true, true})
	c.Err("full indicator", err)
	if err == nil {
		for i, name := range append([]string{"kvo"}, info.OptionalOutputs...) {
			if i < len(res.Rows) {
				fmt.Printf("  %-16s %d values\n", name, len(res.Rows[i]))
			}
		}
		fullKvo := append([]float64(nil), tulip.AsFloat64(res.Rows[0])...)
		res.Close()
		st.Close()

		// ---- partial + batch continuation --------------------------------
		fmt.Println("\n=== partial calculation + batch continuation ===")
		partial := n - 50
		res, st, err := indicators.Kvo.Indicator(high[:partial], low[:partial], close[:partial], volume[:partial], options, nil)
		c.Err("partial indicator", err)
		if err == nil {
			res.Close()
			br, err := st.Batch(high[partial:], low[partial:], close[partial:], volume[partial:], nil)
			c.Err("batch", err)
			if err == nil {
				tailLen := len(br.Rows[0])
				tailStart := len(fullKvo) - tailLen
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("partial+continued kvo[%d] equals full recompute", i),
						demo.SameTol(tulip.AsCDoubles([]float64{fullKvo[tailStart+i]}), br.Rows[0][i:i+1], 1e-6, 1e-9))
				}
				br.Close()
			}

			// ---- persistence ----------------------------------------------
			fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
			blob, err := st.Serialize(tulip.FormatBincode)
			c.Err("serialize", err)
			if err == nil {
				fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.KvoID)
			}
			rs, err := indicators.Kvo.DeserializeState(blob)
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
				tailLen := len(b1.Rows[0])
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("deserialized state continues kvo[%d] identically", i),
						demo.SameTol(b1.Rows[0][i:i+1], b2.Rows[0][i:i+1], 1e-6, 1e-9))
				}
			}
			if e1 == nil && e3 == nil {
				tailLen := len(b1.Rows[0])
				for i := 0; i < tailLen; i++ {
					c.Match(fmt.Sprintf("cloned state continues kvo[%d] identically", i),
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
	assets := [][indicators.KvoInputs][]float64{
		{high, low, close, volume},
		{demo.Scale(high, 1.2), demo.Scale(low, 1.2), demo.Scale(close, 1.2), demo.Scale(volume, 1.2)},
	}
	sim, err := indicators.Kvo.SimdByAssets(assets, options, nil)
	c.Err("simd by assets", err)
	if err == nil {
		for i := range sim.Results {
			r, s2, err := indicators.Kvo.Indicator(assets[i][0], assets[i][1], assets[i][2], assets[i][3], options, nil)
			c.Err("individual asset", err)
			if err == nil {
				tailLen := len(sim.Results[i][0])
				for j := 0; j < tailLen; j++ {
					c.Match(fmt.Sprintf("SIMD asset %d kvo[%d] equals individual", i+1, j),
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
	highTiled := make([]float64, tiledLen)
	lowTiled := make([]float64, tiledLen)
	closeTiled := make([]float64, tiledLen)
	volumeTiled := make([]float64, tiledLen)
	for i := 0; i < 3; i++ {
		copy(highTiled[i*n:(i+1)*n], high)
		copy(lowTiled[i*n:(i+1)*n], low)
		copy(closeTiled[i*n:(i+1)*n], close)
		copy(volumeTiled[i*n:(i+1)*n], volume)
	}
	sim2, err := indicators.Kvo.SimdByOptions(highTiled, lowTiled, closeTiled, volumeTiled,
		[][]float64{{1.0, 3.0}, {2.0, 5.0}, {3.0, 7.0}, {4.0, 10.0}}, nil)
	c.Err("simd by options", err)
	if err == nil {
		for i, o := range [][]float64{{1.0, 3.0}, {2.0, 5.0}, {3.0, 7.0}, {4.0, 10.0}} {
			r, s2, err := indicators.Kvo.Indicator(highTiled, lowTiled, closeTiled, volumeTiled, o, nil)
			c.Err("individual option set", err)
			if err == nil {
				tailLen := len(sim2.Results[i][0])
				for j := 0; j < tailLen; j++ {
					c.Match(fmt.Sprintf("SIMD option set %d kvo[%d] equals individual", i+1, j),
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
