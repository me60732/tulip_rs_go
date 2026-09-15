package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "ichimoku",
		Options: [][]float64{{9.0, 26.0}, {5.0, 10.0}, {7.0, 14.0}, {9.0, 52.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Ichimoku.Indicator(s.High, s.Low, s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume all four rows (conversion, base, leading_span_a, leading_span_b) so the compiler
			// cannot elide the computation.
			for _, row := range res.Rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		CinarFn: nil, // cinar IchimokuCloud hardcodes periods (9/26/52) - file sweeps custom pairs like 9/26, 5/10; also output mismatch (cinar 5 rows vs tulip 4)
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.IchimokuInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.IchimokuInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Ichimoku.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Ichimoku.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
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
