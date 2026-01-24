package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	startDate  = flag.String("start-date", "2026-01-01", "Start date (YYYY-MM-DD)")
	endDate    = flag.String("end-date", "2026-01-01", "End date (YYYY-MM-DD)")
	symbols    = flag.String("symbols", "ag2502,ag2504", "Comma-separated symbols")
	outputDir  = flag.String("output", "./data/market_data", "Output directory")
	ticksPerDay = flag.Int("ticks", 10000, "Ticks per day")
)

func main() {
	flag.Parse()

	// Parse dates
	start, err := time.Parse("2006-01-02", *startDate)
	if err != nil {
		log.Fatalf("Invalid start date: %v", err)
	}
	end, err := time.Parse("2006-01-02", *endDate)
	if err != nil {
		log.Fatalf("Invalid end date: %v", err)
	}

	// Parse symbols
	symbolList := strings.Split(*symbols, ",")

	log.Printf("Generating mock data...")
	log.Printf("  Date range: %s to %s", *startDate, *endDate)
	log.Printf("  Symbols: %v", symbolList)
	log.Printf("  Ticks per day: %d", *ticksPerDay)
	log.Printf("  Output: %s", *outputDir)

	// Generate data for each date
	for date := start; !date.After(end); date = date.AddDate(0, 0, 1) {
		dateStr := date.Format("20060102")
		log.Printf("Generating data for %s...", dateStr)

		// Create directory
		dayDir := filepath.Join(*outputDir, dateStr)
		if err := os.MkdirAll(dayDir, 0755); err != nil {
			log.Fatalf("Failed to create directory: %v", err)
		}

		// Generate data for each symbol
		for _, symbol := range symbolList {
			filepath := filepath.Join(dayDir, symbol+".csv")
			if err := generateSymbolData(filepath, symbol, date, *ticksPerDay); err != nil {
				log.Fatalf("Failed to generate data for %s: %v", symbol, err)
			}
			log.Printf("  ✓ %s", filepath)
		}
	}

	log.Println("Mock data generation completed!")
}

func generateSymbolData(filepath, symbol string, date time.Time, tickCount int) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{
		"timestamp_ns", "symbol", "exchange", "last_price", "last_volume",
		"bid_price1", "bid_volume1", "ask_price1", "ask_volume1",
	})

	// Generate random walk price
	basePrice := 5000.0 + rand.Float64()*1000.0
	volatility := 0.01

	// Trading hours: 09:00 - 15:00 (6 hours = 21600 seconds)
	// Use UTC+8 (Asia/Shanghai) timezone for Chinese futures market
	loc, _ := time.LoadLocation("Asia/Shanghai")
	startTime := time.Date(date.Year(), date.Month(), date.Day(), 9, 0, 0, 0, loc)
	interval := (6 * time.Hour) / time.Duration(tickCount)

	price := basePrice
	for i := 0; i < tickCount; i++ {
		timestamp := startTime.Add(time.Duration(i) * interval)

		// Random walk
		price += price * volatility * (rand.Float64() - 0.5)

		// Generate bid/ask spread
		spread := price * 0.0002 // 2 bps spread
		bidPrice := price - spread/2
		askPrice := price + spread/2

		// Random volumes
		lastVolume := 1 + rand.Intn(50)
		bidVolume := 50 + rand.Intn(150)
		askVolume := 50 + rand.Intn(150)

		writer.Write([]string{
			fmt.Sprintf("%d", timestamp.UnixNano()),
			symbol,
			"SHFE",
			fmt.Sprintf("%.1f", price),
			fmt.Sprintf("%d", lastVolume),
			fmt.Sprintf("%.1f", bidPrice),
			fmt.Sprintf("%d", bidVolume),
			fmt.Sprintf("%.1f", askPrice),
			fmt.Sprintf("%d", askVolume),
		})
	}

	return nil
}

func init() {
	// Go 1.20+: rand is automatically seeded
	// No need to call rand.Seed()
}
