package bench

import "tulip_rs_go/indicators"

func init() {
	register(BenchmarkDef{
		Name:    "bbands",
		Options: [][]float64{{5.0, 2.0}, {14.0, 2.0}, {20.0, 2.0}, {50.0, 2.0}},
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Bbands.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all three rows (lower, middle, upper) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		// CinarFn: nil -- cinar BollingerBands(closing) has no period/std-dev
		// parameters (fixed window); not a genuine equivalent for our
		// variable-period option sets (same rule that rejected Aroon).
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.BbandsInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.BbandsInputs][]float64{s.Close}
			}
			res, err := indicators.Bbands.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Bbands.SimdByOptions(s.Close, optsList, nil)
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
