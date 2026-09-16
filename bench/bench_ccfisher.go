package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "ccfisher",
		Options: [][]float64{{0.0}, {0.05}, {0.07}, {0.10}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Ccfisher.Indicator(s.Close, opts, nil)
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
		CinarFn: nil, // Cinar has no Cyber Cycle Fisher equivalent; checked via grep on cinar/indicator v1.3.0
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 1-lane SIMD across assets (same series, same options)
			assets := make([][1][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [1][]float64{s.Close}
			}
			res, err := indicators.Ccfisher.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Ccfisher.SimdByOptions(s.Close, optionSets, nil)
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
