package com.quantlink.trader.core;

import java.util.HashMap;
import java.util.Map;

/**
 * 每策略配置容器。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h — struct SimConfig (line 707-747)
 */
public class SimConfig {

    // ---- 合约 ----
    // 迁移自: SimConfig::m_instru, m_instru_sec, m_instru_third
    public Instrument instrument;
    public Instrument instrumentSec;
    public Instrument instrumentThird;

    // ---- Instrument 映射 ----
    // 迁移自: SimConfig::m_instruMap, m_instruList[100]
    // C++: unordered_map<string, InstruElem*> m_instruMap
    // Go: Client.instruments map[string]*instrument.Instrument
    // Java: symbol → Instrument（按 symbol 字符串路由，与 Go 一致）
    public final Map<String, Instrument> instruMap = new HashMap<>();

    // ---- 阈值 ----
    // 迁移自: SimConfig::m_tholdSet
    public final ThresholdSet thresholdSet = new ThresholdSet();

    // ---- 策略关联 ----
    // 迁移自: SimConfig::m_execStrategy
    // C++ 使用 ExecutionStrategy* 指针
    public Object executionStrategy;

    // ---- 策略 ID ----
    // 迁移自: SimConfig::m_strategyID
    public int strategyID;

    // ---- 控制配置 ----
    // 迁移自: SimConfig::m_controlConfig
    public String baseName = "";
    public String modelFile = "";
    public String exchangeName = "";
    public String id = "";
    public String execStrat = "";
    public String startTime = "";
    public String endTime = "";
    public String secondName = "";
    public String thirdName = "";

    // ---- 标志位 ----
    // 迁移自: SimConfig 各布尔字段
    public boolean snapshot = false;
    public boolean perContract = false;
    public boolean useStratBook = false;
    public boolean useArbStrat = false;
    public boolean crossBook = false;

    // ---- 交易费用 ----
    // 迁移自: SimConfig::m_buyExchTx 等
    public double buyExchTx = 0;
    public double sellExchTx = 0;
    public double buyExchContractTx = 0;
    public double sellExchContractTx = 0;

    // ---- 索引 ----
    // 迁移自: SimConfig::m_index
    public int index = 0;
}
