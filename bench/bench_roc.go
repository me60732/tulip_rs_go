package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "roc",
		Options: [][]float64{{25.0}, {30.0}, {50.0}, {100.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Roc.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
				return nil
			}
			_ = res.Rows[0][0]
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			// cinar v2 Roc computes percentage rate of change: (current - n periods ago) / n periods ago
			roc := trend.NewRocWithPeriod[float64](int(opts[0]))
			result := roc.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			// Drain unbuffered channel, touch first value to prevent elision
			for v := range result {
				_ = v
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.RocInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.RocInputs][]float64{s.Close}
			}
			res, err := indicators.Roc.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Roc.SimdByOptions(s.Close, optsList, nil)
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
