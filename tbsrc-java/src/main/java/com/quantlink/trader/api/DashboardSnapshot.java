package com.quantlink.trader.api;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.quantlink.trader.core.Instrument;
import com.quantlink.trader.core.OrderStats;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.strategy.ExtraStrategy;
import com.quantlink.trader.strategy.PairwiseArbStrategy;

import java.time.Instant;
import java.util.*;

/**
 * 每秒推送给前端的完整快照。
 * 对齐: tbsrc-golang/pkg/api/snapshot.go — DashboardSnapshot
 *
 * Jackson 默认使用字段名作为 JSON key。
 * 使用 @JsonProperty 确保 snake_case 输出（与 Go 版 JSON tag 一致）。
 */
public class DashboardSnapshot {

    @JsonProperty("timestamp")      public String timestamp;
    @JsonProperty("strategy_id")    public int strategyID;
    @JsonProperty("active")         public boolean active;
    @JsonProperty("account")        public String account = "";
    @JsonProperty("exposure")       public int exposure;
    @JsonProperty("model_file")     public String modelFile = "";
    @JsonProperty("strategy_type")  public String strategyType = "";
    @JsonProperty("control_file")   public String controlFile = "";
    @JsonProperty("spread")       public SpreadSnapshot spread = new SpreadSnapshot();
    @JsonProperty("leg1")         public LegSnapshot leg1 = new LegSnapshot();
    @JsonProperty("leg2")         public LegSnapshot leg2 = new LegSnapshot();

    /** 价差分析 — 对齐 Go SpreadSnapshot */
    public static class SpreadSnapshot {
        @JsonProperty("current")    public double current;
        @JsonProperty("avg_spread") public double avgSpread;
        @JsonProperty("avg_ori")    public double avgOri;
        @JsonProperty("t_value")    public double tValue;
        @JsonProperty("deviation")  public double deviation;
        @JsonProperty("is_valid")   public boolean isValid;
        @JsonProperty("alpha")      public double alpha;
    }

    /** 单腿完整状态 — 对齐 Go LegSnapshot */
    public static class LegSnapshot {
        @JsonProperty("symbol")   public String symbol = "";
        @JsonProperty("exchange") public String exchange = "";
        // 行情
        @JsonProperty("bid_px")        public double bidPx;
        @JsonProperty("ask_px")        public double askPx;
        @JsonProperty("mid_px")        public double midPx;
        @JsonProperty("bid_qty")       public double bidQty;
        @JsonProperty("ask_qty")       public double askQty;
        @JsonProperty("last_trade_px") public double lastTradePx;
        // 持仓
        @JsonProperty("netpos")      public int netpos;
        @JsonProperty("netpos_pass") public int netposPass;
        @JsonProperty("netpos_agg")  public int netposAgg;
        // PNL
        @JsonProperty("realised_pnl")   public double realisedPNL;
        @JsonProperty("unrealised_pnl") public double unrealisedPNL;
        @JsonProperty("net_pnl")        public double netPNL;
        @JsonProperty("gross_pnl")      public double grossPNL;
        @JsonProperty("max_pnl")        public double maxPNL;
        @JsonProperty("drawdown")       public double drawdown;
        // 交易统计
        @JsonProperty("trade_count")    public int tradeCount;
        @JsonProperty("order_count")    public int orderCount;
        @JsonProperty("reject_count")   public int rejectCount;
        @JsonProperty("cancel_count")   public int cancelCount;
        @JsonProperty("buy_total_qty")  public double buyTotalQty;
        @JsonProperty("sell_total_qty") public double sellTotalQty;
        // 动态阈值
        @JsonProperty("thold_bid_place")  public double tholdBidPlace;
        @JsonProperty("thold_bid_remove") public double tholdBidRemove;
        @JsonProperty("thold_ask_place")  public double tholdAskPlace;
        @JsonProperty("thold_ask_remove") public double tholdAskRemove;
        @JsonProperty("thold_max_pos")    public int tholdMaxPos;
        @JsonProperty("thold_size")       public int tholdSize;
        // 挂单
        @JsonProperty("buy_open_orders")  public int buyOpenOrders;
        @JsonProperty("sell_open_orders") public int sellOpenOrders;
        @JsonProperty("buy_open_qty")     public double buyOpenQty;
        @JsonProperty("sell_open_qty")    public double sellOpenQty;
        // 状态标志
        @JsonProperty("on_exit")      public boolean onExit;
        @JsonProperty("on_flat")      public boolean onFlat;
        @JsonProperty("on_stop_loss") public boolean onStopLoss;
        // 订单列表
        @JsonProperty("orders") public List<OrderSnapshot> orders = new ArrayList<>();
    }

