package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volatility"
)

func init() {
	register(BenchmarkDef{
		Name:    "supertrend",
		Options: [][]float64{{7.0, 3.0}, {5.0, 2.0}, {10.0, 2.5}, {14.0, 2.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Supertrend.Indicator(s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume result to prevent dead-code elimination
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			st := volatility.NewSuperTrendWithPeriod[float64](int(opts[0]), opts[1])
			result := st.ComputeWithContext(context.Background(),
				helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlices(result)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.SupertrendInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.SupertrendInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Supertrend.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Supertrend.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
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
