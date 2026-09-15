package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "kvo",
		Options: [][]float64{{34.0, 55.0}, {20.0, 40.0}, {10.0, 30.0}, {5.0, 20.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Kvo.Indicator(s.High, s.Low, s.Close, s.Volume, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume result to prevent dead-code elimination
			if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
				return nil // empty output is valid
			}
			_ = res.Rows[0][0] // touch first value

			return nil
		},
		CinarFn: nil, // v2 Kvo uses fixed internal periods (34/55) and signal period (13);
		// file sweeps {short_period, long_period} not controllable in v2
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.KvoInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.KvoInputs][]float64{s.High, s.Low, s.Close, s.Volume}
			}
			res, err := indicators.Kvo.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Kvo.SimdByOptions(s.High, s.Low, s.Close, s.Volume, optsList, nil)
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
