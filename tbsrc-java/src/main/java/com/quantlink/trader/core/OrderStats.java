package com.quantlink.trader.core;

import com.quantlink.trader.shm.Constants;

/**
 * 订单生命周期状态追踪。
 * 迁移自: tbsrc/Strategies/include/ExecutionStrategyStructs.h
 *
 * C++ OrderStats struct + OrderStatus enum + OrderHitType enum
 */
public class OrderStats {

    // ---- 枚举: 订单状态 ----
    // 迁移自: ExecutionStrategyStructs.h — enum OrderStatus
    public enum Status {
        NEW_ORDER,
        NEW_CONFIRM,
        NEW_REJECT,
        MODIFY_ORDER,
        MODIFY_CONFIRM,
        MODIFY_REJECT,
        CANCEL_ORDER,
        CANCEL_CONFIRM,
        CANCEL_REJECT,
        TRADED,
        INIT
    }

    // ---- 枚举: 订单命中类型 ----
    // 迁移自: ExecutionStrategyStructs.h — enum OrderHitType
    public enum HitType {
        STANDARD,
        IMPROVE,
        CROSS,
        DETECT,
        MATCH
    }

    // ---- 字段 ----
    // 迁移自: ExecutionStrategyStructs.h — struct OrderStats 全部字段

    public boolean active;
    public boolean isNew;          // C++: m_new
    public boolean modifyWait;     // C++: m_modifywait
    public boolean cancel;         // C++: m_cancel
    public int modifyCount;        // C++: m_modify
    public long lastTS;            // C++: m_lastTS (uint64_t)
    public int orderID;            // C++: m_orderID (uint32_t → Java int)
    public int oldQty;             // C++: m_oldQty
    public int newQty;             // C++: m_newQty
    public int qty;                // C++: m_Qty
    public int openQty;            // C++: m_openQty
    public int cxlQty;             // C++: m_cxlQty
    public int doneQty;            // C++: m_doneQty
    public double quantAhead;      // C++: m_quantAhead
    public double quantBehind;     // C++: m_quantBehind
    public double price;           // C++: m_price
    public double newPrice;        // C++: m_newprice
    public double oldPrice;        // C++: m_oldprice
    public int typeOfOrder;        // C++: m_typeOfOrder (TypeOfOrder enum) — 使用 Constants 值
    public HitType hitType;        // C++: m_ordType (OrderHitType)
    public Status status;          // C++: m_status (OrderStatus)
    public byte side;              // C++: m_side (TransactionType) — Constants.SIDE_BUY/SELL

    public OrderStats() {
        this.status = Status.INIT;
        this.hitType = HitType.STANDARD;
        this.side = Constants.SIDE_BUY;
    }

    @Override
    public String toString() {
        return String.format("Order[id=%d, px=%.2f, qty=%d, open=%d, done=%d, status=%s, hit=%s, side=%s]",
                orderID, price, qty, openQty, doneQty, status,
                hitType, side == Constants.SIDE_BUY ? "BUY" : "SELL");
    }
}
