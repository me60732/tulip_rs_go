package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "wilders",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {50.0}},
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Wilders.Indicator(s.Close, opts, nil)
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
			wilders := trend.NewRmaWithPeriod[float64](int(opts[0]))
			result := wilders.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) > 0 {
				_ = rows[0]
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][1][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [1][]float64{s.Close}
			}
			res, err := indicators.Wilders.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Wilders.SimdByOptions(s.Close, optionsList, nil)
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
