package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
)

func init() {
	register(BenchmarkDef{
		Name:    "fisher",
		Options: [][]float64{{5.0}, {9.0}, {14.0}, {20.0}}, // matches Python
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Fisher.Indicator(s.High, s.Low, opts, nil)
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
		CinarFn: func(s Stock, opts []float64) error {
			// cinar v2 EhlersFisher uses (High+Low)/2 with recursive smoothing,
			// matching tulip Fisher semantics for period-swept options.
			fisher := momentum.NewEhlersFisherWithPeriod[float64](int(opts[0]))
			result := fisher.ComputeWithContext(context.Background(), helper.SliceToChan(s.High), helper.SliceToChan(s.Low))
			// Drain unbuffered channel, touch first value to prevent elision
			for v := range result {
				_ = v
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 2-lane SIMD across assets (same series, same options)
			assets := make([][2][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [2][]float64{s.High, s.Low}
			}
			res, err := indicators.Fisher.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Fisher.SimdByOptions(s.High, s.Low, optionSets, nil)
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
