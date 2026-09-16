package bench

import (
	"context"

	"github.com/me60732/tulip_rs_go/indicators"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/momentum"
	"github.com/cinar/indicator/v2/trend"
)

func init() {
	register(BenchmarkDef{
		Name:    "ppo",
		Options: [][]float64{{12.0, 26.0}, {8.0, 18.0}, {5.0, 13.0}, {3.0, 9.0}}, // matches Python options_list (fast/slow pairs)
		TulipFn: func(s Stock, opts []float64) error {
			res, state, err := indicators.Ppo.Indicator(s.Close, opts, nil)
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
		CinarFn: func(s Stock, opts []float64) error {
			ppo := &momentum.Ppo[float64]{
				ShortEma:  trend.NewEmaWithPeriod[float64](int(opts[0])),
				LongEma:   trend.NewEmaWithPeriod[float64](int(opts[1])),
				SignalEma: trend.NewEmaWithPeriod[float64](9),
			}
			ppoChan, signalChan, histogramChan := ppo.ComputeWithContext(context.Background(), helper.SliceToChan(s.Close))
			// Drain all three output channels concurrently to avoid deadlock on unbuffered channels.
			rows := helper.ChanToSlices(ppoChan, signalChan, histogramChan)
			for _, row := range rows {
				if len(row) > 0 {
					_ = row[0]
				}
			}
			return nil
		},
		SimdAssetsFn: func(stocks []Stock, opts []float64) error {
			assets := make([][indicators.PpoInputs][]float64, len(stocks))
			for i, s := range stocks {
				assets[i] = [indicators.PpoInputs][]float64{s.Close}
			}
			res, err := indicators.Ppo.SimdByAssets(assets, opts, nil)
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
			res, err := indicators.Ppo.SimdByOptions(s.Close, optsList, nil)
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