    /** 单个挂单 — 对齐 Go OrderSnapshot */
    public static class OrderSnapshot {
        @JsonProperty("order_id") public int orderID;
        @JsonProperty("side")     public String side;
        @JsonProperty("price")    public double price;
        @JsonProperty("open_qty") public int openQty;
        @JsonProperty("done_qty") public int doneQty;
        @JsonProperty("status")   public String status;
        @JsonProperty("ord_type") public String ordType;
        @JsonProperty("time")     public String time;
    }

    // =======================================================================
    //  快照采集 — 对齐 Go CollectSnapshot()
    // =======================================================================

    /**
     * 从 PairwiseArbStrategy 采集完整快照。
     * 对齐: tbsrc-golang/pkg/api/snapshot.go:CollectSnapshot()
     */
    public static DashboardSnapshot collect(PairwiseArbStrategy pas) {
        DashboardSnapshot snap = new DashboardSnapshot();
        snap.timestamp = Instant.now().toString();
        snap.strategyID = pas.strategyID;
        snap.active = pas.active;
        snap.exposure = pas.firstStrat.netpos + pas.secondStrat.netpos;
        snap.modelFile = pas.modelFile;
        snap.strategyType = pas.strategyType;
        snap.controlFile = pas.controlFilePath;

        // 价差
        snap.spread.current = pas.currSpreadRatio;
        snap.spread.avgSpread = pas.avgSpreadRatio;
        snap.spread.avgOri = pas.avgSpreadRatio_ori;
        snap.spread.tValue = pas.tValue;
        snap.spread.deviation = pas.currSpreadRatio - pas.avgSpreadRatio;
        snap.spread.isValid = pas.is_valid_mkdata;
        snap.spread.alpha = pas.thold_first != null ? pas.thold_first.ALPHA : 0;

        // Leg1
        snap.leg1 = collectLeg(pas.firstinstru, pas.firstStrat, pas.ordMap1);
        // Leg2
        snap.leg2 = collectLeg(pas.secondinstru, pas.secondStrat, pas.ordMap2);

        return snap;
    }

    private static LegSnapshot collectLeg(Instrument inst, ExtraStrategy leg,
                                           Map<Integer, OrderStats> ordMap) {
        LegSnapshot ls = new LegSnapshot();
        if (inst == null || leg == null) return ls;

        ls.symbol = inst.symbol != null ? inst.symbol : "";
        ls.exchange = "";
        // 行情
        ls.bidPx = inst.bidPx[0];
        ls.askPx = inst.askPx[0];
        ls.midPx = inst.calculateMIDPrice();
        ls.bidQty = inst.bidQty[0];
        ls.askQty = inst.askQty[0];
        ls.lastTradePx = inst.lastTradePx;
        // 持仓
        ls.netpos = leg.netpos;
        ls.netposPass = leg.netpos_pass;
        ls.netposAgg = leg.netpos_agg;
        // PNL
        ls.realisedPNL = leg.realisedPNL;
        ls.unrealisedPNL = leg.unrealisedPNL;
        ls.netPNL = leg.netPNL;
        ls.grossPNL = leg.grossPNL;
        ls.maxPNL = leg.maxPNL;
        ls.drawdown = leg.drawdown;
        // 交易统计
        ls.tradeCount = leg.tradeCount;
        ls.orderCount = leg.orderCount;
        ls.rejectCount = leg.rejectCount;
        ls.cancelCount = leg.cancelCount;
        ls.buyTotalQty = leg.buyTotalQty;
        ls.sellTotalQty = leg.sellTotalQty;
        // 动态阈值
        ls.tholdBidPlace = leg.tholdBidPlace;
        ls.tholdBidRemove = leg.tholdBidRemove;
        ls.tholdAskPlace = leg.tholdAskPlace;
        ls.tholdAskRemove = leg.tholdAskRemove;
        ls.tholdMaxPos = leg.tholdMaxPos;
        ls.tholdSize = leg.tholdSize;
        // 挂单统计
        if (ordMap != null) {
            for (OrderStats ord : ordMap.values()) {
                if (ord.side == Constants.SIDE_BUY) {
                    ls.buyOpenOrders++;
                    ls.buyOpenQty += ord.openQty;
                } else {
                    ls.sellOpenOrders++;
                    ls.sellOpenQty += ord.openQty;
                }
            }
        }
        // 状态标志
        ls.onExit = leg.onExit;
        ls.onFlat = leg.onFlat;
        ls.onStopLoss = leg.onStopLoss;
        // 订单列表
        if (ordMap != null) {
            for (OrderStats ord : ordMap.values()) {
                OrderSnapshot os = new OrderSnapshot();
                os.orderID = ord.orderID;
                os.side = ord.side == Constants.SIDE_BUY ? "BUY" : "SELL";
                os.price = ord.price;
                os.openQty = ord.openQty;
                os.doneQty = ord.doneQty;
                os.status = ord.status != null ? ord.status.name() : "UNKNOWN";
                os.ordType = ord.hitType != null ? ord.hitType.name() : "UNKNOWN";
                os.time = "";
                ls.orders.add(os);
            }
        }
        return ls;
    }
}
