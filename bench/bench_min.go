package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "min",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {50.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Min.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the single row (min).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			min := trend.NewMovingMinWithPeriod[float64](int(opts[0]))
			result := min.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) > 0 {
				_ = rows[0]
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MinInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MinInputs][]float64{s.Close}
			}
			res, err := indicators.Min.SimdByAssets(assets, opts)
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
			res, err := indicators.Min.SimdByOptions(s.Close, optsList)
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
