package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "linreg",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {50.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Linreg.Indicator(s.Close, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume result to prevent dead-code elimination
			if len(res.Rows) == 0 || len(res.Rows[0]) == 0 {
				return nil // empty output is valid for some edge cases
			}
			_ = res.Rows[0][0] // touch first value

			return nil
		},
		// CinarFn: nil -- cinar LinearRegressionUsingLeastSquare(x, y []float64)
		// returns a single array but it's the regression line values (not just
		// the END value like tulip linreg which returns the last point of the
		// linear regression fit); different output definition.
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.LinregInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.LinregInputs][]float64{s.Close}
			}
			res, err := indicators.Linreg.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Linreg.SimdByOptions(s.Close, optsList, nil)
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
