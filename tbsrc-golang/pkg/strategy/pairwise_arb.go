package strategy

import (
	"log"
	"sync"

	"tbsrc-golang/pkg/client"
	"tbsrc-golang/pkg/config"
	"tbsrc-golang/pkg/execution"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// PairwiseArbStrategy 配对套利策略
// 对应 C++ PairwiseArbStrategy，组合两个 LegManager 实现双腿配对交易
// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp
//       tbsrc/Strategies/include/PairwiseArbStrategy.h
type PairwiseArbStrategy struct {
	// 两腿管理器（Phase 2）
	Leg1 *execution.LegManager // C++: m_firstStrat — 被动腿（报价）
	Leg2 *execution.LegManager // C++: m_secondStrat — 主动腿（对冲）

	// 价差跟踪
	Spread *SpreadTracker

	// 合约信息
	Inst1 *instrument.Instrument // C++: m_firstinstru
	Inst2 *instrument.Instrument // C++: m_secondinstru

	// 阈值
	Thold1 *types.ThresholdSet // C++: m_thold_first
	Thold2 *types.ThresholdSet // C++: m_thold_second

	// 客户端
	Client *client.Client

	// 配置
	StrategyID       int32
	Account          string
	MaxQuoteLevel    int32 // C++: MAX_QUOTE_LEVEL (default 3)
	UseInvisibleBook bool  // C++: m_configParams->m_bUseInvisibleBook

	// 状态
	Active      bool
	AggRepeat   uint32                 // C++: m_agg_repeat — aggressive retry counter
	LastAggSide types.TransactionType  // C++: last_agg_side
	LastAggTS   uint64                 // C++: nanoseconds timestamp of last agg order
	BuyAggOrder  int32                 // C++: m_secondStrat->buyAggOrder
	SellAggOrder int32                 // C++: m_secondStrat->sellAggOrder

	// tvar SHM — 外部调整值
	TVar *shm.TVar // C++: m_tvar — 如果为 nil 则不使用

	// 监控
	LastMonitorTS uint64

	// daily_init 文件路径（用于 HandleSquareoff 时保存状态）
	DailyInitPath string

	// mu 保护所有策略状态，防止 pollMD 和 pollORS 两个 goroutine 并发修改
	// C++ 中 SHM 回调在同一线程中序列化，Go 需要显式加锁
	mu sync.Mutex
}

// NewPairwiseArbStrategy 创建配对套利策略
// 参考: PairwiseArbStrategy.cpp:7-84
func NewPairwiseArbStrategy(
	c *client.Client,
	inst1, inst2 *instrument.Instrument,
	thold1, thold2 *types.ThresholdSet,
	strategyID int32,
	account string,
) *PairwiseArbStrategy {
	// 创建两个 LegManager
	leg1 := execution.NewLegManager(c, inst1, thold1, strategyID, account)
	leg2 := execution.NewLegManager(c, inst2, thold2, strategyID, account)

	// 创建价差跟踪器
	spread := NewSpreadTracker(thold1.Alpha, inst1.TickSize, int32(thold1.AvgSpreadAway))

	maxQuoteLevel := int32(3) // C++ default
	if thold1.MaxQuoteLevel > 0 {
		maxQuoteLevel = int32(thold1.MaxQuoteLevel)
	}

	pas := &PairwiseArbStrategy{
		Leg1:          leg1,
		Leg2:          leg2,
		Spread:        spread,
		Inst1:         inst1,
		Inst2:         inst2,
		Thold1:        thold1,
		Thold2:        thold2,
		Client:        c,
		StrategyID:    strategyID,
		Account:       account,
		MaxQuoteLevel: maxQuoteLevel,
		AggRepeat:     1,
	}

	// C++: ORS 回调需要先经过 PairwiseArbStrategy.ORSCallBack
	// （handleAggOrder + SendAggressiveOrder），再委托给各腿处理。
	// 设置 override 使 client.orderIDMap → LegManager → PairwiseArbStrategy
	leg1.ORSCallbackOverride = pas
	leg2.ORSCallbackOverride = pas

	return pas
}

// Init 从 daily_init 文件初始化
// C++ 从 ../data/daily_init.<strategyID> 加载:
//   avgSpreadRatio_ori, netpos_ytd1, netpos_2day1, netpos_agg2
// 参考: PairwiseArbStrategy.cpp:24-43
func (pas *PairwiseArbStrategy) Init(avgSpreadOri float64, netposYtd1, netpos2day1, netposAgg2 int32) {
	// C++: avgSpreadRatio_ori from file
	pas.Spread.Seed(avgSpreadOri)

	// C++: m_firstStrat->m_netpos_pass = ytd1 + 2day1
	pas.Leg1.State.NetposPass = netposYtd1 + netpos2day1
	pas.Leg1.State.NetposPassYtd = netposYtd1

	// C++: m_secondStrat->m_netpos_agg = ytd2
	pas.Leg2.State.NetposAgg = netposAgg2

	log.Printf("[PairwiseArb] Init: avgSpreadOri=%.4f leg1.netpos_pass=%d leg2.netpos_agg=%d",
		avgSpreadOri, pas.Leg1.State.NetposPass, pas.Leg2.State.NetposAgg)
}

// SetActive 设置策略激活状态
func (pas *PairwiseArbStrategy) SetActive(active bool) {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.setActiveLocked(active)
}

func (pas *PairwiseArbStrategy) setActiveLocked(active bool) {
	pas.Active = active
	pas.Leg1.State.Active = active
	pas.Leg2.State.Active = active
}

// IsActive 返回策略是否激活
func (pas *PairwiseArbStrategy) IsActive() bool {
	return pas.Active
}

// NetExposure 计算净敞口 = leg1.netpos_pass + leg2.netpos_agg + pendingAgg2
// 参考: PairwiseArbStrategy.cpp:717
func (pas *PairwiseArbStrategy) NetExposure() int32 {
	pending := pas.CalcPendingNetposAgg()
	return pas.Leg1.State.NetposPass + pas.Leg2.State.NetposAgg + pending
}

// HandleSquareoff 平仓退出
// 参考: PairwiseArbStrategy.cpp:586-626
// 注意：可能从外部 goroutine（如 SIGINT handler）调用，需要加锁
func (pas *PairwiseArbStrategy) HandleSquareoff() {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.handleSquareoffLocked()
}

// handleSquareoffLocked 平仓退出的内部实现（调用者已持有锁）
func (pas *PairwiseArbStrategy) handleSquareoffLocked() {
	log.Printf("[PairwiseArb] HandleSquareoff triggered")

	// C++: set exit flags on both legs
	pas.Leg1.State.OnExit = true
	pas.Leg1.State.OnCancel = true
	pas.Leg1.State.OnFlat = true
	pas.Leg2.State.OnExit = true
	pas.Leg2.State.OnCancel = true
	pas.Leg2.State.OnFlat = true

	// C++: cancel all outstanding orders on both legs
	pas.Leg1.HandleSquareoff()
	pas.Leg2.HandleSquareoff()

	// C++: deactivate
	pas.setActiveLocked(false)

	// 保存 daily_init 状态（参考 SaveMatrix2）
	if pas.DailyInitPath != "" {
		saveDaily := &config.DailyInit{
			AvgSpreadOri: pas.Spread.AvgSpreadOri,
			NetposYtd1:   pas.Leg1.State.NetposPassYtd,
			Netpos2day1:  pas.Leg1.State.NetposPass - pas.Leg1.State.NetposPassYtd,
			NetposAgg2:   pas.Leg2.State.NetposAgg,
		}
		if err := config.SaveDailyInit(pas.DailyInitPath, saveDaily); err != nil {
			log.Printf("[PairwiseArb] daily_init 保存失败: %v", err)
		} else {
			log.Printf("[PairwiseArb] daily_init 已保存: %s", pas.DailyInitPath)
		}
	}
}

// HandleSquareON 恢复策略
// 参考: PairwiseArbStrategy.cpp:571-584
func (pas *PairwiseArbStrategy) HandleSquareON() {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.Leg1.State.OnExit = false
	pas.Leg1.State.OnCancel = false
	pas.Leg1.State.OnFlat = false
	pas.Leg2.State.OnExit = false
	pas.Leg2.State.OnCancel = false
	pas.Leg2.State.OnFlat = false
	pas.AggRepeat = 1
	pas.setActiveLocked(true)

	log.Printf("[PairwiseArb] HandleSquareON: strategy reactivated")
}
