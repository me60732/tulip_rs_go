package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
)

func init() {
	register(BenchmarkDef{
		Name:    "willr",
		Options: [][]float64{{25.0}, {35.0}, {50.0}, {100.0}},
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Willr.Indicator(s.High, s.Low, s.Close, opts, nil)
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
			// v2 WilliamsR period is configured through its exported Max/Min
			// MovingMax/MovingMin instances (each has a Period field).
			wr := momentum.NewWilliamsR[float64]()
			period := int(opts[0])
			wr.Max.Period = period
			wr.Min.Period = period
			row := helper.ChanToSlice(wr.ComputeWithContext(context.Background(),
				helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close)))
			if len(row) > 0 {
				_ = row[0]
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][3][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [3][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Willr.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Willr.SimdByOptions(s.High, s.Low, s.Close, optionsList, nil)
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
