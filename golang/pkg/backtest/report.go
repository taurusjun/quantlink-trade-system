package backtest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ReportGenerator generates backtest reports in various formats
type ReportGenerator struct {
	config *BacktestConfig
	result *BacktestResult
}

// NewReportGenerator creates a new report generator
func NewReportGenerator(config *BacktestConfig, result *BacktestResult) *ReportGenerator {
	return &ReportGenerator{
		config: config,
		result: result,
	}
}

// GenerateMarkdown generates a markdown report
func (g *ReportGenerator) GenerateMarkdown() error {
	// Ensure output directory exists
	outputDir := g.config.Backtest.Output.ResultDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("backtest_report_%s.md", timestamp))

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	// Write report
	g.writeMarkdownReport(file)

	fmt.Printf("[Report] Markdown report saved: %s\n", filename)
	return nil
}

// writeMarkdownReport writes the markdown content
func (g *ReportGenerator) writeMarkdownReport(file *os.File) {
	fmt.Fprintf(file, "# 回测报告\n\n")
	fmt.Fprintf(file, "**策略**: %s\n", g.config.Strategy.Type)
	fmt.Fprintf(file, "**日期**: %s 至 %s\n", g.config.Backtest.StartDate, g.config.Backtest.EndDate)
	fmt.Fprintf(file, "**初始资金**: %.2f\n", g.result.InitialCash)
	fmt.Fprintf(file, "**最终资金**: %.2f\n\n", g.result.FinalCash)
	fmt.Fprintf(file, "---\n\n")

	// Performance Summary
	fmt.Fprintf(file, "## 绩效摘要\n\n")
	fmt.Fprintf(file, "| 指标 | 数值 |\n")
	fmt.Fprintf(file, "|------|------|\n")
	fmt.Fprintf(file, "| **总收益** | %.2f |\n", g.result.TotalPNL)
	fmt.Fprintf(file, "| **总收益率** | %.2f%% |\n", g.result.TotalReturn*100)
	fmt.Fprintf(file, "| **年化收益率** | %.2f%% |\n", g.result.AnnualizedReturn*100)
	fmt.Fprintf(file, "| **Sharpe Ratio** | %.2f |\n", g.result.SharpeRatio)
	fmt.Fprintf(file, "| **Sortino Ratio** | %.2f |\n", g.result.SortinoRatio)
	fmt.Fprintf(file, "| **最大回撤** | %.2f%% |\n", g.result.MaxDrawdown*100)
	fmt.Fprintf(file, "| **最大回撤持续期** | %s |\n", g.result.MaxDrawdownDuration.String())
	fmt.Fprintf(file, "| **胜率** | %.2f%% |\n", g.result.WinRate*100)
	fmt.Fprintf(file, "| **盈利因子** | %.2f |\n", g.result.ProfitFactor)
	fmt.Fprintf(file, "| **Calmar Ratio** | %.2f |\n\n", g.result.CalmarRatio)

	// Trade Statistics
	fmt.Fprintf(file, "## 交易统计\n\n")
	fmt.Fprintf(file, "| 指标 | 数值 |\n")
	fmt.Fprintf(file, "|------|------|\n")
	fmt.Fprintf(file, "| **总交易次数** | %d |\n", g.result.TotalTrades)
	fmt.Fprintf(file, "| **盈利交易** | %d |\n", g.result.WinTrades)
	fmt.Fprintf(file, "| **亏损交易** | %d |\n", g.result.LossTrades)
	fmt.Fprintf(file, "| **平均盈利** | %.2f |\n", g.result.AvgWin)
	fmt.Fprintf(file, "| **平均亏损** | %.2f |\n", g.result.AvgLoss)
	fmt.Fprintf(file, "| **最大单笔盈利** | %.2f |\n", g.result.MaxWin)
	fmt.Fprintf(file, "| **最大单笔亏损** | %.2f |\n", g.result.MaxLoss)
	fmt.Fprintf(file, "| **平均交易手数** | %.1f |\n", g.result.AvgTradeSize)
	fmt.Fprintf(file, "| **总手续费** | %.2f |\n\n", g.result.TotalCommission)

	// Daily PNL Table (first 10 days)
	if len(g.result.DailyPNL) > 0 {
		fmt.Fprintf(file, "## 每日PNL（前10天）\n\n")
		fmt.Fprintf(file, "| 日期 | PNL | 收益率 | 交易次数 | 成交量 |\n")
		fmt.Fprintf(file, "|------|-----|--------|---------|-------|\n")

		limit := 10
		if len(g.result.DailyPNL) < limit {
			limit = len(g.result.DailyPNL)
		}

		for i := 0; i < limit; i++ {
			daily := g.result.DailyPNL[i]
			fmt.Fprintf(file, "| %s | %.2f | %.2f%% | %d | %d |\n",
				daily.Date, daily.PNL, daily.Return*100, daily.TradeCount, daily.Volume)
		}
		fmt.Fprintf(file, "\n")

		if len(g.result.DailyPNL) > limit {
			fmt.Fprintf(file, "*...共 %d 天，仅显示前 %d 天*\n\n", len(g.result.DailyPNL), limit)
		}
	}

	// Risk Analysis
	fmt.Fprintf(file, "## 风险分析\n\n")
	fmt.Fprintf(file, "- **Sharpe Ratio**: %.2f %s\n", g.result.SharpeRatio, evaluateSharpe(g.result.SharpeRatio))
	fmt.Fprintf(file, "- **Sortino Ratio**: %.2f %s\n", g.result.SortinoRatio, evaluateSortino(g.result.SortinoRatio))
	fmt.Fprintf(file, "- **最大回撤**: %.2f%% %s\n", g.result.MaxDrawdown*100, evaluateDrawdown(g.result.MaxDrawdown))
	fmt.Fprintf(file, "- **日均波动率**: %.2f%%\n", g.result.AverageDailyVolatility*100)
	fmt.Fprintf(file, "- **盈利因子**: %.2f %s\n\n", g.result.ProfitFactor, evaluateProfitFactor(g.result.ProfitFactor))

	// Configuration
	fmt.Fprintf(file, "## 配置信息\n\n")
	fmt.Fprintf(file, "- **数据路径**: %s\n", g.config.Backtest.Data.DataPath)
	fmt.Fprintf(file, "- **交易品种**: %v\n", g.config.Backtest.Data.Symbols)
	fmt.Fprintf(file, "- **回放模式**: %s\n", g.config.Backtest.Replay.Mode)
	if g.config.Backtest.Replay.Mode == "fast" {
		fmt.Fprintf(file, "- **回放速度**: %.1fx\n", g.config.Backtest.Replay.Speed)
	}
	fmt.Fprintf(file, "- **手续费率**: %.4f%%\n", g.config.GetCommissionRate()*100)
	fmt.Fprintf(file, "- **滑点**: %.1f bps\n\n", g.config.GetSlippage())

	// Footer
	fmt.Fprintf(file, "---\n\n")
	fmt.Fprintf(file, "**报告生成时间**: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(file, "**回测耗时**: %v\n", g.result.Duration)
}

// GenerateJSON generates a JSON report
func (g *ReportGenerator) GenerateJSON() error {
	// Ensure output directory exists
	outputDir := g.config.Backtest.Output.ResultDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("backtest_result_%s.json", timestamp))

	// Marshal result to JSON
	data, err := json.MarshalIndent(g.result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	fmt.Printf("[Report] JSON result saved: %s\n", filename)
	return nil
}

// SaveTrades saves trade history to CSV
func (g *ReportGenerator) SaveTrades() error {
	if !g.config.Backtest.Output.SaveTrades {
		return nil
	}

	// Ensure output directory exists
	outputDir := g.config.Backtest.Output.ResultDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("trades_%s.csv", timestamp))

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create trades file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{
		"TradeID", "OrderID", "Symbol", "Side", "Price", "Volume", "PNL", "Commission", "Timestamp",
	})

	// Write trades
	for _, trade := range g.result.Trades {
		writer.Write([]string{
			trade.TradeID,
			trade.OrderID,
			trade.Symbol,
			trade.Side.String(),
			fmt.Sprintf("%.2f", trade.Price),
			fmt.Sprintf("%d", trade.Volume),
			fmt.Sprintf("%.2f", trade.PNL),
			fmt.Sprintf("%.2f", trade.Commission),
			trade.Timestamp.Format("2006-01-02 15:04:05"),
		})
	}

	fmt.Printf("[Report] Trades saved: %s\n", filename)
	return nil
}

