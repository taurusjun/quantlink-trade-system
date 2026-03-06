package com.quantlink.trader;

import com.quantlink.trader.api.ApiServer;
import com.quantlink.trader.api.SnapshotCollector;
import com.quantlink.trader.config.*;
import com.quantlink.trader.connector.Connector;
import com.quantlink.trader.core.*;
import com.quantlink.trader.core.Watch;
import com.quantlink.trader.shm.Types;
import com.quantlink.trader.strategy.PairwiseArbStrategy;

import java.lang.foreign.MemorySegment;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.logging.FileHandler;
import java.util.logging.Logger;
import java.util.logging.SimpleFormatter;

/**
 * Trader 主程序入口。
 * 迁移自: tbsrc/main/main.cpp:372-1103
 * 对齐: tbsrc-golang/cmd/trader/main.go
 *
 * 启动方式 (与 C++/Go 一致):
 *   java -jar trader.jar --Live -controlFile ./controls/xxx -strategyID 92201 -configFile ./config/xxx.cfg
 */
public class TraderMain {

    private static final Logger logger = Logger.getLogger(TraderMain.class.getName());

    // 可测试性: 允许注入关闭信号
    final CountDownLatch shutdownLatch = new CountDownLatch(1);
    volatile boolean running = true;

    // 组件引用 (包内可见, 用于测试)
    Connector connector;
    CommonClient client;
    PairwiseArbStrategy strategy;
    String dailyInitPath;

    // Web 控制台 — 对齐 Go pkg/api/
    ApiServer apiServer;
    SnapshotCollector snapshotCollector;

    // CLI 参数
    String controlFile;
    String configFile;
    int strategyID;
    String dataDir = "./data";
    String yearPrefix = "";
    String logFile;
    int printMod;

    /**
     * 解析 CLI 参数。
     * 迁移自: tbsrc/main/main.cpp:372-605
     * C++: argv[1] 必须是 --Regress/--Sim/--Live/--LeadLag
     */
    public void parseArgs(String[] args) {
        if (args.length < 1 || !"--Live".equals(args[0])) {
            System.out.println("Invalid Arguments!! Example Command is as below.");
            System.out.println("java -jar trader.jar --Live -controlFile ./controls/xxx -strategyID 92201 -configFile ./config/xxx.cfg");
            throw new IllegalArgumentException("必须指定 --Live 模式");
        }

        for (int i = 1; i < args.length; i++) {
            switch (args[i]) {
                case "-controlFile" -> controlFile = args[++i];
                case "-strategyID" -> strategyID = Integer.parseInt(args[++i]);
                case "-configFile" -> configFile = args[++i];
                case "-dataDir" -> dataDir = args[++i];
                case "-yearPrefix" -> yearPrefix = args[++i];
                case "-logFile" -> logFile = args[++i];
                case "-printMod" -> printMod = Integer.parseInt(args[++i]);
                case "-adjustLTP", "-updateInterval" -> i++; // consume value, ignore
                default -> { /* ignore unknown flags */ }
            }
        }

        if (controlFile == null) throw new IllegalArgumentException("-controlFile 参数必须");
        if (configFile == null) throw new IllegalArgumentException("-configFile 参数必须");
        if (strategyID == 0) throw new IllegalArgumentException("-strategyID 参数必须");

        if (yearPrefix.isEmpty()) {
            yearPrefix = String.format("%02d", java.time.Year.now().getValue() % 100);
        }
    }

