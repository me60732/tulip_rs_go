package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "di",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Di.Indicator(s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume result to prevent dead-code elimination
			if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
				return nil // empty output is valid for some edge cases
			}
			_ = res.Rows[0][0] // touch first value (plus_di)

			return nil
		},
		CinarFn: nil, // cinar has no PlusDI/MinusDI functions; Python benchmark used pandas_ta which computes both +DI and -DI
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 3-lane SIMD across assets (same series, same options)
			assets := make([][3][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [3][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Di.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			// Consume first lane's result
			if len(res.Results) == 0 || len(res.Results[0]) == 0 || len(res.Results[0][0]) == 0 {
				return nil
			}
			_ = res.Results[0][0][0] // first lane, first output (plus_di), first value
			return nil
		},
		SimdOptionsFn: func(s Stock, optionSets [][]float64) error {
			res, err := indicators.Di.SimdByOptions(s.High, s.Low, s.Close, optionSets, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			// Consume first lane's result
			if len(res.Results) == 0 || len(res.Results[0]) == 0 || len(res.Results[0][0]) == 0 {
				return nil
			}
			_ = res.Results[0][0][0] // first lane, first output (plus_di), first value
			return nil
		},
	})
}
