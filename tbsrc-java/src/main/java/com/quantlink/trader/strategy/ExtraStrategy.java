package com.quantlink.trader.strategy;

import com.quantlink.trader.core.*;
import com.quantlink.trader.shm.Constants;

import java.lang.foreign.MemorySegment;
import java.util.Map;
import java.util.logging.Logger;

/**
 * 多合约策略变体 — Instrument 参数化订单方法。
 * 迁移自: tbsrc/Strategies/include/ExtraStrategy.h (line 6-29)
 *         tbsrc/Strategies/ExtraStrategy.cpp (399 lines)
 *
 * C++ class ExtraStrategy : public ExecutionStrategy
 * 差异: SendBidOrder/SendAskOrder/SendNewOrder/SendModifyOrder/SendCancelOrder 都
 *       接受 Instrument* 参数，允许在不同合约上操作。
 */
public class ExtraStrategy extends ExecutionStrategy {

    private static final Logger log = Logger.getLogger(ExtraStrategy.class.getName());

    /**
     * 构造函数。
     * 迁移自: ExtraStrategy::ExtraStrategy(CommonClient*, SimConfig*)
     * Ref: ExtraStrategy.cpp:4-6
     */
    public ExtraStrategy(CommonClient client, SimConfig simConfig) {
        super(client, simConfig);
    }

    /**
     * 空实现 — PairwiseArbStrategy 控制发单。
     * 迁移自: ExtraStrategy::SendOrder() { }
     * Ref: ExtraStrategy.cpp:8-10
     */
    @Override
    public void sendOrder() {
        // 空实现
    }

    /**
     * 行情回调 — ExtraStrategy 不需要特殊处理，由 PairwiseArbStrategy 转发。
     * 迁移自: ExtraStrategy::MDCallBack(MarketUpdateNew*)
     * (C++ 中 ExtraStrategy 的 MDCallBack 实际调用基类)
     */
    @Override
    public void mdCallBack(MemorySegment update) {
        super.mdCallBack(update);
    }

    /**
     * 初始化监控策略数据 — 上报当前持仓。
     * 迁移自: ExtraStrategy::InitMonitorStratDatas()
     * Ref: ExtraStrategy.cpp:12-17
     */
    public void initMonitorStratDatas() {
        // C++: SendMonitorStratPos(m_product, m_strategyID, m_instru->m_instrument, m_buyPrice, m_sellPrice,
        //        m_buyAvgPrice, m_sellAvgPrice, m_buyQty, m_sellQty, m_buyTotalQty, m_sellTotalQty, m_netpos_pass);
        sendMonitorStratPos(product, strategyID, instru.origBaseName,
                buyPrice, sellPrice, buyAvgPrice, sellAvgPrice,
                buyQty, sellQty, buyTotalQty, sellTotalQty, netpos_pass);
    }

    // =======================================================================
    //  Self-book 缓存 — AddtoCache
    // =======================================================================

    /**
     * 将订单添加到 self-book 价格缓存（Instrument 参数化版本）。
     * 迁移自: ExtraStrategy::AddtoCache(OrderMapIter &iter, double &price)
     * Ref: ExtraStrategy.cpp:19-31
     *
     * 与基类 ExecutionStrategy::AddtoCache 逻辑完全一致，
     * 但操作的是 ExtraStrategy 自身的 bidMapCache/askMapCache。
     */
    @Override
    public void addToCache(OrderStats order, double price) {
        // C++: if (iter->second->m_side == BUY) priceMapCache = &m_bidMapCache; else priceMapCache = &m_askMapCache;
        // C++: (*priceMapCache)[price] = iter->second;
        if (order.side == com.quantlink.trader.shm.Constants.SIDE_BUY) {
            bidMapCache.put(price, order);
        } else {
            askMapCache.put(price, order);
        }
    }

    // =======================================================================
    //  Instrument 参数化订单方法
    // =======================================================================

    /**
     * 发买单（基于 tholdSize）。
     * 迁移自: ExtraStrategy::SendBidOrder(Instrument*, RequestType, int32_t, double, OrderHitType, int32_t, uint32_t, double)
     * Ref: ExtraStrategy.cpp:33-84
     */
    public void sendBidOrder(Instrument instrument, int level, double price, OrderStats.HitType ordType) {
        sendBidOrder(instrument, level, price, ordType, 0);
    }

    public void sendBidOrder(Instrument instrument, int level, double price, OrderStats.HitType ordType, int quantity) {
        int qty = quantity > 0 ? quantity : tholdSize;
        if (qty > 0 && price > 0) {
            Instrument saved = instru;
            instru = instrument;
            sendNewOrder(Constants.SIDE_BUY, price, qty, level, ordType);
            instru = saved;
        }
    }

