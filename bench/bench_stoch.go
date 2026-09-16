package bench

import (
	"context"
	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "stoch",
		Options: [][]float64{{28.0, 16.0, 12.0}, {35.0, 21.0, 14.0}, {50.0, 30.0, 21.0}, {100.0, 50.0, 30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Stoch.Indicator(s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume both rows (k, d) to prevent dead-code elimination
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: func(stock Stock, opts []float64) error {
			// The file sweeps {period, kPeriod, dPeriod} which matches SlowStochastic.
			s := trend.NewSlowStochasticWithPeriod[float64](int(opts[0]), int(opts[1]), int(opts[2]))
			slowK, slowD := s.ComputeWithContext(context.Background(), helper.SliceToChan(stock.Close))
			rows := helper.ChanToSlices(slowK, slowD)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.StochInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.StochInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Stoch.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			for _, lanes := range res.Results {
				for _, row := range lanes {
					if len(row) > 0 {
						_ = row[0]
					}
				}
			}
			return nil
		},
		SimdOptionsFn: func(s Stock, optsList [][]float64) error {
			res, err := indicators.Stoch.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
			if err != nil {
				return err
			}
			defer res.Close()

			for _, lanes := range res.Results {
				for _, row := range lanes {
					if len(row) > 0 {
						_ = row[0]
					}
				}
			}
			return nil
		},
	})
}
