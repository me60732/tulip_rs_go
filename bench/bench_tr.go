package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "tr",
		Options: [][]float64{{}}, // TR_OPTIONS=0, no options (mirrors Python [[]])
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Tr.Indicator(s.High, s.Low, s.Close, opts, nil)
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
		CinarFn: nil, // Cinar has no True Range function (only Atr which requires period parameter)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 3-lane SIMD across assets (same series, same options)
			assets := make([][indicators.TrInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.TrInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Tr.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // TR has no options (TR_OPTIONS=0), so simd_by_options is not meaningful
	})
}
