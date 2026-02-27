package com.quantlink.trader.core;

/**
 * 指标基类。
 * 迁移自: tbsrc/Indicators/include/Indicator.h (line 38-96)
 *
 * C++ 中 Indicator 是纯虚基类，所有具体指标（Dependant, BookDelta 等）继承此类。
 * Java 使用抽象类对应。
 */
public abstract class Indicator {

    // ---- 指标值 ----
    // 迁移自: Indicator.h:90-92
    public double value = 0.0;
    public double lastValue = 0.0;
    public double diffValue = 0.0;

    // ---- 状态标志 ----
    // 迁移自: Indicator.h:88-89
    public boolean isValid = false;
    public boolean isDep = false;

    // ---- 关联 ----
    // 迁移自: Indicator.h:84-85, 93-95
    public Instrument instrument;
    public int level = 1;
    public String description = "";
    public double index;
    public int strategyIndex;

    // ---- 纯虚方法 ----
    // 迁移自: Indicator.h:52-55

    /**
     * 行情报价更新（Bid/Ask 变化时调用）。
     * C++: virtual void QuoteUpdate(Tick *t) = 0;
     */
    public abstract void quoteUpdate();

    /**
     * 成交更新（Trade 发生时调用）。
     * C++: virtual void TickUpdate(Tick *t) = 0;
     */
    public abstract void tickUpdate();

    /**
     * 策略订单簿更新（ProcessSelfTrade 调整 StratBook 后调用）。
     * C++: virtual void OrderBookStratUpdate(Tick *t) {};
     */
    public void orderBookStratUpdate() {
        // 默认空实现，子类按需覆盖
    }

    /**
     * 重置指标状态。
     * C++: virtual void Reset() = 0;
     */
    public abstract void reset();

    /**
     * 获取指标值。
     * C++: virtual double Value(bool &status) { status = isValid; return value; }
     * 迁移自: Indicator.h:56-60
     */
    public double getValue() {
        return value;
    }

    /**
     * 获取差值。
     * C++: virtual double diffValue(bool &status) { status = isValid; return diff_value; }
     * 迁移自: Indicator.h:61-65
     */
    public double getDiffValue() {
        return diffValue;
    }

    /**
     * 计算差值 = value - last_value，然后更新 last_value。
     * C++: inline void Calculate() { if (!isValid) value = 0.0; diff_value = value - last_value; last_value = value; }
     * 迁移自: Indicator.h:66-72
     */
    public void calculate() {
        // C++: if (!isValid) value = 0.0;
        if (!isValid) {
            value = 0.0;
        }
        // C++: diff_value = value - last_value;
        diffValue = value - lastValue;
        // C++: last_value = value;
        lastValue = value;
    }
}
