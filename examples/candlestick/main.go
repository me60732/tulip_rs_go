// Candlestick example: pattern detection over synthetic OHLC bars, streaming
// continuation, and state persistence — the Go mirror of candlestick_example.c.
// Candlestick is scalar-only (no SIMD) and its output is pattern ids, not
// f64 series.
package main

import (
	"fmt"

	"tulip_rs_go/examples/internal/demo"
	"tulip_rs_go/indicators"
	"tulip_rs_go/tulip"
)

// collect flattens per-bar pattern ids for equality checks.
func collect(barCount int, r *indicators.CandleResult) [][]uint32 {
	out := make([][]uint32, barCount)
	for b := range out {
		out[b] = r.Patterns(b)
	}
	return out
}

func eq(a, b [][]uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func main() {
	var c demo.Check
	options := []float64{5.0, 1.0, 1.0} // candle, trend, trend-signal periods

	info := indicators.Candlestick.Info()
	fmt.Printf("=== %s (%s) ===\n", info.Name, info.FullName)
	fmt.Printf("Inputs: %v, Options: %v, Pattern table: %d entries\n",
		info.Inputs, info.Options, indicators.Candlestick.NumPatterns())

	n := 2*int(indicators.Candlestick.MinData(options)) + 100
	series := demo.SeriesFor(info.Inputs, n)
	open, high, low, close := series[0], series[1], series[2], series[3]

	// ---- full detection run ------------------------------------------------
	fmt.Println("\n=== full run (no forecast filter) ===")
	res, st, err := indicators.Candlestick.Indicator(open, high, low, close, options, indicators.ForecastNone)
	c.Err("full indicator", err)
	full := [][]uint32{}
	if err == nil {
		full = collect(res.NumBars, res)
		detected := 0
		for b := 0; b < res.NumBars; b++ {
			if len(res.Patterns(b)) > 0 {
				detected++
				if detected <= 5 {
					fmt.Printf("  bar %2d: %v\n", b, res.Names(b))
				}
			}
		}
		fmt.Printf("  %d detections across %d bars (%d pattern ids total)\n",
			detected, res.NumBars, res.TotalPatterns)
		if pi, ok := indicators.Candlestick.PatternInfo(0); ok {
			fmt.Printf("  pattern 0: %s (%s, %d bars, %s)\n",
				pi.FullName, pi.Name, pi.Bars, pi.Forecast)
		}
		res.Close()
	}
	if st != nil {
		st.Close()
	}

	// ---- streaming: partial + batch over the tail --------------------------
	fmt.Println("\n=== partial + batch continuation ===")
	partial := n - 50
	res, st, err = indicators.Candlestick.Indicator(open[:partial], high[:partial], low[:partial], close[:partial], options, indicators.ForecastNone)
	c.Err("partial indicator", err)
	if err == nil {
		res.Close()
		br, err := st.Batch(open[partial:], high[partial:], low[partial:], close[partial:], indicators.ForecastNone)
		c.Err("batch", err)
		if err == nil {
			tail := full[len(full)-br.NumBars:]
			c.Match("partial+continued equals full recompute", eq(tail, collect(br.NumBars, br)))
			br.Close()
		}

		// ---- persistence: serialize / deserialize / clone ------------------
		fmt.Println("\n=== state persistence (serialize / deserialize / clone) ===")
		blob, err := st.Serialize(tulip.FormatBincode)
		c.Err("serialize", err)
		if err == nil {
			fmt.Printf("  bincode blob: %d bytes (indicator id 0x%08x)\n", len(blob), indicators.CandlestickID)
		}
		rs, err := indicators.Candlestick.DeserializeState(blob)
		c.Err("deserialize", err)
		cl, err := st.Clone()
		c.Err("clone", err)

		b1, e1 := st.Batch(open[partial:], high[partial:], low[partial:], close[partial:], indicators.ForecastNone)
		var b2, b3 *indicators.CandleResult
		var e2, e3 error
		if rs != nil {
			b2, e2 = rs.Batch(open[partial:], high[partial:], low[partial:], close[partial:], indicators.ForecastNone)
		}
		if cl != nil {
			b3, e3 = cl.Batch(open[partial:], high[partial:], low[partial:], close[partial:], indicators.ForecastNone)
		}
		c.Err("batch original", e1)
		c.Err("batch restored", e2)
		c.Err("batch clone", e3)
		if e1 == nil && e2 == nil {
			c.Match("deserialized state continues identically", eq(collect(b1.NumBars, b1), collect(b2.NumBars, b2)))
		}
		if e1 == nil && e3 == nil {
			c.Match("cloned state continues identically", eq(collect(b1.NumBars, b1), collect(b3.NumBars, b3)))
		}
		for _, r := range []*indicators.CandleResult{b1, b2, b3} {
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

	// ---- forecast filter demo ----------------------------------------------
	fmt.Println("\n=== forecast filter (bullish only) ===")
	fres, fst, err := indicators.Candlestick.Indicator(open, high, low, close, options, indicators.ForecastBullishReversal)
	c.Err("filtered indicator", err)
	if err == nil {
		fmt.Printf("  bullish-reversal-filtered detections: %d ids across %d bars\n",
			fres.TotalPatterns, fres.NumBars)
		fres.Close()
		fst.Close()
	}

	c.Done()
}
