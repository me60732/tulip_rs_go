package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "psar",
		Options: [][]float64{{0.02, 0.2}, {0.01, 0.2}, {0.02, 0.1}, {0.04, 0.4}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Psar.Indicator(s.High, s.Low, opts, nil)
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
		CinarFn: nil, // cinar ParabolicSar(high, low, closing []float64) -> ([]float64, []Trend)
		// accepts only price series; no acceleration_factor or max_step options.
		// Our option sets [0.02, 0.2] etc. are incompatible.
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.PsarInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.PsarInputs][]float64{s.High, s.Low}
			}
			res, err := indicators.Psar.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Psar.SimdByOptions(s.High, s.Low, optsList, nil)
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
