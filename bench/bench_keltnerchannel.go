package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "keltnerchannel",
		Options: [][]float64{{20.0, 2.0}, {20.0, 1.5}, {14.0, 2.0}, {10.0, 1.5}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Keltnerchannel.Indicator(s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all three rows (lower, middle, upper) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: nil, // cinar KeltnerChannel hardcodes 2*ATR multiplier - file sweeps period+multiplier pairs like {20.0, 2.0}
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.KeltnerchannelInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.KeltnerchannelInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Keltnerchannel.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Keltnerchannel.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
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
