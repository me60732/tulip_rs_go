package bench

import (
	"tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "vwap",
		Options: [][]float64{{}}, // matches Python options_list=[[]]
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Vwap.Indicator(s.High, s.Low, s.Close, s.Volume, opts, nil)
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
		CinarFn: nil, // v2 VWAP uses closing price only; tulip uses high/low/close/volume - different calculation
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][4][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [4][]float64{s.High, s.Low, s.Close, s.Volume}
			}
			res, err := indicators.Vwap.SimdByAssets(assets, opts, nil)
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
		SimdOptionsFn: nil, // vwap has no options, so simd_by_options not applicable
	})
}
