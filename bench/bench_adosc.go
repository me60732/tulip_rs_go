package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
)

func init() {
	register(BenchmarkDef{
		Name:    "adosc",
		Options: [][]float64{{2.0, 5.0}, {3.0, 10.0}, {5.0, 20.0}, {10.0, 30.0}}, // matches Python
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Adosc.Indicator(s.High, s.Low, s.Close, s.Volume, opts, nil)
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
			if len(opts) != 2 {
				return nil // skip if wrong option count
			}
			co := momentum.NewChaikinOscillator[float64]()
			co.ShortEma.Period = int(opts[0])
			co.LongEma.Period = int(opts[1])
			osc, ad := co.ComputeWithContext(context.Background(), helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close), helper.SliceToChan(s.Volume))
			// Both output channels are unbuffered; drain them together.
			rows := helper.ChanToSlices(osc, ad)
			if len(rows) == 0 {
				return nil
			}
			_ = rows[0] // consume to prevent elision
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][4][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [4][]float64{s.High, s.Low, s.Close, s.Volume}
			}
			res, err := indicators.Adosc.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Adosc.SimdByOptions(s.High, s.Low, s.Close, s.Volume, optionSets, nil)
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
