package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "macd",
		Options: [][]float64{{5.0, 13.0, 8.0}, {19.0, 39.0, 9.0}, {10.0, 30.0, 10.0}, {6.0, 20.0, 9.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Macd.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all three rows (macd, short_ema, long_ema) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		// CinarFn: nil -- cinar Macd(closing []float64) returns only 2 rows
		// (macd, signal) with hardcoded defaults (12/26), not the full 3-row
		// output (macd, short_ema, long_ema) and no parameterization for
		// fastperiod/slowperiod(signalperiod). Not a genuine equivalent.
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MacdInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MacdInputs][]float64{s.Close}
			}
			res, err := indicators.Macd.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				for _, row := range lanes {
					if len(row) > 0 {
						_ = row[0]
					}
				}
			}
			return nil
		},
		SimdOptionsFn: func(s Stock, optsList [][]float64) error {
			res, err := indicators.Macd.SimdByOptions(s.Close, optsList, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				for _, row := range lanes {
					if len(row) > 0 {
						_ = row[0]
					}
				}
			}
			return nil
		},
	})
}
