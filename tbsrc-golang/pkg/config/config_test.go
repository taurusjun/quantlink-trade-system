package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFullConfig(t *testing.T) {
	// 使用项目实际配置文件
	configPath := filepath.Join("..", "..", "config", "trader.tbsrc.yaml")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(%s) error: %v", configPath, err)
	}

	// ORS
	if cfg.ORS.MDShmKey != 0x1001 {
		t.Errorf("MDShmKey = 0x%x, want 0x1001", cfg.ORS.MDShmKey)
	}
	if cfg.ORS.MDQueueSize != 65536 {
		t.Errorf("MDQueueSize = %d, want 65536", cfg.ORS.MDQueueSize)
	}

	// Strategy 基本字段
	if cfg.Strategy.StrategyID != 92201 {
		t.Errorf("StrategyID = %d, want 92201", cfg.Strategy.StrategyID)
	}
	if cfg.Strategy.Account != "PRP05" {
		t.Errorf("Account = %q, want PRP05", cfg.Strategy.Account)
	}
	if cfg.Strategy.Product != "AG" {
		t.Errorf("Product = %q, want AG", cfg.Strategy.Product)
	}
	if len(cfg.Strategy.Symbols) != 2 {
		t.Fatalf("Symbols len = %d, want 2", len(cfg.Strategy.Symbols))
	}
	if cfg.Strategy.Symbols[0] != "ag2506" {
		t.Errorf("Symbols[0] = %q, want ag2506", cfg.Strategy.Symbols[0])
	}

	// Instruments
	if cfg.Strategy.Instruments == nil {
		t.Fatal("Instruments is nil")
	}
	ag2506, ok := cfg.Strategy.Instruments["ag2506"]
	if !ok {
		t.Fatal("ag2506 instrument not found")
	}
	if ag2506.Exchange != "SHFE" {
		t.Errorf("ag2506.Exchange = %q, want SHFE", ag2506.Exchange)
	}
	if ag2506.TickSize != 1.0 {
		t.Errorf("ag2506.TickSize = %f, want 1.0", ag2506.TickSize)
	}
	if ag2506.LotSize != 15 {
		t.Errorf("ag2506.LotSize = %f, want 15", ag2506.LotSize)
	}
	if ag2506.PriceMultiplier != 15.0 {
		t.Errorf("ag2506.PriceMultiplier = %f, want 15.0", ag2506.PriceMultiplier)
	}
	if ag2506.SendInLots != true {
		t.Error("ag2506.SendInLots should be true")
	}
	if ag2506.ExpiryDate != 20250615 {
		t.Errorf("ag2506.ExpiryDate = %d, want 20250615", ag2506.ExpiryDate)
	}

	// Thresholds
	if cfg.Strategy.Thresholds == nil {
		t.Fatal("Thresholds is nil")
	}
	first, ok := cfg.Strategy.Thresholds["first"]
	if !ok {
		t.Fatal("first threshold set not found")
	}
	if first["begin_place"] != 0.35 {
		t.Errorf("first.begin_place = %f, want 0.35", first["begin_place"])
	}
	if first["max_size"] != 5 {
		t.Errorf("first.max_size = %f, want 5", first["max_size"])
	}
	if first["spread_ewa"] != 0.6 {
		t.Errorf("first.spread_ewa = %f, want 0.6", first["spread_ewa"])
	}

	second, ok := cfg.Strategy.Thresholds["second"]
	if !ok {
		t.Fatal("second threshold set not found")
	}
	if second["max_size"] != 10 {
		t.Errorf("second.max_size = %f, want 10", second["max_size"])
	}

	// Exchange costs
	if cfg.Strategy.ExchCosts.BuyExchTx != 0.0 {
		t.Errorf("BuyExchTx = %f, want 0.0", cfg.Strategy.ExchCosts.BuyExchTx)
	}
}

func TestLoadMinimalConfig(t *testing.T) {
	content := `
ors:
  md_shm_key: 0x1001
  md_queue_size: 1024
  req_shm_key: 0x2001
  req_queue_size: 512
  resp_shm_key: 0x3001
  resp_queue_size: 512
  client_store_shm_key: 0x4001
strategy:
  strategy_id: 100
  symbols: [test1]
system:
  log_level: debug
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.Strategy.StrategyID != 100 {
		t.Errorf("StrategyID = %d, want 100", cfg.Strategy.StrategyID)
	}
	// Instruments and Thresholds should be nil/empty when not specified
	if len(cfg.Strategy.Instruments) != 0 {
		t.Errorf("Instruments should be empty, got %d entries", len(cfg.Strategy.Instruments))
	}
}

func TestLoadMissingShmKey(t *testing.T) {
	content := `
ors:
  md_shm_key: 0
  req_shm_key: 0x2001
  resp_shm_key: 0x3001
  client_store_shm_key: 0x4001
strategy:
  strategy_id: 1
  symbols: [x]
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Error("expected validation error for zero md_shm_key")
	}
}
