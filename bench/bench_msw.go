package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "msw",
		Options: [][]float64{{5.0}, {8.0}, {14.0}, {20.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Msw.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the two rows (slope, lead).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			if len(res.Rows) > 1 && len(res.Rows[1]) > 0 {
				_ = res.Rows[1][0]
			}
			return nil
		},
		// CinarFn: nil -- cinar v2 has no Mesa Sine Wave function
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MswInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MswInputs][]float64{s.Close}
			}
			res, err := indicators.Msw.SimdByAssets(assets, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				if len(lanes) > 0 && len(lanes[0]) > 0 {
					_ = lanes[0][0]
				}
				if len(lanes) > 1 && len(lanes[1]) > 0 {
					_ = lanes[1][0]
				}
			}
			return nil
		},
		SimdOptionsFn: func(s Stock, optsList [][]float64) error {
			res, err := indicators.Msw.SimdByOptions(s.Close, optsList, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			for _, lanes := range res.Results {
				if len(lanes) > 0 && len(lanes[0]) > 0 {
					_ = lanes[0][0]
				}
				if len(lanes) > 1 && len(lanes[1]) > 0 {
					_ = lanes[1][0]
				}
			}
			return nil
		},
	})
}
