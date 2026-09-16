package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volume"
)

func init() {
	register(BenchmarkDef{
		Name:    "chaikinmf",
		Options: [][]float64{{14.0}, {20.0}, {25.0}, {30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Chaikinmf.Indicator(s.High, s.Low, s.Close, s.Volume, opts, nil)
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
			cmf := volume.NewCmfWithPeriod[float64](int(opts[0]))
			result := cmf.ComputeWithContext(context.Background(), helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close), helper.SliceToChan(s.Volume))
			rows := helper.ChanToSlices(result)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 4-lane SIMD across assets (same series, same options)
			assets := make([][4][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [4][]float64{s.High, s.Low, s.Close, s.Volume}
			}
			res, err := indicators.Chaikinmf.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Chaikinmf.SimdByOptions(s.High, s.Low, s.Close, s.Volume, optionSets, nil)
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
