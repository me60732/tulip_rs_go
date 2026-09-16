package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
)

func init() {
	register(BenchmarkDef{
		Name:    "qstick",
		Options: [][]float64{{5.0}, {8.0}, {14.0}, {20.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Qstick.Indicator(s.Open, s.Close, opts, nil)
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
			qs := momentum.NewQstick[float64]()
			qs.Sma.Period = int(opts[0])
			result := qs.ComputeWithContext(context.Background(), helper.SliceToChan(s.Open), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) == 0 {
				return nil
			}
			_ = rows[0]
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.QstickInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.QstickInputs][]float64{s.Open, s.Close}
			}
			res, err := indicators.Qstick.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Qstick.SimdByOptions(s.Open, s.Close, optsList, nil)
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
