package bench

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// Stock holds OHLCV data for one ticker loaded from the stocks database.
type Stock struct {
	Symbol string
	Open   []float64
	High   []float64
	Low    []float64
	Close  []float64
	Volume []float64
}

const (
	dataLimit = 6705 // same row-count as Python/Rust benchmarks
)

// stocks to benchmark: same four tickers used by all tulip-rs bench suites.
var stocksList = []struct {
	code     string
	exchange string
}{
	{"BHP", "ASX"},
	{"CBA", "ASX"},
	{"AAPL", "NYSE"},
	{"MSFT", "NYSE"},
}

// LoadStockData fetches OHLCV history for all benchmark tickers from the stocks DB.
// Returns chronologically ordered arrays (oldest → newest), same as Python/Rust.
func LoadStockData() ([]Stock, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://tulip:tulip@192.168.50.10:5433/stocks?sslmode=disable"
	}

	query := `
		SELECT e.open, e.high, e.low, e.close, e.volume
		FROM listing l
		INNER JOIN adj_eod e ON l.listing_id = e.listing_id
		WHERE l.code = $1
		  AND l.exchange_code = $2
		  AND e.volume > 0
		ORDER BY e.ts ASC
		LIMIT $3
	`

	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("loadStockData: sql.Open failed: %w", err)
	}
	defer conn.Close()

	results := make([]Stock, 0, len(stocksList))

	for _, s := range stocksList {
		rows, err := conn.Query(query, s.code, s.exchange, dataLimit)
		if err != nil {
			return nil, fmt.Errorf("loadStockData: query failed for %s/%s: %w", s.code, s.exchange, err)
		}

		var bars []struct {
			Open   float64
			High   float64
			Low    float64
			Close  float64
			Volume float64
		}
		for rows.Next() {
			var b struct {
				Open   float64
				High   float64
				Low    float64
				Close  float64
				Volume float64
			}
			if err := rows.Scan(&b.Open, &b.High, &b.Low, &b.Close, &b.Volume); err != nil {
				rows.Close()
				return nil, fmt.Errorf("loadStockData: scan failed for %s/%s: %w", s.code, s.exchange, err)
			}
			bars = append(bars, b)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("loadStockData: rows error for %s/%s: %w", s.code, s.exchange, err)
		}
		rows.Close()

		if len(bars) == 0 {
			fmt.Fprintf(os.Stderr, "[warn] no data for %s/%s — skipping\n", s.code, s.exchange)
			continue
		}

		sym := fmt.Sprintf("%s_%s", s.code, s.exchange)
		stk := Stock{Symbol: sym}
		stk.Open = make([]float64, len(bars))
		stk.High = make([]float64, len(bars))
		stk.Low = make([]float64, len(bars))
		stk.Close = make([]float64, len(bars))
		stk.Volume = make([]float64, len(bars))

		for i, b := range bars {
			stk.Open[i] = b.Open
			stk.High[i] = b.High
			stk.Low[i] = b.Low
			stk.Close[i] = b.Close
			stk.Volume[i] = b.Volume
		}

		fmt.Printf("  loaded %d bars  %s (from Postgres)\n", len(bars), sym)
		results = append(results, stk)
	}

	return results, nil
}
