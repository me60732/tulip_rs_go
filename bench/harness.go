package bench

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// RunFn performs one full indicator call and cleanup cycle.
// fn(s, opts) → consume results → state.Close()/result.Close()
type RunFn func(s Stock, opts []float64) error

// SimdAssetsFn computes SIMD across N assets (same series, same options).
type SimdAssetsFn func(stocks []Stock, opts []float64) error

// SimdOptionsFn computes SIMD across N option sets for one stock.
type SimdOptionsFn func(s Stock, optionsList [][]float64) error

// BenchmarkDef describes one indicator benchmark.
type BenchmarkDef struct {
	Name          string
	Options       [][]float64 // per-indicator option sets (mirrors Python/Rust)
	TulipFn       RunFn       // tulip_rs_go implementation
	CinarFn       RunFn       // cinar reference (nil => skip)
	QuantgoFn     RunFn       // quantgo reference (nil => skip)
	SimdAssetsFn  SimdAssetsFn
	SimdOptionsFn SimdOptionsFn
}

// ---------------------------------------------------------------------
// Global registration and config
// ---------------------------------------------------------------------

var benchDefs = make([]BenchmarkDef, 0)

func register(def BenchmarkDef) {
	benchDefs = append(benchDefs, def)
}

// Config pulled from environment (mirrors Python/Rust naming).
var (
	BENCH_NUMBER int
	BENCH_REPEAT int
	BENCH_WARMUP int
	LOG_TO_DB    bool
)

// InitConfig captures the benchmark knobs from the environment. MUST be
// called after LoadDotEnv() — a package init() would snapshot the raw
// process env before .env is applied, silently ignoring BENCHMARK_LOG_TO_DB
// and friends. RunAll calls this itself, so callers only need to invoke it
// directly if they use the timing vars before RunAll.
func InitConfig() {
	// Defaults mirror the Python harness
	BENCH_NUMBER = envInt("BENCH_NUMBER", 10)
	BENCH_REPEAT = envInt("BENCH_REPEAT", 30)
	BENCH_WARMUP = envInt("BENCH_WARMUP", 10)
	LOG_TO_DB = envStr("BENCHMARK_LOG_TO_DB", "0") == "1"
}

func envInt(name string, def int) int {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	i, _ := strconv.Atoi(v)
	return i
}

func envStr(name, def string) string {
	v := os.Getenv(name)
	if v == "" {
		return def
	}
	return v
}

// ---------------------------------------------------------------------
// Timing helper (mirrors C/Python time_fn methodology)
// ---------------------------------------------------------------------

type TimingResult struct {
	MeanNS      int
	StdDevNS    int
	MinNS       int
	MaxNS       int
	SampleCount int
}

func timeFn(fn func(), number, repeat, warmup int) TimingResult {
	// Warm-up: hot the CPU caches and allocator free-lists (mirrors Criterion)
	for i := 0; i < warmup; i++ {
		fn()
	}

	timer := time.NewTimer(0)
	defer timer.Stop()

	nsSamples := make([]float64, repeat)

	for r := 0; r < repeat; r++ {
		start := time.Now()
		for n := 0; n < number; n++ {
			fn()
		}
		elapsed := time.Since(start)
		nsPerCall := float64(elapsed.Nanoseconds()) / float64(number)
		nsSamples[r] = nsPerCall
	}

	mean := mathMean(nsSamples)
	stddev := mathStdev(nsSamples)
	minVal := mathMin(nsSamples)
	maxVal := mathMax(nsSamples)

	return TimingResult{
		MeanNS:      int(mean),
		StdDevNS:    int(stddev),
		MinNS:       int(minVal),
		MaxNS:       int(maxVal),
		SampleCount: repeat,
	}
}

// ---------------------------------------------------------------------
// Simple stats helpers
// ---------------------------------------------------------------------

func mathMean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func mathStdev(vals []float64) float64 {
	if len(vals) < 2 {
		return 0
	}
	mean := mathMean(vals)
	var sumSqDiff float64
	for _, v := range vals {
		diff := v - mean
		sumSqDiff += diff * diff
	}
	return math.Sqrt(sumSqDiff / float64(len(vals)-1))
}

func mathMin(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	minVal := vals[0]
	for _, v := range vals[1:] {
		if v < minVal {
			minVal = v
		}
	}
	return minVal
}

