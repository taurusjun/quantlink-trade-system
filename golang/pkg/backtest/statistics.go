package backtest

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// BacktestStatistics collects and calculates backtest statistics
type BacktestStatistics struct {
	config      *BacktestConfig
	result      *BacktestResult
	trades      []*Trade
	dailyPNL    map[string]*DailyPNL
	positions   map[string]int32 // symbol -> position
	lastPrices  map[string]float64
	cashBalance float64
	peakCash    float64
	startTime   time.Time
}

// NewBacktestStatistics creates a new statistics collector
func NewBacktestStatistics(config *BacktestConfig) *BacktestStatistics {
	return &BacktestStatistics{
		config:      config,
		trades:      make([]*Trade, 0, 1000),
		dailyPNL:    make(map[string]*DailyPNL),
		positions:   make(map[string]int32),
		lastPrices:  make(map[string]float64),
		cashBalance: config.Backtest.Initial.Capital,
		peakCash:    config.Backtest.Initial.Capital,
		startTime:   time.Now(),
	}
}

// OnTrade records a trade
func (s *BacktestStatistics) OnTrade(fill *Fill, side orspb.OrderSide, symbol string, commission float64) {
	trade := &Trade{
		TradeID:    fmt.Sprintf("TRADE_%d", len(s.trades)+1),
		OrderID:    fill.OrderID,
		Symbol:     symbol,
		Side:       side,
		Price:      fill.Price,
		Volume:     fill.Volume,
		Commission: commission,
		Timestamp:  fill.Timestamp,
	}

	// Update position
	if side == orspb.OrderSide_BUY {
		s.positions[symbol] += fill.Volume
		s.cashBalance -= float64(fill.Volume) * fill.Price
	} else {
		s.positions[symbol] -= fill.Volume
		s.cashBalance += float64(fill.Volume) * fill.Price
	}

	// Subtract commission
	s.cashBalance -= commission

	// Calculate PNL (for closed positions)
	trade.PNL = s.calculateTradePNL(trade)

	s.trades = append(s.trades, trade)

	// Update daily PNL
	dateKey := fill.Timestamp.Format("2006-01-02")
	if _, exists := s.dailyPNL[dateKey]; !exists {
		s.dailyPNL[dateKey] = &DailyPNL{
			Date: dateKey,
		}
	}
	daily := s.dailyPNL[dateKey]
	daily.PNL += trade.PNL
	daily.TradeCount++
	daily.Volume += int64(fill.Volume)

	// Update peak cash
	if s.cashBalance > s.peakCash {
		s.peakCash = s.cashBalance
	}
}

// UpdatePrice updates the last price for a symbol
func (s *BacktestStatistics) UpdatePrice(symbol string, price float64) {
	s.lastPrices[symbol] = price
}

// calculateTradePNL calculates P&L for a trade
func (s *BacktestStatistics) calculateTradePNL(trade *Trade) float64 {
	// Simplified: assume we're closing a position
	// Real implementation would track average entry price per symbol
	// For now, just return 0 (PNL calculated at end)
	return 0
}