    /**
     * 初始化并启动 Trader。
     * 迁移自: tbsrc/main/main.cpp:606-1098
     */
    public void init() throws Exception {
        // ---- 日志设置 ----
        if (logFile != null && !logFile.isEmpty()) {
            FileHandler fh = new FileHandler(logFile, true);
            fh.setFormatter(new SimpleFormatter());
            Logger.getLogger("").addHandler(fh);
        }

        logger.info("*****TradeBot started in Live Mode*****");

        // ---- Step 0: 创建 Watch 全局时钟单例 ----
        // C++: Watch::CreateUniqueInstance(0);
        // Ref: main.cpp:650
        Watch.createInstance(0);

        // ---- Step 1: 加载 controlFile ----
        // C++: LoadControlFile(simConfig[0].m_controlConfig, controlFile)
        ControlConfig controlCfg = ControlConfig.loadControlFile(controlFile);
        logger.info(String.format("[config] controlFile: baseName=%s model=%s exchange=%s strat=%s second=%s",
            controlCfg.baseName, controlCfg.modelFile, controlCfg.exchange,
            controlCfg.execStrat, controlCfg.secondName));

        // ---- Step 2: 加载 .cfg ----
        // C++: illuminati::Configfile::LoadCfg(configParams->m_configFile)
        CfgConfig cfgConfig = CfgConfig.loadCfg(configFile);
        logger.info(String.format("[config] configFile: product=%s exchanges=%s",
            cfgConfig.product, cfgConfig.exchanges));

        // ---- Step 3: 加载 model file ----
        // C++: LoadModelFile(simConfig[i], tholdMap)
        ModelConfig modelCfg = ModelConfig.loadModelFile(controlCfg.modelFile);
        logger.info(String.format("[config] modelFile: %d thresholds", modelCfg.thresholds.size()));

        // ---- Step 4: baseName → symbol 映射 ----
        String sym1 = ConfigParser.baseNameToSymbol(controlCfg.baseName, yearPrefix);
        String sym2 = null;
        if (controlCfg.secondName != null && !controlCfg.secondName.isEmpty()) {
            sym2 = ConfigParser.baseNameToSymbol(controlCfg.secondName, yearPrefix);
        }
        logger.info(String.format("[config] symbols: %s → %s, %s → %s",
            controlCfg.baseName, sym1, controlCfg.secondName, sym2));

        if (sym2 == null) {
            throw new IllegalStateException("配对套利需要至少 2 个品种");
        }

        // ---- Step 5: 获取 SHM 配置 ----
        // C++: cfg.GetExchangeConfig(exchangeSection)
        int[] shmCfg = cfgConfig.getExchangeShmConfig(cfgConfig.exchanges);
        int mdKey = shmCfg[0], reqKey = shmCfg[1], respKey = shmCfg[2], clientStoreKey = shmCfg[3];
        int mdSize = shmCfg[4], reqSize = shmCfg[5], respSize = shmCfg[6];
        logger.info(String.format("[config] SHM: md=0x%x req=0x%x resp=0x%x cs=0x%x",
            mdKey, reqKey, respKey, clientStoreKey));

        // ---- Step 6: 创建 Connector ----
        // C++: ConnectorConfig connectorCfg; connectorCfg.init(cfg);
        // C++: Connector *connector = new Connector(MDcb, ORScb, connectorCfg.INTERACTION_MODE, &connectorCfg);
        // Ref: tbsrc/main/main.cpp:1030-1073

        // C++ ConnectorConfig 字段映射:
        //   INTERESTED_EXCHANGES → exchanges (ExchangeConfig list)
        //   INTERESTED_SYMBOLS → interestedSymbols
        //   EXCH_*_MAP → ExchangeConfig 内部字段
        Connector.Config connCfg = new Connector.Config();

        // C++: INTERESTED_SYMBOLS = TICKERS (排序后的合约列表)
        // Ref: hftbase/Connector/include/connectorconfig.h:102-104
        // 构造函数内部会自动分配 symbolID (0, 1, 2, ...)
        {
            String[] sortedInterested = {sym1, sym2};
            Arrays.sort(sortedInterested);
            for (String s : sortedInterested) {
                connCfg.interestedSymbols.add(s);
            }
        }

        // C++: INTERESTED_EXCHANGES → EXCH_*_MAP
        // Ref: hftbase/Connector/include/connectorconfig.h:287-330
        Connector.ExchangeConfig exchCfg = new Connector.ExchangeConfig();
        exchCfg.exchangeName = cfgConfig.exchanges;  // e.g. "CHINA_SHFE"
        exchCfg.mdShmKeys.add(mdKey);
        exchCfg.mdShmSizes.add(mdSize);
        exchCfg.mdShmReadModes.add(Connector.MD_READ_ROUND_ROBIN);
        exchCfg.reqShmKey = reqKey;
        exchCfg.reqQueueSize = reqSize;
        exchCfg.respShmKey = respKey;
        exchCfg.respQueueSize = respSize;
        exchCfg.clientStoreShmKey = clientStoreKey;
        connCfg.exchanges.add(exchCfg);

        // C++: new Connector(MDcb, ORScb, INTERACTION_MODE, &connectorCfg)
        // [C++差异] Java 省略 InteractionMode 参数 (仅实现 LIVE 模式)
        connector = new Connector(
            this::onMarketData,
            this::onOrderResponse,
            connCfg
        );
        logger.info("[main] Connector 已创建, clientId=" + connector.getClientId());

        // ---- Step 7: 创建 CommonClient ----
        // C++: client->Initialize(&MDcallback, &ORScallback, ...)
        client = new CommonClient();
        client.setConnector(connector);

        // C++: CommonClient.cpp:850-901 — m_exchangeType 从 exchange 字符串映射
        // C++: FillReqInfo() 中 m_reqMsg.Exchange_Type = m_exchangeType
        client.setExchangeType(CfgConfig.parseExchangeType(cfgConfig.exchanges));

        ConfigParams params = ConfigParams.getInstance();
        params.strategyID = strategyID;
        params.modeType = 2; // Live mode
        params.printMode = printMod;

        // ---- Step 8: 创建 Instruments ----
        // C++: AddInstrument(simConfig, controlConfig.baseName)
        String product = ConfigParser.extractProduct(sym1);
        double tickSize = ConfigParser.getTickSize(product);
        double lotSize = ConfigParser.getLotSize(product);

        Instrument instru1 = new Instrument();
        instru1.symbol = sym1;
        // C++: strcpy(m_instrument, symbol); strcpy(m_symbol, m_instrument);
        // Ref: tbsrc/common/Instrument.cpp:913-914 — 中国期货 m_instrument == m_symbol
        instru1.instrument = sym1;
        instru1.origBaseName = controlCfg.baseName;
        instru1.exchange = controlCfg.exchange != null ? controlCfg.exchange : "";
        instru1.tickSize = tickSize;
        instru1.lotSize = lotSize;
        instru1.priceMultiplier = lotSize;
        instru1.priceFactor = 1.0;
        instru1.sendInLots = true;

        Instrument instru2 = new Instrument();
        instru2.symbol = sym2;
        // C++: strcpy(m_instrument, symbol); strcpy(m_symbol, m_instrument);
        // Ref: tbsrc/common/Instrument.cpp:913-914 — 中国期货 m_instrument == m_symbol
        instru2.instrument = sym2;
        instru2.origBaseName = controlCfg.secondName;
        instru2.exchange = controlCfg.exchange != null ? controlCfg.exchange : "";
        instru2.tickSize = tickSize;
        instru2.lotSize = lotSize;
        instru2.priceMultiplier = lotSize;
        instru2.priceFactor = 1.0;
        instru2.sendInLots = true;

        logger.info(String.format("[main] 合约创建: %s (tick=%.1f lot=%.0f) %s (tick=%.1f lot=%.0f)",
            sym1, tickSize, lotSize, sym2, tickSize, lotSize));

        // 注册 symbol → SimConfig 映射
        // Go: c.instruments[inst.Symbol] = inst
        // Ref: tbsrc-golang/pkg/client/client.go:RegisterInstrument()
        SimConfig simConfig1 = new SimConfig();
        simConfig1.instrument = instru1;
        simConfig1.instrumentSec = instru2;
        simConfig1.useArbStrat = true;
        simConfig1.strategyID = strategyID;
        simConfig1.startTime = controlCfg.startTime != null ? controlCfg.startTime : "";
        simConfig1.endTime = controlCfg.endTime != null ? controlCfg.endTime : "";
        // C++: LoadDateConfigEpoch(simConfig, argMap)
        // Ref: TradeBotUtils.cpp:2568-2588
        simConfig1.initDateConfigEpoch();

        List<SimConfig> simList1 = new ArrayList<>();
        simList1.add(simConfig1);
        params.simConfigMap.put(sym1, simList1);

        SimConfig simConfig2 = new SimConfig();
        simConfig2.instrument = instru2;
        simConfig2.instrumentSec = instru1;
        simConfig2.useArbStrat = true;
        simConfig2.strategyID = strategyID;
        simConfig2.startTime = controlCfg.startTime != null ? controlCfg.startTime : "";
        simConfig2.endTime = controlCfg.endTime != null ? controlCfg.endTime : "";
        // C++: LoadDateConfigEpoch(simConfig, argMap)
        // Ref: TradeBotUtils.cpp:2568-2588
        simConfig2.initDateConfigEpoch();

        List<SimConfig> simList2 = new ArrayList<>();
        simList2.add(simConfig2);
        params.simConfigMap.put(sym2, simList2);

        // instruMap: symbol → Instrument
        simConfig1.instruMap.put(sym1, instru1);
        simConfig1.instruMap.put(sym2, instru2);
        simConfig2.instruMap.put(sym1, instru1);
        simConfig2.instruMap.put(sym2, instru2);

        // ---- symbolID 数组构建 ----
        // 迁移自: hftbase/Connector/src/connector.cpp:48-62 — Connector::AddSymbol()
        // C++: std::set<string> sorted → assign symbolID 0,1,2... in sorted order
        // C++: m_configParams->m_simConfigList[symbolID] = m_simConfigMap.find(symbol)
        // C++: simConfig->m_instruList[symbolID] = simConfig->m_instruMap.find(symbol)
        // md_shm_feeder 使用相同排序逻辑 (BuildSymbolIDMap)
        String[] sortedSymbols = {sym1, sym2};
        Arrays.sort(sortedSymbols);

        int numSymbols = sortedSymbols.length;
        @SuppressWarnings("unchecked")
        List<SimConfig>[] scList = new List[numSymbols];
        Instrument[] iList1 = new Instrument[numSymbols];
        Instrument[] iList2 = new Instrument[numSymbols];

        for (int i = 0; i < numSymbols; i++) {
            String s = sortedSymbols[i];
            scList[i] = params.simConfigMap.get(s);
            iList1[i] = simConfig1.instruMap.get(s);
            iList2[i] = simConfig2.instruMap.get(s);
            logger.info(String.format("[main] symbolID %d → %s", i, s));
        }
        params.simConfigList = scList;
        simConfig1.instruList = iList1;
        simConfig2.instruList = iList2;

        // ---- Step 9: 加载阈值 ----
        // C++: LoadModelFile → ThresholdSet
        // SimConfig.thresholdSet is final — 直接加载到现有实例
        ConfigParser.loadThresholds(simConfig1.thresholdSet, modelCfg.thresholds);
        ConfigParser.loadThresholds(simConfig2.thresholdSet, modelCfg.thresholds);

        logger.info(String.format("[main] 阈值: BEGIN_PLACE=%.4f LONG_PLACE=%.4f MAX_SIZE=%d",
            simConfig1.thresholdSet.BEGIN_PLACE, simConfig1.thresholdSet.LONG_PLACE, simConfig1.thresholdSet.MAX_SIZE));

        // ---- Step 10+11: 创建 PairwiseArbStrategy ----
        // C++: Strategy = new PairwiseArbStrategy(client, simConfig)
        // C++: 构造函数中调用 LoadMatrix2 加载 daily_init
        dailyInitPath = dataDir + "/daily_init." + strategyID;
        strategy = new PairwiseArbStrategy(client, simConfig1, dailyInitPath);
        simConfig1.executionStrategy = strategy;
        // Overview 页面所需元数据
        strategy.modelFile = controlCfg.modelFile != null ? controlCfg.modelFile : "";
        strategy.strategyType = controlCfg.execStrat != null ? controlCfg.execStrat : "";
        strategy.controlFilePath = controlFile != null ? controlFile : "";
        logger.info("[main] daily_init 已加载: " + dailyInitPath);

        // ---- 设置回调 ----
        // C++: client->Initialize(&MDcallback, &ORScallback, &IndicatorCallBack, &AuctionCallBack, ...)
        // Ref: main.cpp:985
        client.setMDCallback(md -> strategy.mdCallBack(md));
        client.setORSCallback(resp -> strategy.orsCallBack(resp));

        // C++: IndicatorCallBack() — main.cpp:313-369
        // 迁移自: main.cpp:75 — 全局变量
        //   double currPrice=0, targetPrice=0, targetBidPNL[5]={1,1,1,1,1}, targetAskPNL[5]={1,1,1,1,1}
        final double[] indCbCurrPrice = {0.0};
        final double[] indCbTargetPrice = {0.0};
        final double[] indCbTargetBidPNL = {1.0, 1.0, 1.0, 1.0, 1.0}; // main.cpp:489-490
        final double[] indCbTargetAskPNL = {1.0, 1.0, 1.0, 1.0, 1.0}; // main.cpp:489-490
        client.setINDCallback(simCfg -> {
            // 迁移自: main.cpp:313-369 — IndicatorCallBack()
            // C++: if (indicatorlist != NULL && configParams->m_simConfig->m_dateConfig.m_simActive)
            // Ref: main.cpp:315
            if (!simCfg.simActive) return;
            if (simCfg.executionStrategy == null) return;

            // C++: if (strategyType == 1 && m_execStrategy != NULL && useArbStrategy == 1)
            // Ref: main.cpp:364
            if (simCfg.useArbStrat) {
                // useArbStrategy==1 路径: 直接调用 SetTargetValue，不经过 CalculateTargetPNL
                // C++: m_execStrategy->SetTargetValue(currPrice, targetPrice, targetBidPNL, targetAskPNL);
                // Ref: main.cpp:366
                strategy.setTargetValue(0.0, 0.0, indCbTargetBidPNL, indCbTargetAskPNL);
            } else {
                // 非 arb 路径: 通过 CalculateTargetPNL 计算后调用 SetTargetValue
                // C++: if (m_calculatePNL->CalculateTargetPNL())
                //          m_execStrategy->SetTargetValue(currPrice, targetPrice, targetBidPNL, targetAskPNL);
                // Ref: main.cpp:323-361
                if (simCfg.calculatePNL != null) {
                    boolean hasPNL = simCfg.calculatePNL.calculateTargetPNL(
                            indCbCurrPrice, indCbTargetPrice, indCbTargetBidPNL, indCbTargetAskPNL);
                    if (hasPNL) {
                        strategy.setTargetValue(indCbCurrPrice[0], indCbTargetPrice[0],
                                indCbTargetBidPNL, indCbTargetAskPNL);
                    }
                }
            }
        });

        client.setSimConfigs(new SimConfig[]{simConfig1, simConfig2});

        // ---- Step 12: 启动 API Server (对齐 Go api.NewServer + Start) ----
        apiServer = new ApiServer(9201);
        apiServer.start();
        logger.info("[main] API Server 已启动 (port 9201)");

        logger.info("[main] 初始化完成, strategyID=" + strategyID);
    }

