package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "avgprice",
		Options: [][]float64{{}}, // AVGPRICE has no options (matches Python options_list=[[]])
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Avgprice.Indicator(s.Open, s.High, s.Low, s.Close, opts, nil)
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
		CinarFn: nil, // no cinar equivalent for avgprice in v1.3.0
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 4-lane SIMD across assets (same series, same options)
			assets := make([][4][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [4][]float64{s.Open, s.High, s.Low, s.Close}
			}
			res, err := indicators.Avgprice.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			// Consume first lane's result
			if len(res.Results) == 0 || len(res.Results[0]) == 0 || len(res.Results[0][0]) == 0 {
				return nil
			}
			_ = res.Results[0][0][0] // first lane, first output, first value
			return nil
		},
		SimdOptionsFn: nil, // avgprice has no options, so simd_by_options not applicable
	})
}