// GenerateReport generates the final backtest report
func (s *BacktestStatistics) GenerateReport() *BacktestResult {
	endTime := time.Now()

	result := &BacktestResult{
		StartTime:   s.startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(s.startTime),
		InitialCash: s.config.Backtest.Initial.Capital,
		FinalCash:   s.cashBalance,
		Trades:      s.trades,
	}

	// Calculate unrealized PNL
	unrealizedPNL := 0.0
	for symbol, position := range s.positions {
		if position != 0 {
			lastPrice := s.lastPrices[symbol]
			unrealizedPNL += float64(position) * lastPrice
		}
	}
	result.FinalCash += unrealizedPNL

	// Convert daily PNL map to slice and sort
	dailyPNLSlice := make([]*DailyPNL, 0, len(s.dailyPNL))
	for _, daily := range s.dailyPNL {
		dailyPNLSlice = append(dailyPNLSlice, daily)
	}
	sort.Slice(dailyPNLSlice, func(i, j int) bool {
		return dailyPNLSlice[i].Date < dailyPNLSlice[j].Date
	})

	// Calculate cumulative PNL and returns
	cumPNL := 0.0
	for _, daily := range dailyPNLSlice {
		cumPNL += daily.PNL
		daily.Return = daily.PNL / result.InitialCash

		// Track max/min PNL for the day
		if cumPNL > daily.MaxPNL {
			daily.MaxPNL = cumPNL
		}
		if cumPNL < daily.MinPNL || daily.MinPNL == 0 {
			daily.MinPNL = cumPNL
		}
	}
	result.DailyPNL = dailyPNLSlice

	// Calculate performance metrics
	result.TotalPNL = result.FinalCash - result.InitialCash
	result.TotalReturn = result.TotalPNL / result.InitialCash

	// Calculate trade statistics
	result.TotalTrades = len(s.trades)
	s.calculateTradeStats(result)

	// Calculate performance ratios
	s.calculatePerformanceMetrics(result)

	s.result = result
	return result
}

// calculateTradeStats calculates trade statistics
func (s *BacktestStatistics) calculateTradeStats(result *BacktestResult) {
	if len(result.Trades) == 0 {
		return
	}

	var totalWin, totalLoss float64
	var totalSize int64
	var maxWin, maxLoss float64

	for _, trade := range result.Trades {
		totalSize += int64(trade.Volume)
		result.TotalCommission += trade.Commission

		if trade.PNL > 0 {
			result.WinTrades++
			totalWin += trade.PNL
			if trade.PNL > maxWin {
				maxWin = trade.PNL
			}
		} else if trade.PNL < 0 {
			result.LossTrades++
			totalLoss += -trade.PNL
			if trade.PNL < maxLoss {
				maxLoss = trade.PNL
			}
		}
	}

	// Win rate
	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinTrades) / float64(result.TotalTrades)
	}

	// Average win/loss
	if result.WinTrades > 0 {
		result.AvgWin = totalWin / float64(result.WinTrades)
	}
	if result.LossTrades > 0 {
		result.AvgLoss = totalLoss / float64(result.LossTrades)
	}

	// Max win/loss
	result.MaxWin = maxWin
	result.MaxLoss = maxLoss

	// Average trade size
	if result.TotalTrades > 0 {
		result.AvgTradeSize = float64(totalSize) / float64(result.TotalTrades)
	}

	// Profit factor
	if totalLoss > 0 {
		result.ProfitFactor = totalWin / totalLoss
	}
}

// calculatePerformanceMetrics calculates Sharpe, Sortino, Max Drawdown etc.
func (s *BacktestStatistics) calculatePerformanceMetrics(result *BacktestResult) {
	if len(result.DailyPNL) == 0 {
		return
	}

	// Extract daily returns
	returns := make([]float64, len(result.DailyPNL))
	for i, daily := range result.DailyPNL {
		returns[i] = daily.Return
	}

	// Average daily return
	result.AverageDailyReturn = mean(returns)

	// Daily volatility (standard deviation)
	result.AverageDailyVolatility = stdDev(returns)

	// Annualized return (252 trading days)
	tradingDays := float64(len(result.DailyPNL))
	if tradingDays > 0 {
		result.AnnualizedReturn = result.TotalReturn * (252.0 / tradingDays)
	}

	// Sharpe Ratio (assume risk-free rate = 0)
	if result.AverageDailyVolatility > 0 {
		result.SharpeRatio = result.AverageDailyReturn / result.AverageDailyVolatility * math.Sqrt(252)
	}

	// Sortino Ratio (downside deviation)
	downsideReturns := make([]float64, 0)
	for _, ret := range returns {
		if ret < 0 {
			downsideReturns = append(downsideReturns, ret)
		}
	}
	if len(downsideReturns) > 0 {
		downsideStdDev := stdDev(downsideReturns)
		if downsideStdDev > 0 {
			result.SortinoRatio = result.AverageDailyReturn / downsideStdDev * math.Sqrt(252)
		}
	}

	// Max Drawdown
	result.MaxDrawdown, result.MaxDrawdownDuration = s.calculateMaxDrawdown(result.DailyPNL)

	// Calmar Ratio
	if result.MaxDrawdown > 0 {
		result.CalmarRatio = result.AnnualizedReturn / result.MaxDrawdown
	}
}

