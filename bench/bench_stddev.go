package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volatility"
)

func init() {
	register(BenchmarkDef{
		Name:    "stddev",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {50.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Stddev.Indicator(s.Close, opts, nil)
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
			std := volatility.NewMovingStdWithPeriod[float64](int(opts[0]))
			result := std.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) == 0 {
				return nil // empty output is valid
			}
			_ = rows[0] // consume to prevent elision
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.StddevInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.StddevInputs][]float64{s.Close}
			}
			res, err := indicators.Stddev.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				if len(lanes) > 0 && len(lanes[0]) > 0 {
					_ = lanes[0][0]
				}
			}
			return nil
		},
		SimdOptionsFn: func(s Stock, optsList [][]float64) error {
			res, err := indicators.Stddev.SimdByOptions(s.Close, optsList, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				if len(lanes) > 0 && len(lanes[0]) > 0 {
					_ = lanes[0][0]
				}
			}
			return nil
		},
	})
}
