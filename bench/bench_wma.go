package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "wma",
		Options: [][]float64{{14.0}, {20.0}, {25.0}, {30.0}},
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Wma.Indicator(s.Close, opts, nil)
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
			// cinar v2 WmaWithPeriod[T](period) computes weighted moving average
			wma := trend.NewWmaWithPeriod[float64](int(opts[0]))
			result := wma.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			// Drain unbuffered channel, touch first value to prevent elision
			for v := range result {
				_ = v
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][1][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [1][]float64{s.Close}
			}
			res, err := indicators.Wma.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			if len(res.Results) == 0 || len(res.Results[0]) == 0 || len(res.Results[0][0]) == 0 {
				return nil
			}
			_ = res.Results[0][0][0]
			return nil
		},
		SimdOptionsFn: func(s Stock, optionsList [][]float64) error {
			res, err := indicators.Wma.SimdByOptions(s.Close, optionsList, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			if len(res.Results) == 0 || len(res.Results[0]) == 0 || len(res.Results[0][0]) == 0 {
				return nil
			}
			_ = res.Results[0][0][0]
			return nil
		},
	})
}
