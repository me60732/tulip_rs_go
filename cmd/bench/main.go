package main

import (
	"fmt"
	"os"

	"tulip_rs_go/bench"
)

func main() {
	bench.LoadDotEnv()

	fmt.Println("================================================================")
	fmt.Println("  tulip_rs_go Benchmark Suite")
	fmt.Println("================================================================")

	fmt.Println("\n[1/2] Loading stock data from Postgres...")

	stocks, err := bench.LoadStockData()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] loadStockData failed: %v\n", err)
		os.Exit(1)
	}

	if len(stocks) == 0 {
		fmt.Fprintf(os.Stderr, "[error] no stock data loaded — cannot run benchmarks\n")
		os.Exit(1)
	}

	fmt.Printf("  Loaded %d stocks\n", len(stocks))

	fmt.Println("\n[2/2] Running benchmarks...")
	bench.RunAll(stocks)

	fmt.Println("\n================================================================")
	fmt.Println("  Done")
	fmt.Println("================================================================")
}
