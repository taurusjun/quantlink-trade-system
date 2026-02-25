package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
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
	// ---- C++ 模式参数（argv[1]）----
	// C++: ./TradeBot --Live --controlFile ./controls/xxx --strategyID 92201 --configFile ./config/xxx.cfg
	// C++: main.cpp:386 - 第一个参数必须是 --Regress/--Sim/--Live/--LeadLag
	// Go trader 仅支持 --Live 模式（Sim/Regress/LeadLag 依赖 hftbase ExchSim 磁盘回放架构，不适用）
	// 参考: tbsrc/main/main.cpp:372-998, TradeBotUtils.cpp:2590-2608 (GetMode)
	if len(os.Args) < 2 || os.Args[1] != "--Live" {
		fmt.Println("Invalid Arguments!! Example Command is as below.")
		fmt.Println("./trader --Live --controlFile ./controls/xxx --strategyID 92201 --configFile ./config/xxx.cfg")
		if len(os.Args) >= 2 && os.Args[1] != "--Live" {
			fmt.Printf("Error: Go trader 仅支持 --Live 模式（当前: %s）\n", os.Args[1])
		}
		os.Exit(1)
	}
	// C++: cout << "*****TradeBot started in " << argv[1]+2 << " Mode*****"
	log.Printf("[main] *****TradeBot started in Live Mode*****")

	// 移除 --Live 参数后再解析 flag（flag.Parse 处理 os.Args[2:]）
	os.Args = append(os.Args[:1], os.Args[2:]...)

	// ---- CLI 参数（对齐 C++ TradeBot） ----
	controlFile := flag.String("controlFile", "", "C++ controlFile 路径")
	strategyIDStr := flag.String("strategyID", "", "策略 ID")
	configFile := flag.String("configFile", "", "C++ configFile (.cfg) 路径")
	adjustLTP := flag.Int("adjustLTP", 0, "调整最后成交价 (C++ --adjustLTP)")
	printMod := flag.Int("printMod", 0, "打印模式 (C++ --printMod)")
	updateInterval := flag.Int("updateInterval", 0, "更新间隔 (C++ --updateInterval)")
	logFile := flag.String("logFile", "", "日志文件路径 (C++ --logFile)")
	apiPort := flag.Int("apiPort", 9201, "Web UI / REST API 端口")
	yearPrefix := flag.String("yearPrefix", "", "年份后两位 (e.g. 26)，用于 baseName→symbol 映射")
	dataDir := flag.String("dataDir", "./data", "数据目录 (daily_init 等运行时状态，如 ./data/sim 或 ./data/live)")

	flag.Parse()

	_ = adjustLTP
	_ = printMod
	_ = updateInterval

	// ---- 验证必须参数 ----
	if *controlFile == "" {
		log.Fatal("[main] --controlFile 参数必须")
	}
	if *strategyIDStr == "" {
		log.Fatal("[main] --strategyID 参数必须")
	}
	if *configFile == "" {
		log.Fatal("[main] --configFile 参数必须")
	}

	strategyID, err := strconv.Atoi(*strategyIDStr)
	if err != nil {
		log.Fatalf("[main] --strategyID 无效: %v", err)
	}

	// ---- 设置日志输出 ----
	if *logFile != "" {
		f, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("[main] 日志文件打开失败: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	// ---- 从 C++ 格式文件加载配置 ----
	buildParams := config.BuildParams{
		ControlFile: *controlFile,
		ConfigFile:  *configFile,
		StrategyID:  strategyID,
		APIPort:     *apiPort,
		YearPrefix:  *yearPrefix,
	}
	cfg, controlCfg, err := config.BuildFromCppFiles(buildParams)
	if err != nil {
		log.Fatalf("[main] 配置加载失败: %v", err)
	}
	log.Printf("[main] 配置加载成功: strategy_id=%d symbols=%v strat=%s",
		cfg.Strategy.StrategyID, cfg.Strategy.Symbols, controlCfg.ExecStrat)

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
	// C++: PairwiseArbStrategy 构造函数 (PairwiseArbStrategy.cpp:18-28)
	// C++: 路径硬编码为 ../data/daily_init.<strategyID>（C++ CWD=bin/，所以 ../data/ 即部署根目录的 data/）
	// Go: -dataDir 指定数据目录（默认 ./data，模式分离时为 ./data/sim 或 ./data/live）
	dailyPath := config.DailyInitPath(*dataDir, cfg.Strategy.StrategyID)
	log.Printf("[main] dataDir=%s dailyInitPath=%s", *dataDir, dailyPath)
	daily, err := config.LoadMatrix2(dailyPath, int32(cfg.Strategy.StrategyID))
	if err != nil {
		log.Fatalf("[main] daily_init 加载失败: %v", err)
	}
	// C++: PairwiseArbStrategy.cpp:24-28
	if daily.OrigBaseName1 == daily.OrigBaseName2 {
		log.Fatalf("[main] daily_init ERROR! m_origbaseName1:%s m_origbaseName2:%s",
			daily.OrigBaseName1, daily.OrigBaseName2)
	}
	pas.DailyInitPath = dailyPath
	pas.Init(daily.AvgSpreadOri, daily.NetposYtd1, daily.Netpos2day1, daily.NetposAgg2)
	log.Printf("[main] daily_init: avgSpreadOri=%.4f ytd1=%d 2day1=%d agg2=%d origBase1=%s origBase2=%s",
		daily.AvgSpreadOri, daily.NetposYtd1, daily.Netpos2day1, daily.NetposAgg2,
		daily.OrigBaseName1, daily.OrigBaseName2)

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
	srvPort := cfg.System.APIPort
	if srvPort == 0 {
		srvPort = 9201
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
	apiServer := api.NewServer(srvPort, webFS)
	apiServer.Start()
	defer apiServer.Stop()
	log.Printf("[main] API Server 已启动: http://localhost:%d/", srvPort)

	// ---- 启动 Connector ----
	conn.Start()
	log.Printf("[main] Connector 已启动，开始接收行情和回报")

	// ---- 激活策略 ----
	// C++: ExecutionStrategy.cpp:377-380
	// C++: if (m_configParams->m_modeType == ModeType_Sim) m_Active = true; else m_Active = false;
	// Live 模式: 策略启动时不激活，等待 SIGUSR1 信号激活
	log.Printf("[main] 策略未激活 (Live 模式，等待 SIGUSR1 激活): strategy_id=%d", cfg.Strategy.StrategyID)

	// ---- 信号注册（对应 C++ sigfillset + sigwait）----
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM,
		syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTSTP)

	// ---- 快照 ticker (1秒) ----
	snapshotTicker := time.NewTicker(1 * time.Second)
	defer snapshotTicker.Stop()

	// ---- 阈值热加载函数 ----
	// C++: SIGUSR2 触发 LoadThresholds(simConfig) 重新读取 model file
	modelFilePath := controlCfg.ModelFile
	reloadThresholds := func() {
		mc, err := config.ParseModelFile(modelFilePath)
		if err != nil {
			log.Printf("[main] 阈值重载失败: %v", err)
			return
		}
		tholdMap := make(map[string]float64, len(mc.Thresholds))
		for k, v := range mc.Thresholds {
			f, _ := fmt.Sscanf(v, "%f", new(float64))
			if f > 0 {
				var val float64
				fmt.Sscanf(v, "%f", &val)
				tholdMap[config.UpperToSnake(k)] = val
			}
		}
		pas.ReloadThresholds(tholdMap, tholdMap)
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

	// 3. 关闭 tvar
	// 注: daily_init 保存已在 HandleSquareoff 内部完成（对齐 C++ SaveMatrix2 语义），
	// 此处不再重复保存，避免覆盖已手动修正的 daily_init 文件
	if tvar != nil {
		tvar.Close()
	}

	// 4. 关闭 Connector SHM
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
