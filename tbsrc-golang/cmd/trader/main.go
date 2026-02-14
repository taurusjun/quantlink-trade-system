package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tbsrc-golang/pkg/api"
	"tbsrc-golang/pkg/client"
	"tbsrc-golang/pkg/config"
	"tbsrc-golang/pkg/connector"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/strategy"
	"tbsrc-golang/pkg/types"
)

func main() {
	// ---- CLI 参数 ----
	configPath := flag.String("config", "config/trader.tbsrc.yaml", "配置文件路径")
	dataDir := flag.String("data", "../data", "daily_init 文件目录")
	flag.Parse()

	// ---- 加载配置 ----
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[main] 配置加载失败: %v", err)
	}
	log.Printf("[main] 配置加载成功: strategy_id=%d symbols=%v",
		cfg.Strategy.StrategyID, cfg.Strategy.Symbols)

	// ---- 验证配置 ----
	if len(cfg.Strategy.Symbols) < 2 {
		log.Fatal("[main] 配对套利需要至少 2 个品种")
	}
	sym1 := cfg.Strategy.Symbols[0]
	sym2 := cfg.Strategy.Symbols[1]

	// ---- 创建 Connector ----
	connCfg := connector.Config{
		MDShmKey:          cfg.ORS.MDShmKey,
		MDQueueSz:         cfg.ORS.MDQueueSize,
		ReqShmKey:         cfg.ORS.ReqShmKey,
		ReqQueueSz:        cfg.ORS.ReqQueueSize,
		RespShmKey:        cfg.ORS.RespShmKey,
		RespQueueSz:       cfg.ORS.RespQueueSize,
		ClientStoreShmKey: cfg.ORS.ClientStoreShmKey,
	}

	// Client 和 Connector 需要互相引用:
	// Connector 需要 Client 的 OnMDUpdate/OnORSUpdate 作为回调
	var cli *client.Client

	exchangeType := exchangeTypeFromString(cfg.Strategy.Instruments[sym1].Exchange)

	conn, err := connector.New(connCfg,
		func(md *shm.MarketUpdateNew) {
			if cli != nil {
				cli.OnMDUpdate(md)
			}
		},
		func(resp *shm.ResponseMsg) {
			if cli != nil {
				cli.OnORSUpdate(resp)
			}
		},
	)
	if err != nil {
		log.Fatalf("[main] Connector 创建失败: %v", err)
	}

	// ---- 创建 Client ----
	cli = client.NewClient(conn, int32(cfg.Strategy.StrategyID),
		cfg.Strategy.Account, cfg.Strategy.Product, exchangeType)

	// ---- 创建 Instruments ----
	icfg1 := cfg.Strategy.Instruments[sym1]
	inst1 := instrument.NewFromConfig(sym1, icfg1.Exchange, icfg1.TickSize, icfg1.LotSize,
		icfg1.ContractFactor, icfg1.PriceMultiplier, icfg1.PriceFactor, icfg1.SendInLots,
		icfg1.Token, icfg1.ExpiryDate)

	icfg2 := cfg.Strategy.Instruments[sym2]
	inst2 := instrument.NewFromConfig(sym2, icfg2.Exchange, icfg2.TickSize, icfg2.LotSize,
		icfg2.ContractFactor, icfg2.PriceMultiplier, icfg2.PriceFactor, icfg2.SendInLots,
		icfg2.Token, icfg2.ExpiryDate)

	cli.RegisterInstrument(inst1)
	cli.RegisterInstrument(inst2)

	log.Printf("[main] 合约创建: %s (tick=%.1f lot=%.0f) %s (tick=%.1f lot=%.0f)",
		sym1, inst1.TickSize, inst1.LotSize, sym2, inst2.TickSize, inst2.LotSize)

	// ---- 创建 ThresholdSets ----
	thold1 := types.NewThresholdSet()
	if m, ok := cfg.Strategy.Thresholds["first"]; ok {
		thold1.LoadFromMap(m)
	}

	thold2 := types.NewThresholdSet()
	if m, ok := cfg.Strategy.Thresholds["second"]; ok {
		thold2.LoadFromMap(m)
	}

	log.Printf("[main] 阈值加载: first.begin_place=%.2f first.max_size=%d second.max_size=%d",
		thold1.BeginPlace, thold1.MaxSize, thold2.MaxSize)

	// ---- 创建 PairwiseArbStrategy ----
	pas := strategy.NewPairwiseArbStrategy(cli, inst1, inst2, thold1, thold2,
		int32(cfg.Strategy.StrategyID), cfg.Strategy.Account)

	// 设置交易所费率
	ec := cfg.Strategy.ExchCosts
	pas.Leg1.SetExchangeCosts(ec.BuyExchTx, ec.SellExchTx, ec.BuyExchContractTx, ec.SellExchContractTx)
	pas.Leg2.SetExchangeCosts(ec.BuyExchTx, ec.SellExchTx, ec.BuyExchContractTx, ec.SellExchContractTx)

	// 注册策略（两个品种都路由到同一个策略）
	cli.RegisterStrategy(sym1, pas)
	cli.RegisterStrategy(sym2, pas)

	// ---- 加载 daily_init ----
	dailyPath := config.DailyInitPath(*dataDir, cfg.Strategy.StrategyID)
	daily, err := config.LoadDailyInit(dailyPath)
	if err != nil {
		log.Printf("[main] daily_init 加载失败 (使用零值): %v", err)
		daily = &config.DailyInit{}
	}
	pas.DailyInitPath = dailyPath
	pas.Init(daily.AvgSpreadOri, daily.NetposYtd1, daily.Netpos2day1, daily.NetposAgg2)
	log.Printf("[main] daily_init: avgSpreadOri=%.4f ytd1=%d 2day1=%d agg2=%d",
		daily.AvgSpreadOri, daily.NetposYtd1, daily.Netpos2day1, daily.NetposAgg2)

	// ---- 打开 tvar SHM ----
	var tvar *shm.TVar
	if thold1.TVarKey > 0 {
		tvar, err = shm.OpenTVar(thold1.TVarKey)
		if err != nil {
			log.Printf("[main] tvar SHM 打开失败 (跳过): %v", err)
		} else if tvar != nil {
			pas.TVar = tvar
			log.Printf("[main] tvar SHM 已连接: key=0x%x", thold1.TVarKey)
		}
	}

	// ---- 启动 API Server ----
	apiPort := cfg.System.APIPort
	if apiPort == 0 {
		apiPort = 9201
	}
	// 从可执行文件相对路径加载 web/ 静态文件
	var webFS fs.FS
	exe, _ := os.Executable()
	webDir := filepath.Join(filepath.Dir(exe), "web")
	if info, err := os.Stat(webDir); err == nil && info.IsDir() {
		webFS = os.DirFS(webDir)
		log.Printf("[main] Web 静态文件目录: %s", webDir)
	} else {
		// 尝试当前工作目录下的 web/
		if info, err := os.Stat("web"); err == nil && info.IsDir() {
			webFS = os.DirFS("web")
			log.Printf("[main] Web 静态文件目录: web/ (当前目录)")
		} else {
			log.Printf("[main] Web 静态文件目录未找到 (仅 REST API 可用)")
		}
	}
	apiServer := api.NewServer(apiPort, webFS)
	apiServer.Start()
	defer apiServer.Stop()
	log.Printf("[main] API Server 已启动: http://localhost:%d/", apiPort)

	// ---- 启动 Connector ----
	conn.Start()
	log.Printf("[main] Connector 已启动，开始接收行情和回报")

	// ---- 激活策略 ----
	pas.SetActive(true)
	log.Printf("[main] 策略已激活: strategy_id=%d", cfg.Strategy.StrategyID)

	// ---- 信号注册（对应 C++ sigfillset + sigwait）----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTSTP)

	// ---- 快照 ticker (1秒) ----
	snapshotTicker := time.NewTicker(1 * time.Second)
	defer snapshotTicker.Stop()

	// ---- 阈值热加载函数 ----
	reloadThresholds := func() {
		newCfg, err := config.Load(*configPath)
		if err != nil {
			log.Printf("[main] 阈值重载失败: %v", err)
			return
		}
		if m, ok := newCfg.Strategy.Thresholds["first"]; ok {
			thold1.LoadFromMap(m)
		}
		if m, ok := newCfg.Strategy.Thresholds["second"]; ok {
			thold2.LoadFromMap(m)
		}
		log.Printf("[main] 阈值已热加载")
	}

	// ---- 主事件循环 ----
	log.Printf("[main] 进入主事件循环 (信号 + Web UI + 快照)")
	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGUSR1:
				// C++: Strategy->m_Active = true; HandleSquareON()
				log.Printf("[main] 收到 SIGUSR1，激活策略")
				pas.HandleSquareON()
			case syscall.SIGUSR2:
				// C++: LoadThresholds(simConfig)
				log.Printf("[main] 收到 SIGUSR2，热加载阈值")
				reloadThresholds()
			case syscall.SIGTSTP:
				// C++: HandleSquareoff()
				log.Printf("[main] 收到 SIGTSTP，平仓退出")
				pas.HandleSquareoff()
			default:
				// SIGINT / SIGTERM → 关闭进程
				log.Printf("[main] 收到 %v，关闭进程", sig)
				goto shutdown
			}

		case <-snapshotTicker.C:
			snap := api.CollectSnapshot(pas)
			apiServer.UpdateSnapshot(snap)

		case cmd := <-apiServer.CommandChan():
			switch cmd.Type {
			case "activate":
				log.Printf("[main] Web UI: 激活策略")
				pas.HandleSquareON()
			case "deactivate":
				log.Printf("[main] Web UI: 停用策略")
				pas.SetActive(false)
			case "squareoff":
				log.Printf("[main] Web UI: 平仓退出")
				pas.HandleSquareoff()
			case "reload_thresholds":
				log.Printf("[main] Web UI: 热加载阈值")
				reloadThresholds()
			}
		}
	}

