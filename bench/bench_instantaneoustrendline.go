package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "instantaneoustrendline",
		Options: [][]float64{{}}, // matches Python options_list=[[]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Instantaneoustrendline.Indicator(s.Close, opts, nil)
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
		CinarFn: nil, // instantaneoustrendline has no cinar equivalent (no function with matching signature)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.InstantaneoustrendlineInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.InstantaneoustrendlineInputs][]float64{s.Close}
			}
			res, err := indicators.Instantaneoustrendline.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // instantaneoustrendline has no options, so simd_by_options not applicable
	})
}
