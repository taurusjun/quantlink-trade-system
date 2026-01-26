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

// ModelWatcher 监控 model 文件变化
type ModelWatcher struct {
	modelFilePath string
	lastModTime   time.Time
	checkInterval time.Duration
	stopChan      chan struct{}
	onReload      func(newParams map[string]interface{}) error
	mu            sync.RWMutex
	enabled       bool
	autoReload    bool

	// 历史记录
	history   []ModelReloadHistory
	historyMu sync.RWMutex
	maxHistory int
}

// ModelWatcherConfig model watcher配置
type ModelWatcherConfig struct {
	ModelFilePath string
	CheckInterval time.Duration
	AutoReload    bool
	OnReload      func(newParams map[string]interface{}) error
}

// NewModelWatcher 创建model watcher
func NewModelWatcher(cfg ModelWatcherConfig) (*ModelWatcher, error) {
	if cfg.ModelFilePath == "" {
		return nil, fmt.Errorf("model file path is empty")
	}

	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 5 * time.Second
	}

	// 检查文件是否存在
	stat, err := os.Stat(cfg.ModelFilePath)
	if err != nil {
		return nil, fmt.Errorf("stat model file: %w", err)
	}

	watcher := &ModelWatcher{
		modelFilePath: cfg.ModelFilePath,
		lastModTime:   stat.ModTime(),
		checkInterval: cfg.CheckInterval,
		stopChan:      make(chan struct{}),
		onReload:      cfg.OnReload,
		enabled:       false,
		autoReload:    cfg.AutoReload,
		history:       make([]ModelReloadHistory, 0, 100),
		maxHistory:    100,
	}

	return watcher, nil
}

// Start 启动watcher
func (w *ModelWatcher) Start() error {
	w.mu.Lock()
	if w.enabled {
		w.mu.Unlock()
		return fmt.Errorf("model watcher already started")
	}
	w.enabled = true
	w.mu.Unlock()

	log.Printf("[ModelWatcher] Started watching: %s", w.modelFilePath)
	log.Printf("[ModelWatcher] Check interval: %v", w.checkInterval)
	log.Printf("[ModelWatcher] Auto reload: %v", w.autoReload)

	go w.watchLoop()

	return nil
}

// Stop 停止watcher
func (w *ModelWatcher) Stop() error {
	w.mu.Lock()
	if !w.enabled {
		w.mu.Unlock()
		return nil
	}
	w.enabled = false
	w.mu.Unlock()

	close(w.stopChan)
	log.Println("[ModelWatcher] Stopped")

	return nil
}

// watchLoop 监控循环
func (w *ModelWatcher) watchLoop() {
	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.RLock()
			autoReload := w.autoReload
			w.mu.RUnlock()

			if autoReload {
				if err := w.checkAndReload(); err != nil {
					log.Printf("[ModelWatcher] Auto reload error: %v", err)
				}
			} else {
				// 只检查不重载
				w.checkFileChange()
			}

		case <-w.stopChan:
			log.Println("[ModelWatcher] Watch loop stopped")
			return
		}
	}
}

// checkFileChange 检查文件是否变化（不重载）
func (w *ModelWatcher) checkFileChange() {
	w.mu.RLock()
	filePath := w.modelFilePath
	lastModTime := w.lastModTime
	w.mu.RUnlock()

	stat, err := os.Stat(filePath)
	if err != nil {
		return
	}

	if stat.ModTime().After(lastModTime) {
		log.Printf("[ModelWatcher] Model file changed: %s (auto reload disabled, use API to reload manually)",
			filePath)
	}
}

// checkAndReload 检查文件并重载
func (w *ModelWatcher) checkAndReload() error {
	w.mu.RLock()
	if !w.enabled {
		w.mu.RUnlock()
		return nil
	}
	filePath := w.modelFilePath
	lastModTime := w.lastModTime
	w.mu.RUnlock()

	// 检查文件修改时间
	stat, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// 文件未修改
	if !stat.ModTime().After(lastModTime) {
		return nil
	}

	log.Printf("[ModelWatcher] Model file changed: %s", filePath)
	log.Printf("[ModelWatcher]   Last modified: %s", stat.ModTime().Format("2006-01-02 15:04:05"))

	// 执行重载
	return w.reload()
}

// Reload 手动触发重载
func (w *ModelWatcher) Reload() error {
	w.mu.RLock()
	if !w.enabled {
		w.mu.RUnlock()
		return fmt.Errorf("model watcher not started")
	}
	w.mu.RUnlock()

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

	// 更新修改时间
	stat, _ := os.Stat(filePath)
	w.mu.Lock()
	w.lastModTime = stat.ModTime()
	w.mu.Unlock()

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

	return map[string]interface{}{
		"enabled":        w.enabled,
		"auto_reload":    w.autoReload,
		"model_file":     w.modelFilePath,
		"last_mod_time":  w.lastModTime.Format("2006-01-02 15:04:05"),
		"check_interval": w.checkInterval.String(),
		"file_exists":    stat != nil,
	}
}

// SetAutoReload 设置自动重载
func (w *ModelWatcher) SetAutoReload(enabled bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.autoReload = enabled
	log.Printf("[ModelWatcher] Auto reload set to: %v", enabled)
}
