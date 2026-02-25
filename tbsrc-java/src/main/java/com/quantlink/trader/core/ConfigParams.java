package com.quantlink.trader.core;

import java.util.HashMap;
import java.util.List;
import java.util.Map;

/**
 * 全局配置管理单例。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h — class ConfigParams (line 615-705)
 *
 * C++ ConfigParams 使用 m_simConfigList[100] (symbolID → SimConfigMapIter)
 *     和 m_orderIDStrategyMap (orderID → ExecutionStrategy*)。
 */
public class ConfigParams {

    // ---- 单例 ----
    // 迁移自: ConfigParams::GetInstance() + m_instance
    private static ConfigParams instance;

    public static ConfigParams getInstance() {
        if (instance == null) {
            instance = new ConfigParams();
        }
        return instance;
    }

    /** 重置单例（测试用） */
    public static void resetInstance() {
        instance = null;
    }

    private ConfigParams() {}

    // ---- 策略 ID ----
    // 迁移自: ConfigParams::m_strategyID
    public int strategyID;

    // ---- 运行模式 ----
    // 迁移自: ConfigParams::m_modeType (ModeType enum)
    public int modeType; // 0=Regress, 1=Sim, 2=Live, 3=LeadLag

    // ---- 策略数量 ----
    // 迁移自: ConfigParams::m_strategyCount
    public int strategyCount = 1;

    // ---- SimConfig 映射 ----
    // 迁移自: ConfigParams::m_simConfigMap + m_simConfigList[100]
    // C++: SimConfigMap m_simConfigMap (string→SimConfigList*)
    //      SimConfigMapIter m_simConfigList[100] (symbolID→iterator)
    // Java: symbolID → List<SimConfig>
    public final Map<Integer, List<SimConfig>> simConfigMap = new HashMap<>();

    // ---- 当前活跃 SimConfig ----
    // 迁移自: ConfigParams::m_simConfig
    public SimConfig simConfig;

    // ---- OrderID → 策略映射 ----
    // 迁移自: ConfigParams::m_orderIDStrategyMap
    // C++: map<uint32_t, ExecutionStrategy*>
    // [C++差异] Java 使用 Object 引用，Phase 3 细化为 ExecutionStrategy 类型
    public final Map<Integer, Object> orderIDStrategyMap = new HashMap<>();

    // ---- 全局配置标志 ----
    // 迁移自: ConfigParams 各布尔/数值字段
    public boolean useExchTS = false;
    public boolean squareOff = false;
    public boolean commonBook = false;
    public boolean selfBook = false;
    public boolean useCombined = false;
    public boolean useEndPkt = false;
    public boolean deltaStrategy = false;
    public int optionStrategy = 0;
    public boolean sweepStrategy = false;

    // ---- 产品标识（eric625） ----
    // 迁移自: ConfigParams::m_product
    public String product = "";

    // ---- 更新间隔 ----
    // 迁移自: ConfigParams::m_updateInterval — 默认120秒
    public long updateInterval = 120_000_000_000L; // 120 seconds in nanos

    // ---- 更新 symbol ----
    // 迁移自: ConfigParams::m_updateSymbol
    public String updateSymbol = "";
}
