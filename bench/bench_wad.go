package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "wad",
		Options: [][]float64{{}}, // option-less (none)
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Wad.Indicator(s.High, s.Low, s.Close, opts, nil)
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
		CinarFn: nil, // no AccumulationDistribution with period parameter in cinar
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][3][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [3][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Wad.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // wad has no options, so simd_by_options not applicable
	})
}