// calculateMaxDrawdown calculates the maximum drawdown
func (s *BacktestStatistics) calculateMaxDrawdown(dailyPNL []*DailyPNL) (float64, time.Duration) {
	if len(dailyPNL) == 0 {
		return 0, 0
	}

	var maxDrawdown float64
	var maxDrawdownDuration time.Duration
	var peak float64
	var peakTime time.Time

	cumPNL := 0.0
	for _, daily := range dailyPNL {
		cumPNL += daily.PNL

		// Update peak
		if cumPNL > peak {
			peak = cumPNL
			peakTime, _ = time.Parse("2006-01-02", daily.Date)
		}

		// Calculate drawdown
		if peak > 0 {
			drawdown := (peak - cumPNL) / peak
			if drawdown > maxDrawdown {
				maxDrawdown = drawdown
				currentTime, _ := time.Parse("2006-01-02", daily.Date)
				maxDrawdownDuration = currentTime.Sub(peakTime)
			}
		}
	}

	return maxDrawdown, maxDrawdownDuration
}

// GetResult returns the current result (may be incomplete)
func (s *BacktestStatistics) GetResult() *BacktestResult {
	if s.result == nil {
		return s.GenerateReport()
	}
	return s.result
}

// Helper functions for statistics

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func stdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	m := mean(values)
	sumSquares := 0.0
	for _, v := range values {
		diff := v - m
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

// PrintSummary prints a summary of the backtest results
func (s *BacktestStatistics) PrintSummary() {
	if s.result == nil {
		s.GenerateReport()
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("BACKTEST SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Printf("\nPeriod: %s to %s (%.0f days)\n",
		s.result.StartTime.Format("2006-01-02"),
		s.result.EndTime.Format("2006-01-02"),
		s.result.Duration.Hours()/24)

	fmt.Printf("\nInitial Capital: %.2f\n", s.result.InitialCash)
	fmt.Printf("Final Capital:   %.2f\n", s.result.FinalCash)
	fmt.Printf("Total PNL:       %.2f (%.2f%%)\n",
		s.result.TotalPNL, s.result.TotalReturn*100)

	fmt.Printf("\nPerformance Metrics:\n")
	fmt.Printf("  Sharpe Ratio:      %.2f\n", s.result.SharpeRatio)
	fmt.Printf("  Sortino Ratio:     %.2f\n", s.result.SortinoRatio)
	fmt.Printf("  Max Drawdown:      %.2f%%\n", s.result.MaxDrawdown*100)
	fmt.Printf("  Calmar Ratio:      %.2f\n", s.result.CalmarRatio)

	fmt.Printf("\nTrade Statistics:\n")
	fmt.Printf("  Total Trades:      %d\n", s.result.TotalTrades)
	fmt.Printf("  Win Trades:        %d (%.1f%%)\n",
		s.result.WinTrades, s.result.WinRate*100)
	fmt.Printf("  Loss Trades:       %d\n", s.result.LossTrades)
	fmt.Printf("  Profit Factor:     %.2f\n", s.result.ProfitFactor)
	fmt.Printf("  Avg Win:           %.2f\n", s.result.AvgWin)
	fmt.Printf("  Avg Loss:          %.2f\n", s.result.AvgLoss)
	fmt.Printf("  Total Commission:  %.2f\n", s.result.TotalCommission)

	fmt.Println(strings.Repeat("=", 60))
}
