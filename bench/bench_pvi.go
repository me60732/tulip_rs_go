package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "pvi",
		Options: [][]float64{{}}, // matches Python options_list=[[]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Pvi.Indicator(s.Close, s.Volume, opts, nil)
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
		CinarFn: nil, // cinar has no PositiveVolumeIndex; only private percentDiff helper
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.PviInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.PviInputs][]float64{s.Close, s.Volume}
			}
			res, err := indicators.Pvi.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // pvi has no options, so simd_by_options not applicable
	})
}
