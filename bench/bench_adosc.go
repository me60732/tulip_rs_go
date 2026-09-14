package bench

import (
	"tulip_rs_go/indicators"

	"github.com/cinar/indicator"
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
			// Cinar has ChaikinOscillator(fastPeriod, slowPeriod int, low, high, closing, volume []float64) ([]float64, []float64)
			// We need to pass options as periods: opts[0]=fast, opts[1]=slow
			if len(opts) != 2 {
				return nil // skip if wrong option count
			}
			fastPeriod := int(opts[0])
			slowPeriod := int(opts[1])
			result1, result2 := indicator.ChaikinOscillator(fastPeriod, slowPeriod, s.Low, s.High, s.Close, s.Volume)
			if len(result1) == 0 && len(result2) == 0 {
				return nil
			}
			// Consume both outputs (ChaikinOscillator returns two arrays: ADOSC and signal)
			if len(result1) > 0 {
				_ = result1[0]
			}
			if len(result2) > 0 {
				_ = result2[0]
			}
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
