package bench

import (
	"tulip_rs_go/indicators"

	"github.com/cinar/indicator"
)

func init() {
	register(BenchmarkDef{
		Name:    "max",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {50.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Max.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the single row (max).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			// cinar Max(period int, values) is a genuine rolling maximum with
			// the same swept period parameter.
			result := indicator.Max(int(opts[0]), s.Close)
			if len(result) == 0 {
				return nil
			}
			_ = result[0]
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MaxInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MaxInputs][]float64{s.Close}
			}
			res, err := indicators.Max.SimdByAssets(assets, opts)
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
			res, err := indicators.Max.SimdByOptions(s.Close, optsList)
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
