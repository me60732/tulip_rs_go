package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volatility"
)

func init() {
	register(BenchmarkDef{
		Name:    "bbands",
		Options: [][]float64{{5.0, 2.0}, {14.0, 2.0}, {20.0, 2.0}, {50.0, 2.0}},
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Bbands.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all three rows (lower, middle, upper) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			// cinar v2 BollingerBands is a stream-based generic type; period
			// and std-dev multiplier are set on the struct, matching our
			// {period, stdDev} option sweep one-to-one.
			bb := volatility.NewBollingerBandsWithPeriod[float64](int(opts[0]))
			bb.Multiplier = opts[1]
			upper, middle, lower := bb.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			// Drain all three output channels concurrently to avoid deadlock on
			// unbuffered channels.
			rows := helper.ChanToSlices(upper, middle, lower)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.BbandsInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.BbandsInputs][]float64{s.Close}
			}
			res, err := indicators.Bbands.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Bbands.SimdByOptions(s.Close, optsList, nil)
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
