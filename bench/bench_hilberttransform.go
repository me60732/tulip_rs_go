package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "hilberttransform",
		Options: [][]float64{{10.0, 20.0}, {15.0, 30.0}, {20.0, 40.0}, {25.0, 50.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Hilberttransform.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all two rows (in_phase, quadrature) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: nil, // hilberttransform has no cinar equivalent (no function with matching signature)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.HilberttransformInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.HilberttransformInputs][]float64{s.Close}
			}
			res, err := indicators.Hilberttransform.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Hilberttransform.SimdByOptions(s.Close, optsList, nil)
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
