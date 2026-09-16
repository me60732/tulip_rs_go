package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "aroonosc",
		Options: [][]float64{{25.0}, {35.0}, {50.0}, {100.0}}, // matches Python
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Aroonosc.Indicator(s.High, s.Low, opts, nil)
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
		CinarFn: nil, // Cinar does not have AroonOscillator (only Aroon which returns up/down lines)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][2][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [2][]float64{s.High, s.Low}
			}
			res, err := indicators.Aroonosc.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Aroonosc.SimdByOptions(s.High, s.Low, optionSets, nil)
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