func mathMax(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	maxVal := vals[0]
	for _, v := range vals[1:] {
		if v > maxVal {
			maxVal = v
		}
	}
	return maxVal
}

// ---------------------------------------------------------------------
// DB Logger (mirrors Python BenchmarkLogger with reconnect-retry)
// ---------------------------------------------------------------------

type BenchmarkLogger struct {
	conn     *sql.DB
	runID    int64
	indCache map[string]int64
	benchURL string
}

func newBenchmarkLogger() (*BenchmarkLogger, error) {
	dbURL := os.Getenv("BENCHMARK_DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://tulip:tulip@192.168.50.10:5433/indicator_benchmark?sslmode=disable"
	}

	conn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, fmt.Errorf("newBenchmarkLogger: sql.Open failed: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("newBenchmarkLogger: Ping failed: %w", err)
	}

	logger := &BenchmarkLogger{
		conn:     conn,
		indCache: make(map[string]int64),
		benchURL: dbURL,
	}
	if err := logger.loadIndicators(); err != nil {
		logger.Close()
		return nil, err
	}
	return logger, nil
}

func (l *BenchmarkLogger) reconnect() error {
	l.conn.Close()
	conn, err := sql.Open("postgres", l.benchURL)
	if err != nil {
		return fmt.Errorf("reconnect: sql.Open failed: %w", err)
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return fmt.Errorf("reconnect: Ping failed: %w", err)
	}
	l.conn = conn
	return nil
}

func (l *BenchmarkLogger) withRetry(label string, fn func() error) error {
	if err := fn(); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] %s: connection error (%v); reconnecting and retrying once\n", label, err)
		// sql.DB has no Rollback - just close/reopen for retry
		if reterr := l.reconnect(); reterr != nil {
			fmt.Fprintf(os.Stderr, "[warn] %s: reconnect failed (%v); skipping this write\n", label, reterr)
			return reterr
		}
		return fn()
	}
	return nil
}

func (l *BenchmarkLogger) loadIndicators() error {
	var rows *sql.Rows
	var err error

	if err := l.withRetry("loadIndicators", func() error {
		rows, err = l.conn.Query("SELECT id, name FROM indicators")
		return err
	}); err != nil {
		return fmt.Errorf("loadIndicators: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return fmt.Errorf("loadIndicators: scan failed: %w", err)
		}
		l.indCache[name] = id
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("loadIndicators: rows error: %w", err)
	}
	return nil
}

func (l *BenchmarkLogger) startRun(notes string) error {
	systemInfo := map[string]interface{}{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"cpu_cores":  runtime.NumCPU(),
		"hostname":   getHostname(),
		"go_version": runtime.Version(),
	}

	infoJSON, _ := json.Marshal(systemInfo)

	var row *sql.Row
	if err := l.withRetry("startRun", func() error {
		row = l.conn.QueryRow(`
			INSERT INTO benchmark_runs (notes, system_info)
			VALUES ($1, $2::jsonb) RETURNING id`,
			notes, infoJSON)
		return nil
	}); err != nil {
		return fmt.Errorf("startRun: %w", err)
	}

	if err := row.Scan(&l.runID); err != nil {
		return fmt.Errorf("startRun: Scan failed: %w", err)
	}

	fmt.Printf("  benchmark run id: %d\n", l.runID)
	return nil
}

func getHostname() string {
	h, _ := os.Hostname()
	return h
}

func (l *BenchmarkLogger) log(indicator, implType string, options []float64,
	timing TimingResult, symbol string, inputSize int) error {

	_, ok := l.indCache[indicator]
	if !ok {
		fmt.Fprintf(os.Stderr, "[warn] '%s' not in indicators table — skipping\n", indicator)
		return fmt.Errorf("indicator not found")
	}

	optsJSON, _ := json.Marshal(options)

	query := `
		INSERT INTO benchmark_results
			(run_id, indicator_id, implementation_type, stock_symbol,
			 data_source, options, mean_time_ns, std_dev_ns,
			 min_time_ns, max_time_ns, sample_count, input_size)
		SELECT $1, id, $2, $3, 'real_data', $4::jsonb,
		       $5, $6, $7, $8, $9, $10
		FROM indicators WHERE name = $11`

	if err := l.withRetry(fmt.Sprintf("log(%s/%s/%s)", indicator, implType, symbol), func() error {
		_, err := l.conn.Exec(query,
			l.runID, implType, symbol, optsJSON,
			timing.MeanNS, timing.StdDevNS, timing.MinNS, timing.MaxNS,
			timing.SampleCount, inputSize, indicator)
		return err
	}); err != nil {
		fmt.Fprintf(os.Stderr, "[warn] INSERT failed for %s/%s/%s: %v\n", indicator, implType, symbol, err)
		return err
	}
	return nil
}

