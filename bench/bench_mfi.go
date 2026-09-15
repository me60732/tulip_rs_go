package bench

import (
	"context"

	"tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/volume"
)

func init() {
	register(BenchmarkDef{
		Name:    "mfi",
		Options: [][]float64{{14.0}, {20.0}, {25.0}, {30.0}}, // matches Python options_list
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Mfi.Indicator(s.High, s.Low, s.Close, s.Volume, opts, nil)
			if err != nil {
				return err
			}
			defer res.Close()
			defer state.Close()

			// Consume the single row (mfi).
			if len(res.Rows) > 0 && len(res.Rows[0]) > 0 {
				_ = res.Rows[0][0]
			}
			return nil
		},
		CinarFn: func(s Stock, opts []float64) error {
			mfi := volume.NewMfiWithPeriod[float64](int(opts[0]))
			result := mfi.ComputeWithContext(context.Background(), helper.SliceToChan(s.High), helper.SliceToChan(s.Low), helper.SliceToChan(s.Close), helper.SliceToChan(s.Volume))
			rows := helper.ChanToSlice(result)
			if len(rows) > 0 {
				_ = rows[0]
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.MfiInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.MfiInputs][]float64{s.High, s.Low, s.Close, s.Volume}
			}
			res, err := indicators.Mfi.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Mfi.SimdByOptions(s.High, s.Low, s.Close, s.Volume, optsList, nil)
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
