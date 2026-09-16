package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
)

func init() {
	register(BenchmarkDef{
		Name:    "elderray",
		Options: [][]float64{{5.0}, {13.0}, {20.0}, {30.0}}, // matches Python options_list=[[5.0], [13.0], [20.0], [30.0]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Elderray.Indicator(s.High, s.Low, s.Close, opts, nil)
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
			er := momentum.NewElderRayWithPeriod[float64](int(opts[0]))
			bullPower, bearPower := er.ComputeWithContext(context.Background(),
				helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlices(bullPower, bearPower)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 3-lane SIMD across assets (high, low, close)
			assets := make([][3][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [3][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Elderray.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Elderray.SimdByOptions(s.High, s.Low, s.Close, optionSets, nil)
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
