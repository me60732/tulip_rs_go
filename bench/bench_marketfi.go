package bench

import "github.com/me60732/tulip_rs_go/indicators"

func init() {
	register(BenchmarkDef{
		Name:    "marketfi",
		Options: [][]float64{{}}, // matches Python options_list [[]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Marketfi.Indicator(s.High, s.Low, s.Volume, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the single row (marketfi).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MarketfiInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MarketfiInputs][]float64{s.High, s.Low, s.Volume}
			}
			res, err := indicators.Marketfi.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // zero-option indicators have no option sets to vary
	})
}
