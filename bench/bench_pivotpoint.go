package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "pivotpoint",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {30.0}}, // matches Python options_list for similar indicators
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Pivotpoint.Indicator(s.High, s.Low, s.Close, opts, nil)
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
		CinarFn: nil, // v2 PivotPoint has no period parameter (only Method field for calculation style); cannot match Tulip's swept period options
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.PivotpointInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.PivotpointInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Pivotpoint.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Pivotpoint.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
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
