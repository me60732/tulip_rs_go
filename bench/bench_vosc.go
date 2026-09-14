package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "vosc",
		Options: [][]float64{{5.0, 20.0}, {9.0, 26.0}, {12.0, 26.0}, {3.0, 10.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Vosc.Indicator(s.Volume, opts, nil)
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
		// CinarFn: nil -- cinar PercentageVolumeOscillator requires a third
		// signalPeriod param (extra EMA work, arbitrary choice) that our
		// [fast, slow] option sets don't sweep; not a clean equivalent.
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][1][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [1][]float64{s.Volume}
			}
			res, err := indicators.Vosc.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Vosc.SimdByOptions(s.Volume, optionSets, nil)
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