    /**
     * 行情回调入口 — 由 Connector 轮询线程调用。
     * 迁移自: main.cpp:247-260 — MDcallback(MarketUpdateNew *up)
     * C++: if (!up->m_endPkt) { ... MDCallBack(up); }
     */
    private void onMarketData(MemorySegment md) {
        // C++: if (!up->m_endPkt)
        // Ref: main.cpp:249
        byte endPkt = (byte) Types.MDD_END_PKT_VH.get(md, Types.MU_DATA_OFFSET);
        if (endPkt != 0) {
            return; // endPkt 标志位为 true，跳过
        }
        client.sendInfraMDUpdate(md);
    }

    /**
     * 回报回调入口 — 由 Connector 轮询线程调用。
     */
    private void onOrderResponse(MemorySegment resp) {
        client.sendInfraORSUpdate(resp);
    }

    /**
     * 启动 Connector 轮询并进入信号循环。
     * 迁移自: tbsrc/main/main.cpp:1098-1103 (connector->StartAsync() + HandleSignals)
     */
    public void start() {
        // ---- 启动 Connector ----
        connector.startAsync();
        logger.info("[main] Connector 已启动，开始接收行情和回报");

        // ---- 启动 SnapshotCollector (对齐 Go ticker 每秒采集) ----
        snapshotCollector = new SnapshotCollector(strategy, apiServer);
        snapshotCollector.start();

        // ---- 策略未激活 (Live 模式) ----
        logger.info("[main] 策略未激活 (Live 模式，等待 SIGUSR1 或 Web 控制台激活)");

        // ---- 信号处理 ----
        // C++: sigfillset + sigwait 循环
        // Java: 使用 sun.misc.Signal
        registerSignalHandlers();

        // ---- 启动 Web 命令消费线程 (对齐 Go: for cmd := range apiServer.CommandChan()) ----
        startCommandConsumer();

        logger.info("[main] 进入主事件循环 (信号 + Web 命令)");

        // 阻塞等待关闭
        try {
            shutdownLatch.await();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }

        shutdown();
    }

