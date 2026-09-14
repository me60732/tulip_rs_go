package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "kama",
		Options: [][]float64{{5.0}, {10.0}, {14.0}, {20.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Kama.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume result to prevent dead-code elimination
			if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
				return nil // empty output is valid for some edge cases
			}
			_ = res.Rows[0][0] // touch first value

			return nil
		},
		// CinarFn: nil -- cinar has no KAMA implementation; ta.momentum.KAMAIndicator
		// exists but we only wire cinar if there's a direct equivalent with matching
		// parameterization (period-only, same output definition).
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.KamaInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.KamaInputs][]float64{s.Close}
			}
			res, err := indicators.Kama.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Kama.SimdByOptions(s.Close, optsList, nil)
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
