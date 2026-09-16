package bench

import (
	"context"
	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volatility"
)

func init() {
	register(BenchmarkDef{
		Name:    "chandelierexit",
		Options: [][]float64{{14.0, 3.0}, {20.0, 3.0}, {22.0, 3.0}, {22.0, 2.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Chandelierexit.Indicator(s.High, s.Low, s.Close, opts, nil)
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
			ce := volatility.NewChandelierExit[float64]()
			ce.Period = int(opts[0])
			ce.Multiplier = opts[1]
			long, short := ce.ComputeWithContext(context.Background(),
				helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlices(long, short) // drains two output chans together
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			// 3-lane SIMD across assets (same series, same options)
			assets := make([][3][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [3][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Chandelierexit.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Chandelierexit.SimdByOptions(s.High, s.Low, s.Close, optionSets, nil)
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
