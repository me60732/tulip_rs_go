package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
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
			bop := trend.NewBop[float64]()
			result := bop.ComputeWithContext(context.Background(), helper.SliceToChan(s.Open), helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close))
			rows := helper.ChanToSlice(result)
			if len(rows) > 0 {
				_ = rows[0]
			}
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
