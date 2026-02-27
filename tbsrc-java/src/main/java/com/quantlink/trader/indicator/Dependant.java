package com.quantlink.trader.indicator;

import com.quantlink.trader.core.Indicator;
import com.quantlink.trader.core.Instrument;

/**
 * 因变量指标 — 从 Instrument 订单簿读取价格作为因变量。
 * 迁移自: tbsrc/Indicators/include/Dependant.h + tbsrc/Indicators/Dependant.cpp
 *
 * C++ 中 Dependant 是 IndicatorList 的第一个元素，代表被预测的"当前市价"。
 * 支持多种价格类型：MID_PX, MKTW_PX, WGT_PX, LTP_PX 等。
 */
public class Dependant extends Indicator {

    // 迁移自: Dependant.h:18 — TBPriceType style
    private final int style;

    // 价格类型常量 — 对齐 C++ TBPriceType enum
    // 迁移自: TradeBotUtils.h:TBPriceType
    public static final int MKTW_PX2 = 0;   // MSW 价格
    public static final int MID_PX2 = 1;    // MID 价格
    public static final int MKTMID_PX2 = 2; // MSW-MID 混合
    public static final int WGT_PX = 3;     // 加权价格
    public static final int LTP_PX = 4;     // 最新成交价

    /**
     * 构造 Dependant 指标。
     * 迁移自: Dependant::Dependant(Instrument *Inst, string Style)
     * Ref: Dependant.cpp:10-30
     *
     * @param inst  关联合约
     * @param styleStr 价格类型字符串（如 "MID_PX", "MKTW_PX2"）
     */
    public Dependant(Instrument inst, String styleStr) {
        this.instrument = inst;

        // C++: if (Style.compare("MKTW_PX2") == 0 || Style.compare("MKTW_PX") == 0 || ...)
        // Ref: Dependant.cpp:14-23
        if ("MKTW_PX2".equals(styleStr) || "MKTW_PX".equals(styleStr) || "MKTW_RATIO".equals(styleStr)) {
            style = MKTW_PX2;
        } else if ("MID_PX".equals(styleStr) || "MID_PX2".equals(styleStr) || "MID_RATIO".equals(styleStr)) {
            style = MID_PX2;
        } else if ("MKTMID_PX2".equals(styleStr)) {
            style = MKTMID_PX2;
        } else if ("WGT_PX".equals(styleStr)) {
            style = WGT_PX;
        } else if ("LTP_PX".equals(styleStr)) {
            style = LTP_PX;
        } else {
            style = MID_PX2; // 默认 MID
        }

        // C++: instrument->SubscribeTBPriceType(style);
        // Ref: Dependant.cpp:25
        instrument.subscribeTBPriceType(style);

        // C++: description = "Dependant"; isDep = true;
        description = "Dependant";
        value = 0;
        isValid = false;
        isDep = true;
    }

    /**
     * 获取价格类型。
     */
    public int getStyle() {
        return style;
    }

    // 迁移自: Dependant::Reset() — Dependant.cpp:32-36
    @Override
    public void reset() {
        value = 0;
        isValid = false;
    }

    /**
     * 从 market book 更新价格。
     * 迁移自: Dependant::OrderBookUpdate(Tick *t) — Dependant.cpp:38-58
     */
    public void orderBookUpdate() {
        // C++: isValid = true;
        isValid = true;

        // C++: if (instrument->bidPx[0] == 0 || instrument->askPx[0] == 0) { isValid = false; return; }
        if (instrument.bidPx[0] == 0 || instrument.askPx[0] == 0) {
            isValid = false;
            return;
        }

        // C++: price = instrument->GetTBPriceType(style);
        double price = instrument.getTBPriceType(style);

        // C++: if (price == 0) { isValid = false; return; }
        if (price == 0) {
            isValid = false;
            return;
        }

        // C++: value = price;
        value = price;
    }

    /**
     * 从 strat book 更新价格。
     * 迁移自: Dependant::OrderBookStratUpdate(Tick *t) — Dependant.cpp:61-82
     */
    @Override
    public void orderBookStratUpdate() {
        isValid = true;

        // C++: if (instrument->bidPxStrat[0] == 0 || instrument->askPxStrat[0] == 0)
        if (instrument.bidPxStrat[0] == 0 || instrument.askPxStrat[0] == 0) {
            isValid = false;
            return;
        }

        // C++: price = instrument->GetTBStratPriceType(style);
        double price = instrument.getTBStratPriceType(style);

        if (price == 0) {
            isValid = false;
            return;
        }

        value = price;
    }

    /**
     * 行情报价更新 — L1 时更新订单簿价格。
     * 迁移自: Dependant::QuoteUpdate(Tick *t) — Dependant.cpp:84-88
     * C++: if (t->tickLevel == 1) OrderBookUpdate(t);
     *
     * [C++差异] C++ 用 Tick.tickLevel 判断，Java 中 Dependant 的 level 字段默认为 1。
     * CommonClient.update() 在 L1 更新时调用 quoteUpdate()。
     */
    @Override
    public void quoteUpdate() {
        orderBookUpdate();
    }

    /**
     * 成交更新。
     * 迁移自: Dependant::TickUpdate(Tick *t) — Dependant.cpp:90-93
     * C++: OrderBookUpdate(t);
     */
    @Override
    public void tickUpdate() {
        orderBookUpdate();
    }
}