shutdown:
	// ---- 优雅关闭 ----
	// 1. 停止策略
	if pas.IsActive() {
		pas.HandleSquareoff()
		log.Printf("[main] 策略已平仓退出")
	}

	// 2. 停止 Connector
	conn.Stop()
	log.Printf("[main] Connector 已停止")

	// 3. 保存 daily_init
	saveDaily := &config.DailyInit{
		AvgSpreadOri: pas.Spread.AvgSpreadOri,
		NetposYtd1:   pas.Leg1.State.NetposPassYtd,
		Netpos2day1:  pas.Leg1.State.NetposPass - pas.Leg1.State.NetposPassYtd,
		NetposAgg2:   pas.Leg2.State.NetposAgg,
	}
	if err := config.SaveDailyInit(dailyPath, saveDaily); err != nil {
		log.Printf("[main] daily_init 保存失败: %v", err)
	} else {
		log.Printf("[main] daily_init 已保存: %s", dailyPath)
	}

	// 4. 关闭 tvar
	if tvar != nil {
		tvar.Close()
	}

	// 5. 关闭 Connector SHM
	if err := conn.Close(); err != nil {
		log.Printf("[main] Connector 关闭失败: %v", err)
	}

	log.Printf("[main] 系统关闭完成")
}

// exchangeTypeFromString 将交易所名称转换为 SHM 代码
func exchangeTypeFromString(exchange string) uint8 {
	switch exchange {
	case "SHFE":
		return shm.ChinaSHFE
	case "CFFEX":
		return shm.ChinaCFFEX
	case "ZCE":
		return shm.ChinaZCE
	case "DCE":
		return shm.ChinaDCE
	case "GFEX":
		return shm.ChinaGFEX
	default:
		return shm.ExchangeUnknown
	}
}

func init() {
	// 设置日志格式：添加日期时间和文件信息
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetOutput(os.Stdout)
}

// 确保 PairwiseArbStrategy 实现 StrategyCallback 接口
var _ client.StrategyCallback = (*strategy.PairwiseArbStrategy)(nil)

// 避免 unused import
var _ = fmt.Sprintf
