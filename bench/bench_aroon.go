package bench

import (
	"context"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"

	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "aroon",
		Options: [][]float64{{25.0}, {35.0}, {50.0}, {100.0}}, // matches Python
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Aroon.Indicator(s.High, s.Low, opts, nil)
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
			a := trend.NewAroon[float64]()
			a.Period = int(opts[0])
			highChan := helper.SliceToChan(s.High)
			lowChan := helper.SliceToChan(s.Low)
			out1, out2 := a.ComputeWithContext(context.Background(), highChan, lowChan)
			helper.ChanToSlices(out1, out2)
			_ = s.Close[0] // touch first value to prevent dead-code elimination
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][2][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [2][]float64{s.High, s.Low}
			}
			res, err := indicators.Aroon.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: func(s Stock, optionSets [][]float64) error {
			res, err := indicators.Aroon.SimdByOptions(s.High, s.Low, optionSets, nil)
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
