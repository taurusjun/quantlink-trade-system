package backtest

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

// HistoricalDataReader reads historical market data and replays it through NATS
type HistoricalDataReader struct {
	config   *BacktestConfig
	natsConn *nats.Conn
	ticks    []*MarketDataTick
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewHistoricalDataReader creates a new data reader
func NewHistoricalDataReader(config *BacktestConfig, natsConn *nats.Conn) (*HistoricalDataReader, error) {
	ctx, cancel := context.WithCancel(context.Background())

	reader := &HistoricalDataReader{
		config:   config,
		natsConn: natsConn,
		ticks:    make([]*MarketDataTick, 0, 10000),
		ctx:      ctx,
		cancel:   cancel,
	}

	return reader, nil
}

// LoadData loads historical data from CSV files
func (r *HistoricalDataReader) LoadData() error {
	log.Println("[DataReader] Loading historical data...")

	// Get date range
	startDate, err := time.Parse("2006-01-02", r.config.Backtest.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start date: %w", err)
	}
	endDate, err := time.Parse("2006-01-02", r.config.Backtest.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end date: %w", err)
	}

	// Parse time range
	startTime, err := time.Parse("15:04:05", r.config.Backtest.StartTime)
	if err != nil {
		return fmt.Errorf("invalid start time: %w", err)
	}
	endTime, err := time.Parse("15:04:05", r.config.Backtest.EndTime)
	if err != nil {
		return fmt.Errorf("invalid end time: %w", err)
	}

	// Load data for each date and symbol
	for date := startDate; !date.After(endDate); date = date.AddDate(0, 0, 1) {
		dateStr := date.Format("20060102")

		for _, symbol := range r.config.Backtest.Data.Symbols {
			// Construct file path: data_path/YYYYMMDD/symbol.csv
			filePath := filepath.Join(r.config.Backtest.Data.DataPath, dateStr, symbol+".csv")

			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				log.Printf("[DataReader] Warning: data file not found: %s", filePath)
				continue
			}

			// Load ticks from file
			ticks, err := r.loadTicksFromCSV(filePath, date, startTime, endTime)
			if err != nil {
				log.Printf("[DataReader] Error loading %s: %v", filePath, err)
				continue
			}

			r.ticks = append(r.ticks, ticks...)
			log.Printf("[DataReader] Loaded %d ticks from %s", len(ticks), filePath)
		}
	}

	if len(r.ticks) == 0 {
		return fmt.Errorf("no data loaded")
	}

	// Sort all ticks by timestamp
	sort.Slice(r.ticks, func(i, j int) bool {
		return r.ticks[i].TimestampNs < r.ticks[j].TimestampNs
	})

	log.Printf("[DataReader] Total ticks loaded: %d", len(r.ticks))
	log.Printf("[DataReader] Time range: %s to %s",
		time.Unix(0, r.ticks[0].TimestampNs).Format("2006-01-02 15:04:05"),
		time.Unix(0, r.ticks[len(r.ticks)-1].TimestampNs).Format("2006-01-02 15:04:05"))

	return nil
}

// loadTicksFromCSV loads ticks from a single CSV file
func (r *HistoricalDataReader) loadTicksFromCSV(filePath string, date time.Time, startTime, endTime time.Time) ([]*MarketDataTick, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	// Validate header (basic check)
	if len(header) < 9 {
		return nil, fmt.Errorf("invalid CSV format: expected at least 9 columns, got %d", len(header))
	}

	ticks := make([]*MarketDataTick, 0, 1000)

	// Read data rows
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		// Parse tick
		tick, err := r.parseCSVRecord(record)
		if err != nil {
			// Skip invalid rows
			continue
		}

		// Filter by time range
		tickTime := time.Unix(0, tick.TimestampNs)
		tickTimeOfDay := tickTime.Hour()*3600 + tickTime.Minute()*60 + tickTime.Second()
		startTimeOfDay := startTime.Hour()*3600 + startTime.Minute()*60 + startTime.Second()
		endTimeOfDay := endTime.Hour()*3600 + endTime.Minute()*60 + endTime.Second()

		if tickTimeOfDay < startTimeOfDay || tickTimeOfDay > endTimeOfDay {
			continue
		}

		ticks = append(ticks, tick)
	}

	return ticks, nil
}

