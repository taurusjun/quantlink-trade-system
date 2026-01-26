package trader

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
)

// ModelReloadHistory 重载历史记录
type ModelReloadHistory struct {
	Timestamp time.Time              `json:"timestamp"`
	FilePath  string                 `json:"file_path"`
	OldParams map[string]interface{} `json:"old_params,omitempty"`
	NewParams map[string]interface{} `json:"new_params"`
	Success   bool                   `json:"success"`
	ErrorMsg  string                 `json:"error_msg,omitempty"`
}

// ModelWatcher 手动重载 model 文件
type ModelWatcher struct {
	modelFilePath string
	onReload      func(newParams map[string]interface{}) error
	mu            sync.RWMutex

	// 历史记录
	history    []ModelReloadHistory
	historyMu  sync.RWMutex
	maxHistory int
}

// ModelWatcherConfig model watcher配置
type ModelWatcherConfig struct {
	ModelFilePath string
	OnReload      func(newParams map[string]interface{}) error
}

// NewModelWatcher 创建model watcher
func NewModelWatcher(cfg ModelWatcherConfig) (*ModelWatcher, error) {
	if cfg.ModelFilePath == "" {
		return nil, fmt.Errorf("model file path is empty")
	}

	// 检查文件是否存在
	if _, err := os.Stat(cfg.ModelFilePath); err != nil {
		return nil, fmt.Errorf("stat model file: %w", err)
	}

	watcher := &ModelWatcher{
		modelFilePath: cfg.ModelFilePath,
		onReload:      cfg.OnReload,
		history:       make([]ModelReloadHistory, 0, 100),
		maxHistory:    100,
	}

	return watcher, nil
}

// Start 启动watcher（手动模式下不需要实际启动）
func (w *ModelWatcher) Start() error {
	log.Printf("[ModelWatcher] Model watcher initialized (manual reload mode)")
	log.Printf("[ModelWatcher] Model file: %s", w.modelFilePath)
	log.Printf("[ModelWatcher] Use API to trigger reload: POST /api/v1/model/reload")
	return nil
}

// Stop 停止watcher
func (w *ModelWatcher) Stop() error {
	log.Println("[ModelWatcher] Model watcher stopped")
	return nil
}

// Reload 手动触发重载
func (w *ModelWatcher) Reload() error {
	log.Println("[ModelWatcher] Manual reload triggered")
	return w.reload()
}

// reload 执行重载
func (w *ModelWatcher) reload() error {
	w.mu.RLock()
	filePath := w.modelFilePath
	w.mu.RUnlock()

	// 解析新参数
	parser := config.NewModelFileParser(filePath)
	newModelParams, err := parser.Parse()
	if err != nil {
		w.recordHistory(filePath, nil, nil, false, err.Error())
		return fmt.Errorf("parse model file: %w", err)
	}

	// 验证参数
	if err := config.ValidateParameters(newModelParams); err != nil {
		w.recordHistory(filePath, nil, newModelParams, false, err.Error())
		return fmt.Errorf("validate parameters: %w", err)
	}

	// 转换为策略参数
	newStrategyParams := config.ConvertModelToStrategyParams(newModelParams)

	log.Printf("[ModelWatcher] Parsed %d model parameters", len(newModelParams))
	log.Printf("[ModelWatcher] Converted to %d strategy parameters", len(newStrategyParams))

	// 打印关键参数
	if entryZ, ok := newStrategyParams["entry_zscore"].(float64); ok {
		log.Printf("[ModelWatcher]   entry_zscore: %.2f", entryZ)
	}
	if exitZ, ok := newStrategyParams["exit_zscore"].(float64); ok {
		log.Printf("[ModelWatcher]   exit_zscore: %.2f", exitZ)
	}
	if size, ok := newStrategyParams["order_size"].(int); ok {
		log.Printf("[ModelWatcher]   order_size: %d", size)
	}

	// 调用回调函数应用新参数
	if w.onReload != nil {
		if err := w.onReload(newStrategyParams); err != nil {
			w.recordHistory(filePath, nil, newStrategyParams, false, err.Error())
			return fmt.Errorf("apply parameters: %w", err)
		}
	}

	// 记录成功历史
	w.recordHistory(filePath, nil, newStrategyParams, true, "")

	log.Println("[ModelWatcher] ✓ Model reloaded successfully")
	return nil
}

// recordHistory 记录重载历史
func (w *ModelWatcher) recordHistory(filePath string, oldParams, newParams map[string]interface{}, success bool, errMsg string) {
	w.historyMu.Lock()
	defer w.historyMu.Unlock()

	record := ModelReloadHistory{
		Timestamp: time.Now(),
		FilePath:  filePath,
		OldParams: oldParams,
		NewParams: newParams,
		Success:   success,
		ErrorMsg:  errMsg,
	}

	w.history = append(w.history, record)

	// 限制历史记录数量
	if len(w.history) > w.maxHistory {
		w.history = w.history[len(w.history)-w.maxHistory:]
	}
}

// GetHistory 获取重载历史
func (w *ModelWatcher) GetHistory(limit int) []ModelReloadHistory {
	w.historyMu.RLock()
	defer w.historyMu.RUnlock()

	if limit <= 0 || limit > len(w.history) {
		limit = len(w.history)
	}

	// 返回最新的N条记录
	start := len(w.history) - limit
	result := make([]ModelReloadHistory, limit)
	copy(result, w.history[start:])

	return result
}

// GetStatus 获取watcher状态
func (w *ModelWatcher) GetStatus() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stat, _ := os.Stat(w.modelFilePath)

	var lastModTime string
	if stat != nil {
		lastModTime = stat.ModTime().Format("2006-01-02 15:04:05")
	}

	return map[string]interface{}{
		"mode":          "manual",
		"model_file":    w.modelFilePath,
		"last_mod_time": lastModTime,
		"file_exists":   stat != nil,
	}
}
