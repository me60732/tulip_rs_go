package bench

import (
	"tulip_rs_go/indicators"

	"github.com/cinar/indicator"
)

func init() {
	register(BenchmarkDef{
		Name:    "bop",
		Options: [][]float64{{}}, // BOP has no options
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Bop.Indicator(s.Open, s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			result := indicator.BalanceOfPower(s.Open, s.High, s.Low, s.Close)
			if len(result) == 0 {
				return nil
			}
			_ = result[0]
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.BopInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.BopInputs][]float64{s.Open, s.High, s.Low, s.Close}
			}
			res, err := indicators.Bop.SimdByAssets(assets, opts, nil)
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
		// SimdOptionsFn: nil -- bop has no options to sweep
		// (bop_simd_by_options is not exposed by the FFI).
	})
}