// parseCSVRecord parses a single CSV record into MarketDataTick
func (r *HistoricalDataReader) parseCSVRecord(record []string) (*MarketDataTick, error) {
	if len(record) < 9 {
		return nil, fmt.Errorf("invalid CSV record: expected at least 9 fields, got %d", len(record))
	}

	// Parse fields
	timestampNs, err := strconv.ParseInt(record[0], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	lastPrice, err := strconv.ParseFloat(record[3], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid last_price: %w", err)
	}

	lastVolume, err := strconv.ParseInt(record[4], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid last_volume: %w", err)
	}

	bidPrice1, err := strconv.ParseFloat(record[5], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid bid_price1: %w", err)
	}

	bidVolume1, err := strconv.ParseInt(record[6], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid bid_volume1: %w", err)
	}

	askPrice1, err := strconv.ParseFloat(record[7], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid ask_price1: %w", err)
	}

	askVolume1, err := strconv.ParseInt(record[8], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid ask_volume1: %w", err)
	}

	tick := &MarketDataTick{
		TimestampNs: timestampNs,
		Symbol:      record[1],
		Exchange:    record[2],
		LastPrice:   lastPrice,
		LastVolume:  int32(lastVolume),
		BidPrice1:   bidPrice1,
		BidVolume1:  int32(bidVolume1),
		AskPrice1:   askPrice1,
		AskVolume1:  int32(askVolume1),
	}

	// Parse additional levels if available
	if len(record) >= 19 {
		tick.BidPrice2, _ = strconv.ParseFloat(record[9], 64)
		tick.BidVolume2, _ = int32Conv(record[10])
		tick.AskPrice2, _ = strconv.ParseFloat(record[11], 64)
		tick.AskVolume2, _ = int32Conv(record[12])
		tick.BidPrice3, _ = strconv.ParseFloat(record[13], 64)
		tick.BidVolume3, _ = int32Conv(record[14])
		tick.AskPrice3, _ = strconv.ParseFloat(record[15], 64)
		tick.AskVolume3, _ = int32Conv(record[16])
		tick.BidPrice4, _ = strconv.ParseFloat(record[17], 64)
		tick.BidVolume4, _ = int32Conv(record[18])
	}

	if len(record) >= 29 {
		tick.AskPrice4, _ = strconv.ParseFloat(record[19], 64)
		tick.AskVolume4, _ = int32Conv(record[20])
		tick.BidPrice5, _ = strconv.ParseFloat(record[21], 64)
		tick.BidVolume5, _ = int32Conv(record[22])
		tick.AskPrice5, _ = strconv.ParseFloat(record[23], 64)
		tick.AskVolume5, _ = int32Conv(record[24])
	}

	return tick, nil
}

// int32Conv converts string to int32
func int32Conv(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	return int32(v), err
}

// Start starts the data reader (loads data if not already loaded)
func (r *HistoricalDataReader) Start() error {
	if len(r.ticks) == 0 {
		if err := r.LoadData(); err != nil {
			return err
		}
	}
	return nil
}

// Replay replays the loaded data through NATS
func (r *HistoricalDataReader) Replay() error {
	if len(r.ticks) == 0 {
		return fmt.Errorf("no data to replay")
	}

	log.Println("[DataReader] Starting data replay...")

	mode := r.config.GetReplayMode()
	speed := r.config.GetReplaySpeed()

	var prevTimestamp int64
	tickCount := 0

	for i, tick := range r.ticks {
		// Check for cancellation
		select {
		case <-r.ctx.Done():
			log.Println("[DataReader] Replay cancelled")
			return nil
		default:
		}

		// Calculate delay based on replay mode
		if mode == ReplayModeRealtime || mode == ReplayModeFast {
			if prevTimestamp > 0 {
				interval := tick.TimestampNs - prevTimestamp
				delay := time.Duration(interval)

				if mode == ReplayModeFast {
					delay = time.Duration(float64(interval) / speed)
				}

				if delay > 0 {
					time.Sleep(delay)
				}
			}
		}
		// For ReplayModeInstant, no delay

		// Convert to protobuf
		md := tick.ToProtobuf()

		// Serialize
		data, err := proto.Marshal(md)
		if err != nil {
			log.Printf("[DataReader] Failed to marshal tick: %v", err)
			continue
		}

		// Publish to NATS
		// Topic format: md.{exchange}.{symbol}
		topic := fmt.Sprintf("md.%s.%s", tick.Exchange, tick.Symbol)
		if err := r.natsConn.Publish(topic, data); err != nil {
			log.Printf("[DataReader] Failed to publish to %s: %v", topic, err)
			continue
		}

		prevTimestamp = tick.TimestampNs
		tickCount++

		// Log progress every 1000 ticks
		if (i+1)%1000 == 0 {
			log.Printf("[DataReader] Replayed %d/%d ticks (%.1f%%)",
				i+1, len(r.ticks), float64(i+1)/float64(len(r.ticks))*100)
		}
	}

	log.Printf("[DataReader] Replay completed: %d ticks", tickCount)
	return nil
}

// Stop stops the data reader
func (r *HistoricalDataReader) Stop() error {
	r.cancel()
	return nil
}

// GetTickCount returns the number of loaded ticks
func (r *HistoricalDataReader) GetTickCount() int {
	return len(r.ticks)
}

// GetTimeRange returns the time range of loaded data
func (r *HistoricalDataReader) GetTimeRange() (time.Time, time.Time) {
	if len(r.ticks) == 0 {
		return time.Time{}, time.Time{}
	}
	start := time.Unix(0, r.ticks[0].TimestampNs)
	end := time.Unix(0, r.ticks[len(r.ticks)-1].TimestampNs)
	return start, end
}
