package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "dm",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {30.0}}, // matches Python options_list=[[5.0], [14.0], [20.0], [30.0]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Dm.Indicator(s.High, s.Low, opts, nil)
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
		CinarFn: nil, // Cinar has no DirectionalMovement function (verified: no fn exists in /home/mark/go/pkg/mod/github.com/cinar/indicator@v1.3.0/*.go)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 2-lane SIMD across assets (high, low)
			assets := make([][2][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [2][]float64{s.High, s.Low}
			}
			res, err := indicators.Dm.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: func(s Stock, optionSets [][]float64) error {
			res, err := indicators.Dm.SimdByOptions(s.High, s.Low, optionSets, nil)
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
	})
}
