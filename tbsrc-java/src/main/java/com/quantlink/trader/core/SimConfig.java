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
    // C++: unordered_map<string, InstruElem*> m_instruMap — 按 symbol 字符串路由
    // Ref: CommonClient.cpp:437
    public final Map<String, Instrument> instruMap = new HashMap<>();

    // ---- Instrument 数组 (symbolID 索引) ----
    // 迁移自: SimConfig::m_instruList[100]
    // C++: InstruMapIter m_instruList[100] — symbolID → InstruMap 迭代器
    // Ref: CommonClient.cpp:437 — iter = m_configParams->m_simConfig->m_instruList[update->m_symbolID]
    // symbolID 由 Connector 排序 symbol 后分配 (0,1,2...)
    public Instrument[] instruList;

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

    // ---- DateConfig ----
    // 迁移自: SimConfig::m_dateConfig.m_simActive
    // C++: DateConfig m_dateConfig — 包含交易时间控制
    // C++: bool m_simActive — 当前是否在交易时段内
    // 在 SendINDUpdate 中检查: if (m_simConfig->m_dateConfig.m_simActive)
    public boolean simActive = true;

    // ---- StratBook 控制 ----
    // 迁移自: SimConfig::m_bUseStratBook
    // C++: 在 SendINDUpdate 中: m_bUseStratBook = false 当策略主合约收到行情时
    // 区分于 ConfigParams.bUseStratBook (全局配置), 此为 per-simConfig 运行时状态
    public boolean bUseStratBookRuntime = false;

    // ---- 交易费用 ----
    // 迁移自: SimConfig::m_buyExchTx 等
    public double buyExchTx = 0;
    public double sellExchTx = 0;
    public double buyExchContractTx = 0;
    public double sellExchContractTx = 0;

    // ---- 索引 ----
    // 迁移自: SimConfig::m_index
    public int index = 0;

    // ---- 最后遍历的合约引用 ----
    // 迁移自: SimConfig::m_lastInstruMapIter — 用于 CrossBook endPkt 判定
    // C++: 在 SendINDUpdate 循环中更新此迭代器
    public Instrument lastInstruMapInstrument;

    /**
     * 获取最后遍历合约的 crossUpdate 标志。
     * 迁移自: m_simConfig->m_lastInstruMapIter->second->m_instrument->m_crossUpdate
     * Ref: ExecutionStrategy.cpp:469 — CrossBookEnd 条件
     */
    public boolean lastCrossUpdate() {
        return lastInstruMapInstrument != null && lastInstruMapInstrument.crossUpdate;
    }
}
