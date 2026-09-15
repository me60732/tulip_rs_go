package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "medprice",
		Options: [][]float64{{}}, // matches Python options_list=[[]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Medprice.Indicator(s.High, s.Low, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the single row (medprice).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		// CinarFn: nil -- cinar has no median price; TypicalPrice is (high+low+close)/3
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MedpriceInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MedpriceInputs][]float64{s.High, s.Low}
			}
			res, err := indicators.Medprice.SimdByAssets(assets, opts)
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
		SimdOptionsFn: nil, // Medprice has no SimdByOptions
	})
}