    /**
     * 注册 Unix 信号处理器。
     * C++: SIGUSR1=激活, SIGUSR2=重载, SIGTSTP=平仓, SIGINT/SIGTERM=关闭
     */
    @SuppressWarnings("restriction")
    private void registerSignalHandlers() {
        try {
            // SIGUSR1: 激活策略
            // C++: main.cpp:140-148 — 顺序: 重置标志 → active=true → HandleSquareON()
            sun.misc.Signal.handle(new sun.misc.Signal("USR1"), sig -> {
                logger.info("[main] 收到 SIGUSR1，激活策略");
                // C++: Strategy->m_onExit = false; Strategy->m_onCancel = false; Strategy->m_onFlat = false;
                strategy.onExit = false;
                strategy.onCancel = false;
                strategy.onFlat = false;
                // C++: Strategy->m_Active = true;
                strategy.active = true;
                strategy.firstStrat.active = true;
                strategy.secondStrat.active = true;
                // C++: Strategy->HandleSquareON();
                strategy.handleSquareON();
            });

            // SIGUSR2: 热加载阈值
            sun.misc.Signal.handle(new sun.misc.Signal("USR2"), sig -> {
                logger.info("[main] 收到 SIGUSR2，热加载阈值");
                reloadThresholds();
            });

            // SIGTSTP: 平仓
            // C++: main.cpp:132-138 — 顺序: 设置标志 → HandleSquareoff()
            sun.misc.Signal.handle(new sun.misc.Signal("TSTP"), sig -> {
                logger.info("[main] 收到 SIGTSTP，平仓退出");
                // C++: Strategy->m_onExit = true; Strategy->m_onCancel = true; Strategy->m_onFlat = true;
                strategy.onExit = true;
                strategy.onCancel = true;
                strategy.onFlat = true;
                // C++: Strategy->HandleSquareoff();
                strategy.handleSquareoff();
            });

            // SIGINT: 关闭
            sun.misc.Signal.handle(new sun.misc.Signal("INT"), sig -> {
                logger.info("[main] 收到 SIGINT，关闭进程");
                shutdownLatch.countDown();
            });

            // SIGTERM: 关闭
            sun.misc.Signal.handle(new sun.misc.Signal("TERM"), sig -> {
                logger.info("[main] 收到 SIGTERM，关闭进程");
                shutdownLatch.countDown();
            });
        } catch (IllegalArgumentException e) {
            // 某些平台不支持特定信号
            logger.warning("[main] 信号注册失败: " + e.getMessage());
        }
    }