func (l *BenchmarkLogger) Close() error {
	if l.conn != nil {
		l.conn.Close()
		l.conn = nil
	}
	return nil
}

// ---------------------------------------------------------------------
// Output helpers (mirrors C harness line format)
// ---------------------------------------------------------------------

func fmtOptions(opts []float64) string {
	if len(opts) == 0 {
		return "—"
	}
	parts := make([]string, len(opts))
	for i, o := range opts {
		if o == math.Trunc(o) {
			parts[i] = strconv.FormatInt(int64(o), 10)
		} else {
			parts[i] = strconv.FormatFloat(o, 'f', -1, 64)
		}
	}
	return strings.Join(parts, ", ")
}

func printRow(impl, symbol string, opts []float64, t TimingResult) {
	fmt.Printf("    %-8s %-30s %-10s %-16s %10d ns +/- %d\n",
		impl, symbol, fmtOptions(opts), fmt.Sprintf("%d", t.SampleCount),
		t.MeanNS, t.StdDevNS)
}

// ---------------------------------------------------------------------
// Core runner
// ---------------------------------------------------------------------

// runBenchmark measures one indicator in implementation-isolated phases,
// mirroring the Rust criterion layout (bench_rust_ema / bench_c_ema / ...):
// every stock × option set is timed for tulip_rs_go first, then the same
// grid is re-walked for each reference implementation. Interleaving the
// implementations per option set lets one implementation's runtime
// behaviour leak into its neighbour's timing window — most visibly cinar's
// per-call goroutine/channel churn and GC pressure landing directly before
// a tulip measurement. runtime.GC() at each phase boundary keeps that
// hand-off clean.
func runBenchmark(def BenchmarkDef, stocks []Stock, logger *BenchmarkLogger) {
	fmt.Printf("\n--- %s ---\n", def.Name)

	// SIMD by assets — one option set, every stock processed together
	if def.SimdAssetsFn != nil && len(stocks) > 0 {
		for _, opts := range def.Options {
			simdResult := timeFn(func() {
				if err := def.SimdAssetsFn(stocks, opts); err != nil {
					fmt.Fprintf(os.Stderr, "[warn] simd_assets_fn failed: %v\n", err)
				}
			}, BENCH_NUMBER, BENCH_REPEAT, BENCH_WARMUP)
			symbol := fmt.Sprintf("ALL_%d_ASSETS", len(stocks))
			printRow("simd_by_assets", symbol, opts, simdResult)
			if logger != nil {
				if err := logger.log(def.Name, "tulip_rs_go_simd_by_assets", opts,
					simdResult, symbol, len(stocks[0].Close)); err != nil {
					// log failure already printed by logger
				}
			}
		}
	}

	// SIMD by options — one stock, every option set processed together
	if def.SimdOptionsFn != nil && len(def.Options) > 0 {
		for _, s := range stocks {
			simdResult := timeFn(func() {
				if err := def.SimdOptionsFn(s, def.Options); err != nil {
					fmt.Fprintf(os.Stderr, "[warn] simd_options_fn failed for %s: %v\n", s.Symbol, err)
				}
			}, BENCH_NUMBER, BENCH_REPEAT, BENCH_WARMUP)
			optsForLog := [][]float64{def.Options[0]} // first option set only
			printRow("simd_by_options", s.Symbol, optsForLog[0], simdResult)
			if logger != nil {
				if err := logger.log(def.Name, "tulip_rs_go_simd_by_options",
					optsForLog[0], simdResult, s.Symbol, len(s.Close)); err != nil {
					// log failure already printed by logger
				}
			}
		}
	}
	// Phase 1: tulip_rs_go — full stock × option grid.
	for _, s := range stocks {
		for _, opts := range def.Options {
			tulipResult := timeFn(func() {
				if err := def.TulipFn(s, opts); err != nil {
					fmt.Fprintf(os.Stderr, "[warn] tulip_fn failed for %s: %v\n", s.Symbol, err)
				}
			}, BENCH_NUMBER, BENCH_REPEAT, BENCH_WARMUP)
			printRow("tulip_rs_go", s.Symbol, opts, tulipResult)
			if logger != nil {
				if err := logger.log(def.Name, "tulip_rs_go", opts, tulipResult, s.Symbol, len(s.Close)); err != nil {
					// log failure already printed by logger
				}
			}
		}
	}

	
	// Phase 2: Cinar (if provided) — full grid after tulip has finished.
	if def.CinarFn != nil {
		runtime.GC() // clear the previous phase's garbage before timing
		for _, s := range stocks {
			for _, opts := range def.Options {
				cinarResult := timeFn(func() {
					if err := def.CinarFn(s, opts); err != nil {
						fmt.Fprintf(os.Stderr, "[warn] cinar_fn failed for %s: %v\n", s.Symbol, err)
					}
				}, BENCH_NUMBER, 30/*BENCH_REPEAT*/, 20/*BENCH_WARMUP*/)
				printRow("cinar", s.Symbol, opts, cinarResult)
				if logger != nil {
					if err := logger.log(def.Name, "cinar", opts, cinarResult, s.Symbol, len(s.Close)); err != nil {
						// log failure already printed by logger
					}
				}
			}
		}
	}

	// Phase 3: Quantgo (if provided) — full grid after the previous phases.
	if def.QuantgoFn != nil {
		runtime.GC()
		for _, s := range stocks {
			for _, opts := range def.Options {
				qgResult := timeFn(func() {
					if err := def.QuantgoFn(s, opts); err != nil {
						fmt.Fprintf(os.Stderr, "[warn] quantgo_fn failed for %s: %v\n", s.Symbol, err)
					}
				}, BENCH_NUMBER, BENCH_REPEAT, BENCH_WARMUP)
				printRow("quantgo", s.Symbol, opts, qgResult)
				if logger != nil {
					if err := logger.log(def.Name, "quantgo", opts, qgResult, s.Symbol, len(s.Close)); err != nil {
						// log failure already printed by logger
					}
				}
			}
		}
	}

	
}

