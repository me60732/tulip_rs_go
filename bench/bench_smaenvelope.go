package bench

import "tulip_rs_go/indicators"

func init() {
	register(BenchmarkDef{
		Name:    "smaenvelope",
		Options: [][]float64{{20.0, 2.5}, {20.0, 5.0}, {50.0, 2.5}, {50.0, 5.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Smaenvelope.Indicator(s.Close, opts, nil)
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
		CinarFn: nil, // no cinar equivalent (smaenvelope not in cinar/indicator)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.SmaenvelopeInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.SmaenvelopeInputs][]float64{s.Close}
			}
			res, err := indicators.Smaenvelope.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Smaenvelope.SimdByOptions(s.Close, optsList, nil)
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