    /**
     * 启动 Web 命令消费线程。
     * 对齐: tbsrc-golang/cmd/trader/main.go 中 for cmd := range apiServer.CommandChan()
     */
    private void startCommandConsumer() {
        Thread cmdThread = new Thread(() -> {
            while (running) {
                try {
                    String cmd = apiServer.commandQueue().poll(1, TimeUnit.SECONDS);
                    if (cmd == null) continue;

                    logger.info("[main] 收到 Web 命令: " + cmd);
                    switch (cmd) {
                        case "activate" -> {
                            // C++: main.cpp:140-148 — 顺序: 重置标志 → active=true → HandleSquareON()
                            strategy.onExit = false;
                            strategy.onCancel = false;
                            strategy.onFlat = false;
                            strategy.active = true;
                            strategy.firstStrat.active = true;
                            strategy.secondStrat.active = true;
                            strategy.handleSquareON();
                            logger.info("[main] 策略已通过 Web 激活");
                        }
                        case "deactivate" -> {
                            strategy.active = false;
                            strategy.firstStrat.active = false;
                            strategy.secondStrat.active = false;
                            logger.info("[main] 策略已通过 Web 停用");
                        }
                        case "squareoff" -> {
                            // C++: main.cpp:132-138 — 顺序: 设置标志 → HandleSquareoff()
                            strategy.onExit = true;
                            strategy.onCancel = true;
                            strategy.onFlat = true;
                            strategy.handleSquareoff();
                            logger.info("[main] 策略已通过 Web 平仓");
                        }
                        case "reload_thresholds" -> {
                            reloadThresholds();
                            logger.info("[main] 阈值已通过 Web 热加载");
                        }
                        default -> logger.warning("[main] 未知 Web 命令: " + cmd);
                    }
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    break;
                }
            }
        }, "cmd-consumer");
        cmdThread.setDaemon(true);
        cmdThread.start();
    }

