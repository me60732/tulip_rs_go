package bench

import (
	"github.com/me60732/tulip_rs_go/indicators"
)

func init() {
	register(BenchmarkDef{
		Name:    "natr",
		Options: [][]float64{{5.0}, {14.0}, {20.0}, {30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Natr.Indicator(s.High, s.Low, s.Close, opts, nil)
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
		CinarFn: nil, // no NATR fn in cinar; Atr returns (tr,atr) not normalized → don't post-process
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.NatrInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.NatrInputs][]float64{s.High, s.Low, s.Close}
			}
			res, err := indicators.Natr.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Natr.SimdByOptions(s.High, s.Low, s.Close, optsList, nil)
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
