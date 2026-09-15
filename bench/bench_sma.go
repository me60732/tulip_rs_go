package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "sma",
		Options: [][]float64{{50.0}, {100.0}, {200.0}, {300.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Sma.Indicator(s.Close, opts, nil)
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
			sma := trend.NewSmaWithPeriod[float64](int(opts[0]))
			result := sma.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) == 0 {
				return nil // empty output is valid
			}
			_ = rows[0] // consume to prevent elision
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.SmaInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.SmaInputs][]float64{s.Close}
			}
			res, err := indicators.Sma.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Sma.SimdByOptions(s.Close, optsList, nil)
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