    /**
     * 热加载阈值。
     * C++: SIGUSR2 触发 LoadThresholds(simConfig)
     */
    private void reloadThresholds() {
        try {
            ControlConfig cc = ControlConfig.loadControlFile(controlFile);
            ModelConfig mc = ModelConfig.loadModelFile(cc.modelFile);
            ConfigParser.loadThresholds(strategy.thold_first, mc.thresholds);
            ConfigParser.loadThresholds(strategy.thold_second, mc.thresholds);
            strategy.setThresholds();
            logger.info("[main] 阈值热加载完成");
        } catch (Exception e) {
            logger.warning("[main] 阈值热加载失败: " + e.getMessage());
        }
    }

    /**
     * 优雅关闭。
     * C++: Squareoff() + connector->Stop()
     */
    public void shutdown() {
        if (!running) return;
        running = false;

        // 1. 停止 SnapshotCollector
        if (snapshotCollector != null) {
            snapshotCollector.stop();
        }

        // 2. 停止 API Server
        if (apiServer != null) {
            apiServer.stop();
        }

        // 3. 策略平仓 + 保存 daily_init
        if (strategy != null) {
            strategy.handleSquareoff();
            logger.info("[main] 策略已停止，daily_init 已保存");
        }

        // 4. 停止 Connector
        if (connector != null) {
            connector.stop();
            logger.info("[main] Connector 已停止");
        }

        // 5. 释放 SHM
        if (connector != null) {
            connector.close();
        }

        logger.info("[main] 系统关闭完成");
    }

    /**
     * main 入口。
     */
    public static void main(String[] args) {
        // 设置日志格式
        System.setProperty("java.util.logging.SimpleFormatter.format",
            "%1$tF %1$tT.%1$tL %4$s %5$s%6$s%n");

        TraderMain trader = new TraderMain();
        try {
            trader.parseArgs(args);
            trader.init();
            trader.start();
        } catch (Exception e) {
            Logger.getLogger(TraderMain.class.getName()).severe("启动失败: " + e.getMessage());
            e.printStackTrace();
            System.exit(1);
        }
    }
}
