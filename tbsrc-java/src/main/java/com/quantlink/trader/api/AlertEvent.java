package com.quantlink.trader.api;

import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * 策略告警事件。
 * 由 AlertCollector 收集，通过 DashboardSnapshot.alerts 推送到前端。
 */
public class AlertEvent {

    @JsonProperty("timestamp")    public long timestamp;       // System.currentTimeMillis()
    @JsonProperty("level")        public String level;         // "WARNING" | "CRITICAL"
    @JsonProperty("type")         public String type;          // UPNL_LOSS, MAX_LOSS, AVG_SPREAD_AWAY, ...
    @JsonProperty("message")      public String message;       // 人可读描述
    @JsonProperty("symbol")       public String symbol;        // 触发合约
    @JsonProperty("strategy_id")  public int strategyId;       // 策略 ID

    public AlertEvent() {}

    public AlertEvent(String level, String type, String message, String symbol, int strategyId) {
        this.timestamp = System.currentTimeMillis();
        this.level = level;
        this.type = type;
        this.message = message;
        this.symbol = symbol;
        this.strategyId = strategyId;
    }

    // 告警类型常量
    public static final String TYPE_UPNL_LOSS = "UPNL_LOSS";
    public static final String TYPE_STOP_LOSS = "STOP_LOSS";
    public static final String TYPE_MAX_LOSS = "MAX_LOSS";
    public static final String TYPE_MAX_ORDERS = "MAX_ORDERS";
    public static final String TYPE_MAX_TRADED = "MAX_TRADED";
    public static final String TYPE_END_TIME = "END_TIME";
    public static final String TYPE_END_TIME_AGG = "END_TIME_AGG";
    public static final String TYPE_AVG_SPREAD_AWAY = "AVG_SPREAD_AWAY";
    public static final String TYPE_REJECT_LIMIT = "REJECT_LIMIT";

    // 告警级别常量
    public static final String LEVEL_WARNING = "WARNING";
    public static final String LEVEL_CRITICAL = "CRITICAL";
}