    /**
     * 发卖单（基于 tholdSize）。
     * 迁移自: ExtraStrategy::SendAskOrder(Instrument*, ...)
     * Ref: ExtraStrategy.cpp:86-136
     */
    public void sendAskOrder(Instrument instrument, int level, double price, OrderStats.HitType ordType) {
        sendAskOrder(instrument, level, price, ordType, 0);
    }

    public void sendAskOrder(Instrument instrument, int level, double price, OrderStats.HitType ordType, int quantity) {
        int qty = quantity > 0 ? quantity : tholdSize;
        if (qty > 0 && price > 0) {
            Instrument saved = instru;
            instru = instrument;
            sendNewOrder(Constants.SIDE_SELL, price, qty, level, ordType);
            instru = saved;
        }
    }

    /**
     * 发买单（基于 tholdBidSize），返回是否成功。
     * 迁移自: ExtraStrategy::SendBidOrder2(Instrument*, ...)
     * Ref: ExtraStrategy.cpp:139-168
     */
    public boolean sendBidOrder2(Instrument instrument, int level, double price, OrderStats.HitType ordType, int quantity) {
        int qty = quantity > 0 ? quantity : tholdBidSize;
        if (qty > 0) {
            if (price <= 0) return false;
            Instrument saved = instru;
            instru = instrument;
            OrderStats result = sendNewOrder(Constants.SIDE_BUY, price, qty, level, ordType);
            instru = saved;
            return result != null;
        }
        return false;
    }

    /**
     * 发卖单（基于 tholdAskSize），返回是否成功。
     * 迁移自: ExtraStrategy::SendAskOrder2(Instrument*, ...)
     * Ref: ExtraStrategy.cpp:170-199
     */
    public boolean sendAskOrder2(Instrument instrument, int level, double price, OrderStats.HitType ordType, int quantity) {
        int qty = quantity > 0 ? quantity : tholdAskSize;
        if (qty > 0) {
            if (price <= 0) return false;
            Instrument saved = instru;
            instru = instrument;
            OrderStats result = sendNewOrder(Constants.SIDE_SELL, price, qty, level, ordType);
            instru = saved;
            return result != null;
        }
        return false;
    }

    /**
     * Instrument 参数化新订单。
     * 迁移自: ExtraStrategy::SendNewOrder(TransactionType, double, int32_t, int32_t, Instrument*, TypeOfOrder, OrderHitType)
     * Ref: ExtraStrategy.cpp:201-278
     */
    public OrderStats sendNewOrder(byte side, double price, int qty, int orderLevel, Instrument instrument, OrderStats.HitType ordtype) {
        Instrument saved = instru;
        instru = instrument;
        OrderStats result = sendNewOrder(side, price, qty, orderLevel, ordtype);
        instru = saved;
        return result;
    }

    /**
     * Instrument 参数化改单。
     * 迁移自: ExtraStrategy::SendModifyOrder(Instrument*, uint32_t, ...)
     * Ref: ExtraStrategy.cpp:280-373
     */
    public OrderStats sendModifyOrder(Instrument instrument, int orderID, double price, double oldPx, int qty, int orderLevel, OrderStats.HitType ordtype) {
        Instrument saved = instru;
        instru = instrument;
        OrderStats result = sendModifyOrder(orderID, price, oldPx, qty, orderLevel, ordtype);
        instru = saved;
        return result;
    }

    /**
     * Instrument 参数化撤单（按 orderID）。
     * 迁移自: ExtraStrategy::SendCancelOrder(Instrument*, uint32_t)
     * Ref: ExtraStrategy.cpp:401-441
     */
    public boolean sendCancelOrder(Instrument instrument, int orderID) {
        Instrument saved = instru;
        instru = instrument;
        boolean result = sendCancelOrder(orderID);
        instru = saved;
        return result;
    }

    /**
     * Instrument 参数化撤单（按价格+方向）。
     * 迁移自: ExtraStrategy::SendCancelOrder(Instrument*, double, TransactionType)
     * Ref: ExtraStrategy.cpp:375-399
     */
    public boolean sendCancelOrder(Instrument instrument, double price, byte side) {
        Instrument saved = instru;
        instru = instrument;
        boolean result = sendCancelOrder(price, side);
        instru = saved;
        return result;
    }

    /**
     * Instrument 参数化平仓。
     * 迁移自: ExtraStrategy::HandleSquareoff(Instrument*)
     * Ref: ExtraStrategy.h:25
     */
    public void handleSquareoff(Instrument instrument) {
        Instrument saved = instru;
        instru = instrument;
        handleSquareoff();
        instru = saved;
    }
}
