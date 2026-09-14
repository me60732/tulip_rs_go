package bench

import "tulip_rs_go/indicators"

func init() {
	register(BenchmarkDef{
		Name:    "stochrsi",
		Options: [][]float64{{14.0}, {20.0}, {25.0}, {30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Stochrsi.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume both rows (stochrsi, signal) to prevent dead-code elimination
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: nil, // cinar does not have StochRSI
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.StochrsiInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.StochrsiInputs][]float64{s.Close}
			}
			res, err := indicators.Stochrsi.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Stochrsi.SimdByOptions(s.Close, optsList, nil)
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