// SaveDailyPNL saves daily PNL to CSV
func (g *ReportGenerator) SaveDailyPNL() error {
	if !g.config.Backtest.Output.SaveDailyPNL {
		return nil
	}

	// Ensure output directory exists
	outputDir := g.config.Backtest.Output.ResultDir
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(outputDir, fmt.Sprintf("daily_pnl_%s.csv", timestamp))

	// Create file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create daily PNL file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.Write([]string{
		"Date", "PNL", "Return", "MaxPNL", "MinPNL", "TradeCount", "Volume",
	})

	// Write daily PNL
	for _, daily := range g.result.DailyPNL {
		writer.Write([]string{
			daily.Date,
			fmt.Sprintf("%.2f", daily.PNL),
			fmt.Sprintf("%.4f", daily.Return),
			fmt.Sprintf("%.2f", daily.MaxPNL),
			fmt.Sprintf("%.2f", daily.MinPNL),
			fmt.Sprintf("%d", daily.TradeCount),
			fmt.Sprintf("%d", daily.Volume),
		})
	}

	fmt.Printf("[Report] Daily PNL saved: %s\n", filename)
	return nil
}

// Helper functions for evaluation

func evaluateSharpe(sharpe float64) string {
	if sharpe > 2.0 {
		return "(优秀)"
	} else if sharpe > 1.0 {
		return "(良好)"
	} else if sharpe > 0.5 {
		return "(一般)"
	}
	return "(较差)"
}

func evaluateSortino(sortino float64) string {
	if sortino > 2.0 {
		return "(优秀)"
	} else if sortino > 1.0 {
		return "(良好)"
	} else if sortino > 0.5 {
		return "(一般)"
	}
	return "(较差)"
}

func evaluateDrawdown(dd float64) string {
	if dd < 0.05 {
		return "(优秀)"
	} else if dd < 0.10 {
		return "(良好)"
	} else if dd < 0.20 {
		return "(可接受)"
	}
	return "(风险较高)"
}

func evaluateProfitFactor(pf float64) string {
	if pf > 2.0 {
		return "(优秀)"
	} else if pf > 1.5 {
		return "(良好)"
	} else if pf > 1.0 {
		return "(盈利)"
	}
	return "(亏损)"
}
