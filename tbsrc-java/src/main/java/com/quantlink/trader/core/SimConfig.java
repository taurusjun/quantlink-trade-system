package com.quantlink.trader.core;

import java.time.LocalDate;
import java.time.ZoneId;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.logging.Logger;

/**
 * 每策略配置容器。
 * 迁移自: tbsrc/main/include/TradeBotUtils.h — struct SimConfig (line 707-747)
 */
public class SimConfig {

    private static final Logger log = Logger.getLogger(SimConfig.class.getName());

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
    // 迁移自: TradeBotUtils.h:557-600 — struct DateConfig
    // C++: DateConfig m_dateConfig — 包含交易时间控制
    // C++: bool m_simActive — 当前是否在交易时段内 (默认 false, Reset() 设为 false)
    // C++: uint64_t m_startTimeEpoch, m_endTimeEpoch — 交易时段起止 epoch
    // C++: int32_t m_startTime, m_endTime — 原始时间配置值
    // 在 SendINDUpdate 中调用 UpdateActive(), 检查: if (m_simConfig->m_dateConfig.m_simActive)
    public boolean simActive = false;
    public long startTimeEpoch = 0;
    public long endTimeEpoch = Long.MAX_VALUE;

    // 迁移自: TradeBotUtils.h:559 — char m_currDate[9]
    // C++: Live 模式下在进程启动时设置为当天日期 (YYYYMMDD)，此后不变。
    // 用于日志格式化和文件操作（夜盘跨日后 LocalDate.now() 会变，但 currDate 不变）。
    // Ref: TradeBotUtils.cpp:2534 — strcpy(dateConfig.m_currDate, buf)
    public String currDate = "";

    /**
     * 更新交易时段状态。
     * 迁移自: DateConfig::UpdateActive(uint64_t exchTS)
     * Ref: TradeBotUtils.h:587-592
     *
     * C++: bool UpdateActive(uint64_t exchTS) {
     *          m_simActive = IsActive(exchTS);
     *          return m_simActive;
     *      }
     * C++: bool IsActive(uint64_t exchTS) {
     *          if (exchTS < m_startTimeEpoch || exchTS >= m_endTimeEpoch) return false;
     *          return true;
     *      }
     *
     * @param currentTime Watch 当前时间 (纳秒 epoch)
     * @return 是否在交易时段内
     */
    public boolean updateActive(long currentTime) {
        // C++: m_simActive = IsActive(exchTS)
        simActive = currentTime >= startTimeEpoch && currentTime < endTimeEpoch;
        return simActive;
    }

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

    // ---- 指标列表 ----
    // 迁移自: SimConfig::m_indicatorList — IndicatorList (vector<IndElem*>)
    // Ref: TradeBotUtils.h:737 — IndicatorList m_indicatorList
    public List<IndElem> indicatorList;

    // ---- PNL 计算引擎 ----
    // 迁移自: SimConfig::m_calculatePNL — CalculatePNL*
    // Ref: TradeBotUtils.h:736 — CalculatePNL *m_calculatePNL
    public CalculateTargetPNL calculatePNL;

    /**
     * 获取最后遍历合约的 crossUpdate 标志。
     * 迁移自: m_simConfig->m_lastInstruMapIter->second->m_instrument->m_crossUpdate
     * Ref: ExecutionStrategy.cpp:469 — CrossBookEnd 条件
     */
    public boolean lastCrossUpdate() {
        return lastInstruMapInstrument != null && lastInstruMapInstrument.crossUpdate;
    }

    /**
     * 初始化 DateConfig 时间 epoch 值。
     * 迁移自: TradeBotUtils.cpp:2568-2588 — LoadDateConfigEpoch()
     *
     * C++: dateConfig.m_startTimeEpoch = Watch::GetNanoSecsFromEpoch(currDate, startTime)
     *      dateConfig.m_endTimeEpoch = Watch::GetNanoSecsFromEpoch(currDate, endTime)
     *
     * 从 HHMM 格式的 startTime/endTime 字符串计算 epoch 纳秒值。
     * 夜盘跨日场景 (startTime > endTime): endTimeEpoch 使用下一日基准。
     */
    public void initDateConfigEpoch() {
        // C++: strcpy(dateConfig.m_currDate, buf)  — 进程启动时的日期，后续不变
        // Ref: TradeBotUtils.cpp:2534
        if (currDate.isEmpty()) {
            currDate = LocalDate.now().format(java.time.format.DateTimeFormatter.ofPattern("yyyyMMdd"));
        }

        if (startTime == null || startTime.isEmpty()
                || endTime == null || endTime.isEmpty()) {
            // 未配置 — 使用默认值 (0 ~ Long.MAX_VALUE)，simActive 始终为 true
            startTimeEpoch = 0;
            endTimeEpoch = Long.MAX_VALUE;
            simActive = true;
            log.info("[DateConfig] 交易时间未配置，simActive 默认为 true (currDate=" + currDate + ")");
            return;
        }
        try {
            int startHHMM = Integer.parseInt(startTime.trim());
            int endHHMM = Integer.parseInt(endTime.trim());

            // C++: dateConfig.m_startTime = ((h*3600) + (m*60)) * 1000  → 毫秒自午夜
            long startMs = ((long)(startHHMM / 100) * 3600 + (long)(startHHMM % 100) * 60) * 1000;
            long endMs = ((long)(endHHMM / 100) * 3600 + (long)(endHHMM % 100) * 60) * 1000;

            LocalDate baseDate = LocalDate.now();
            long midnightNanos = baseDate
                .atStartOfDay(ZoneId.of("Asia/Shanghai"))
                .toInstant()
                .toEpochMilli() * 1_000_000L;

            startTimeEpoch = midnightNanos + startMs * 1_000_000L;

            // 跨日判断
            if (endHHMM < startHHMM) {
                // 夜盘跨日 (e.g. start=2100 end=0230)
                long tomorrowMidnightNanos = baseDate.plusDays(1)
                    .atStartOfDay(ZoneId.of("Asia/Shanghai"))
                    .toInstant()
                    .toEpochMilli() * 1_000_000L;
                endTimeEpoch = tomorrowMidnightNanos + endMs * 1_000_000L;
            } else {
                endTimeEpoch = midnightNanos + endMs * 1_000_000L;
            }

            log.info(String.format("[DateConfig] startTime=%s endTime=%s → startEpoch=%d endEpoch=%d",
                startTime, endTime, startTimeEpoch, endTimeEpoch));
        } catch (NumberFormatException e) {
            startTimeEpoch = 0;
            endTimeEpoch = Long.MAX_VALUE;
            simActive = true;
            log.warning("[DateConfig] 时间格式错误: start=" + startTime + " end=" + endTime);
        }
    }
}