func RunAll(stocks []Stock) (int64, int) {
	InitConfig()

	// Sort registrations by name for deterministic output order
	sort.Slice(benchDefs, func(i, j int) bool {
		return benchDefs[i].Name < benchDefs[j].Name
	})

	// BENCH_ONLY="bbands,ema" restricts the run to a subset of indicator
	// names — for debugging and targeted re-runs (empty = run everything).
	if only := envStr("BENCH_ONLY", ""); only != "" {
		wanted := make(map[string]bool)
		for _, n := range strings.Split(only, ",") {
			if n = strings.TrimSpace(strings.ToLower(n)); n != "" {
				wanted[n] = true
			}
		}
		filtered := benchDefs[:0]
		for _, def := range benchDefs {
			if wanted[def.Name] {
				filtered = append(filtered, def)
			}
		}
		benchDefs = filtered
		if len(benchDefs) == 0 {
			fmt.Fprintln(os.Stderr, "[error] BENCH_ONLY matched no registered indicators")
			return 0, 0
		}
	}

	var logger *BenchmarkLogger
	if LOG_TO_DB {
		var err error
		logger, err = newBenchmarkLogger()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[warn] failed to create benchmark logger: %v\n", err)
		} else {
			if err := logger.startRun("Go bindings benchmarks -- tulip_rs_go, cinar"); err != nil {
				fmt.Fprintf(os.Stderr, "[warn] startRun failed: %v\n", err)
				logger.Close()
				logger = nil
			}
		}
	}

	var insertedCount int64
	var failedCount int

	for _, def := range benchDefs {
		runBenchmark(def, stocks, logger)
		if logger != nil {
			// Count rows from this run would require DB query; for now we track via log() errors
			// This is a simplified count — actual insertion tracking happens inside logger.log()
		}
	}

	if logger != nil {
		logger.Close()
	}

	fmt.Printf("\n================================================================\n")
	fmt.Printf("  Collected benchmark results\n")

	return insertedCount, failedCount
}
