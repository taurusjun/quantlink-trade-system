package com.quantlink.trader;

import com.quantlink.trader.config.*;
import com.quantlink.trader.connector.Connector;
import com.quantlink.trader.core.*;
import com.quantlink.trader.strategy.PairwiseArbStrategy;

import java.lang.foreign.MemorySegment;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.CountDownLatch;
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

        // ---- Step 1: 加载 controlFile ----
        // C++: LoadControlFile(simConfig[0].m_controlConfig, controlFile)
        ControlConfig controlCfg = ControlConfig.parse(controlFile);
        logger.info(String.format("[config] controlFile: baseName=%s model=%s exchange=%s strat=%s second=%s",
            controlCfg.baseName, controlCfg.modelFile, controlCfg.exchange,
            controlCfg.execStrat, controlCfg.secondName));

        // ---- Step 2: 加载 .cfg ----
        // C++: illuminati::Configfile::LoadCfg(configParams->m_configFile)
        CfgConfig cfgConfig = CfgConfig.parse(configFile);
        logger.info(String.format("[config] configFile: product=%s exchanges=%s",
            cfgConfig.product, cfgConfig.exchanges));

        // ---- Step 3: 加载 model file ----
        // C++: LoadModelFile(simConfig[i], tholdMap)
        ModelConfig modelCfg = ModelConfig.parse(controlCfg.modelFile);
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
        // C++: new Connector(MDconnection, ORSconnection, connectorCfg)
        Connector.Config connCfg = new Connector.Config();
        connCfg.mdShmKey = mdKey;
        connCfg.mdQueueSize = mdSize;
        connCfg.reqShmKey = reqKey;
        connCfg.reqQueueSize = reqSize;
        connCfg.respShmKey = respKey;
        connCfg.respQueueSize = respSize;
        connCfg.clientStoreShmKey = clientStoreKey;

        connector = Connector.open(connCfg,
            this::onMarketData,
            this::onOrderResponse
        );
        logger.info("[main] Connector 已创建, clientId=" + connector.getClientId());

        // ---- Step 7: 创建 CommonClient ----
        // C++: client->Initialize(&MDcallback, &ORScallback, ...)
        client = new CommonClient();
        client.setConnector(connector);

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
        instru1.origBaseName = controlCfg.baseName;
        instru1.tickSize = tickSize;
        instru1.lotSize = lotSize;
        instru1.priceMultiplier = lotSize;
        instru1.priceFactor = 1.0;
        instru1.sendInLots = true;

        Instrument instru2 = new Instrument();
        instru2.symbol = sym2;
        instru2.origBaseName = controlCfg.secondName;
        instru2.tickSize = tickSize;
        instru2.lotSize = lotSize;
        instru2.priceMultiplier = lotSize;
        instru2.priceFactor = 1.0;
        instru2.sendInLots = true;

        logger.info(String.format("[main] 合约创建: %s (tick=%.1f lot=%.0f) %s (tick=%.1f lot=%.0f)",
            sym1, tickSize, lotSize, sym2, tickSize, lotSize));

        // 注册 symbolID → SimConfig 映射
        SimConfig simConfig1 = new SimConfig();
        simConfig1.instrument = instru1;
        simConfig1.instrumentSec = instru2;
        simConfig1.useArbStrat = true;
        simConfig1.strategyID = strategyID;

        // C++: 两个 symbolID 都注册到同一个 SimConfig
        int symId1 = instru1.symbol.hashCode() & 0x7FFFFFFF;
        int symId2 = instru2.symbol.hashCode() & 0x7FFFFFFF;
        instru1.symbolID = symId1;
        instru2.symbolID = symId2;

        List<SimConfig> simList1 = new ArrayList<>();
        simList1.add(simConfig1);
        params.simConfigMap.put(symId1, simList1);

        SimConfig simConfig2 = new SimConfig();
        simConfig2.instrument = instru2;
        simConfig2.instrumentSec = instru1;
        simConfig2.useArbStrat = true;
        simConfig2.strategyID = strategyID;

        List<SimConfig> simList2 = new ArrayList<>();
        simList2.add(simConfig2);
        params.simConfigMap.put(symId2, simList2);

        // instruMap: symbolID → Instrument
        simConfig1.instruMap.put(symId1, instru1);
        simConfig1.instruMap.put(symId2, instru2);
        simConfig2.instruMap.put(symId1, instru1);
        simConfig2.instruMap.put(symId2, instru2);

        // ---- Step 9: 加载阈值 ----
        // C++: LoadModelFile → ThresholdSet
        // SimConfig.thresholdSet is final — 直接加载到现有实例
        ConfigParser.loadThresholds(simConfig1.thresholdSet, modelCfg.thresholds);
        ConfigParser.loadThresholds(simConfig2.thresholdSet, modelCfg.thresholds);

        logger.info(String.format("[main] 阈值: BEGIN_PLACE=%.4f LONG_PLACE=%.4f MAX_SIZE=%d",
            simConfig1.thresholdSet.BEGIN_PLACE, simConfig1.thresholdSet.LONG_PLACE, simConfig1.thresholdSet.MAX_SIZE));

        // ---- Step 10: 创建 PairwiseArbStrategy ----
        // C++: Strategy = new PairwiseArbStrategy(client, simConfig)
        strategy = new PairwiseArbStrategy(client, simConfig1);
        simConfig1.executionStrategy = strategy;

        // ---- Step 11: 加载 daily_init ----
        // C++: PairwiseArbStrategy 构造函数中调用 LoadMatrix2
        dailyInitPath = dataDir + "/daily_init." + strategyID;
        strategy.loadDailyInit(dailyInitPath);
        logger.info("[main] daily_init 已加载: " + dailyInitPath);

        // ---- 设置回调 ----
        // C++: MDcallback → strategy.MDCallBack()
        // C++: ORScallback → strategy.ORSCallBack()
        client.setMDCallback(md -> strategy.mdCallBack(md));
        client.setORSCallback(resp -> strategy.orsCallBack(resp));
        client.setSimConfigs(new SimConfig[]{simConfig1, simConfig2});

        logger.info("[main] 初始化完成, strategyID=" + strategyID);
    }

    /**
     * 行情回调入口 — 由 Connector 轮询线程调用。
     */
    private void onMarketData(MemorySegment md) {
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
        connector.start();
        logger.info("[main] Connector 已启动，开始接收行情和回报");

        // ---- 策略未激活 (Live 模式) ----
        logger.info("[main] 策略未激活 (Live 模式，等待 SIGUSR1 激活)");

        // ---- 信号处理 ----
        // C++: sigfillset + sigwait 循环
        // Java: 使用 sun.misc.Signal
        registerSignalHandlers();

        logger.info("[main] 进入主事件循环 (信号)");

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
            sun.misc.Signal.handle(new sun.misc.Signal("USR1"), sig -> {
                logger.info("[main] 收到 SIGUSR1，激活策略");
                strategy.active = true;
                strategy.firstStrat.active = true;
                strategy.secondStrat.active = true;
            });

            // SIGUSR2: 热加载阈值
            sun.misc.Signal.handle(new sun.misc.Signal("USR2"), sig -> {
                logger.info("[main] 收到 SIGUSR2，热加载阈值");
                reloadThresholds();
            });

            // SIGTSTP: 平仓
            sun.misc.Signal.handle(new sun.misc.Signal("TSTP"), sig -> {
                logger.info("[main] 收到 SIGTSTP，平仓退出");
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
     * 热加载阈值。
     * C++: SIGUSR2 触发 LoadThresholds(simConfig)
     */
    private void reloadThresholds() {
        try {
            ControlConfig cc = ControlConfig.parse(controlFile);
            ModelConfig mc = ModelConfig.parse(cc.modelFile);
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

        // 1. 策略平仓 + 保存 daily_init
        if (strategy != null) {
            strategy.handleSquareoff();
            logger.info("[main] 策略已停止，daily_init 已保存");
        }

        // 2. 停止 Connector
        if (connector != null) {
            connector.stop();
            logger.info("[main] Connector 已停止");
        }

        // 3. 释放 SHM
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
