package com.quantlink.trader.api.overview;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.quantlink.trader.api.DashboardSnapshot;

import java.time.Instant;
import java.util.*;

/**
 * 聚合数据模型 — Overview 页面推送给前端的完整数据包。
 *
 * 包含: 策略列表 + 全局持仓 + 全局挂单 + 全局成交(Spread Trades + Fills)
 */
public class OverviewSnapshot {

    @JsonProperty("timestamp")  public String timestamp;
    @JsonProperty("strategies") public List<StrategyRow> strategies = new ArrayList<>();
    @JsonProperty("positions")  public List<PositionRow> positions = new ArrayList<>();
    @JsonProperty("orders")     public List<OrderRow> orders = new ArrayList<>();
    @JsonProperty("fills")      public List<FillRow> fills = new ArrayList<>();
    @JsonProperty("spread_trades") public List<SpreadTradeRow> spreadTrades = new ArrayList<>();

    /** ② 策略列表表格 — 每行一个策略实例 */
    public static class StrategyRow {
        @JsonProperty("status")        public String status = "";        // 运行中/无进程/未连接
        @JsonProperty("alert")         public String alert = "";         // 预留
        @JsonProperty("at")            public boolean at;                // 自动交易开关
        @JsonProperty("pro")           public String pro = "";           // 品种 (ag, cu 等)
        @JsonProperty("id")            public int id;                    // 策略 ID
        @JsonProperty("ip")            public String ip = "";            // 端口号
        @JsonProperty("model_file")    public String modelFile = "";     // 模型文件名
        @JsonProperty("strategy_type") public String strategyType = "";  // TB_PAIR_STRAT 等
        @JsonProperty("key")           public String key = "";           // 预留
        @JsonProperty("val")           public boolean val;               // is_valid
        @JsonProperty("l1")            public int l1;                    // Leg1 持仓
        @JsonProperty("l2")            public int l2;                    // Leg2 持仓
        @JsonProperty("pnl")           public double pnl;               // 总 PNL
        @JsonProperty("time")          public String time = "";          // 最后更新
        @JsonProperty("information")   public String information = "";   // 摘要信息
        @JsonProperty("port")          public int port;                  // 端口号 (用于命令转发)
    }

    /** ⑥ Position Table — 全局持仓 */
    public static class PositionRow {
        @JsonProperty("symbol")   public String symbol = "";
        @JsonProperty("pos")      public int pos;
        @JsonProperty("cxl_rio")  public String cxlRio = "";  // 撤单比
        @JsonProperty("pro")      public String pro = "";
    }

    /** ⑤ Orders — 全局挂单 */
    public static class OrderRow {
        @JsonProperty("symbol")     public String symbol = "";
        @JsonProperty("side")       public String side = "";
        @JsonProperty("qty")        public int qty;
        @JsonProperty("price")      public double price;
        @JsonProperty("model_file") public String modelFile = "";
        @JsonProperty("id")         public int id;
        @JsonProperty("pro")        public String pro = "";
        @JsonProperty("port")       public int port;  // 用于撤单转发
    }

    /** ⑦ Fills — 全局成交 */
    public static class FillRow {
        @JsonProperty("time")   public String time = "";
        @JsonProperty("symbol") public String symbol = "";
        @JsonProperty("side")   public String side = "";
        @JsonProperty("price")  public double price;
        @JsonProperty("qty")    public int qty;
        @JsonProperty("id")     public int id;
        @JsonProperty("pro")    public String pro = "";
    }

    /** ④ Spread Trades — 价差成交 */
    public static class SpreadTradeRow {
        @JsonProperty("model_file") public String modelFile = "";
        @JsonProperty("side")       public String side = "";
        @JsonProperty("qty")        public int qty;
        @JsonProperty("spread")     public double spread;
        @JsonProperty("time")       public String time = "";
        @JsonProperty("pro")        public String pro = "";
    }

    // =======================================================================
    //  聚合逻辑
    // =======================================================================

    /**
     * 从 StrategyConnector 的数据聚合生成 OverviewSnapshot。
     */
    public static OverviewSnapshot aggregate(
            Map<Integer, DashboardSnapshot> snapshots,
            Map<Integer, StrategyConnector.ConnectionStatus> statuses) {

        OverviewSnapshot overview = new OverviewSnapshot();
        overview.timestamp = Instant.now().toString();

        // 遍历所有端口
        for (var entry : statuses.entrySet()) {
            int port = entry.getKey();
            StrategyConnector.ConnectionStatus connStatus = entry.getValue();
            DashboardSnapshot snap = snapshots.get(port);

            // 只处理已连接或有快照的端口
            if (connStatus == StrategyConnector.ConnectionStatus.NO_PROCESS && snap == null) {
                continue;
            }

            // ② 策略列表行
            StrategyRow row = new StrategyRow();
            row.port = port;
            row.ip = String.valueOf(port);

            switch (connStatus) {
                case CONNECTED -> row.status = "running";
                case DISCONNECTED -> row.status = "disconnected";
                case NO_PROCESS -> row.status = "no_process";
            }

            if (snap != null) {
                row.at = snap.active;
                row.id = snap.strategyID;
                row.modelFile = snap.modelFile;
                row.strategyType = snap.strategyType;
                row.val = snap.spread.isValid;
                row.l1 = snap.leg1.netpos;
                row.l2 = snap.leg2.netpos;
                row.pnl = snap.leg1.netPNL + snap.leg2.netPNL;
                row.time = snap.timestamp;

                // 提取品种 (从 symbol 中提取，如 ag2603 → ag)
                row.pro = extractProduct(snap.leg1.symbol);

                // 摘要信息: 价差 + 偏差
                row.information = String.format("spread=%.2f dev=%.2f",
                        snap.spread.current, snap.spread.deviation);

                // ⑤ 聚合挂单
                aggregateOrders(overview.orders, snap.leg1, snap.modelFile, row.pro, port);
                aggregateOrders(overview.orders, snap.leg2, snap.modelFile, row.pro, port);

                // ⑥ 聚合持仓
                aggregatePosition(overview.positions, snap.leg1, row.pro);
                aggregatePosition(overview.positions, snap.leg2, row.pro);

                // ⑦ 聚合成交 (从 orders 中提取 status=TRADED 的)
                aggregateFills(overview.fills, snap.leg1, row.pro);
                aggregateFills(overview.fills, snap.leg2, row.pro);

                // ④ 聚合价差成交 (配对 Leg1 + Leg2 的已成交订单)
                aggregateSpreadTrades(overview.spreadTrades, snap, row.pro);
            }

            overview.strategies.add(row);
        }

        return overview;
    }

    private static void aggregateOrders(List<OrderRow> target,
                                         DashboardSnapshot.LegSnapshot leg,
                                         String modelFile, String pro, int port) {
        if (leg.orders == null) return;
        for (DashboardSnapshot.OrderSnapshot ord : leg.orders) {
            // 只添加活跃挂单（非 TRADED/CANCEL_CONFIRM/NEW_REJECT）
            if ("TRADED".equals(ord.status) || "CANCEL_CONFIRM".equals(ord.status)
                    || "NEW_REJECT".equals(ord.status)) {
                continue;
            }
            OrderRow row = new OrderRow();
            row.symbol = leg.symbol;
            row.side = ord.side;
            row.qty = ord.openQty;
            row.price = ord.price;
            row.modelFile = modelFile;
            row.id = ord.orderID;
            row.pro = pro;
            row.port = port;
            target.add(row);
        }
    }

    private static void aggregatePosition(List<PositionRow> target,
                                            DashboardSnapshot.LegSnapshot leg,
                                            String pro) {
        if (leg.netpos == 0 && leg.netposPass == 0) return;
        PositionRow row = new PositionRow();
        row.symbol = leg.symbol;
        row.pos = leg.netpos;
        row.pro = pro;
        // 撤单比: cancelCount / orderCount
        if (leg.orderCount > 0) {
            double ratio = (double) leg.cancelCount / leg.orderCount * 100;
            row.cxlRio = String.format("%.1f%%", ratio);
        }
        target.add(row);
    }

    private static void aggregateFills(List<FillRow> target,
                                        DashboardSnapshot.LegSnapshot leg,
                                        String pro) {
        if (leg.orders == null) return;
        for (DashboardSnapshot.OrderSnapshot ord : leg.orders) {
            if ("TRADED".equals(ord.status) && ord.doneQty > 0) {
                FillRow row = new FillRow();
                row.time = ord.time;
                row.symbol = leg.symbol;
                row.side = ord.side;
                row.price = ord.price;
                row.qty = ord.doneQty;
                row.id = ord.orderID;
                row.pro = pro;
                target.add(row);
            }
        }
    }

    /**
     * 从 Leg1 + Leg2 的已成交订单中配对生成价差成交。
     * 配对逻辑: Leg1 BUY + Leg2 SELL → 买入价差; Leg1 SELL + Leg2 BUY → 卖出价差
     */
    private static void aggregateSpreadTrades(List<SpreadTradeRow> target,
                                               DashboardSnapshot snap, String pro) {
        List<DashboardSnapshot.OrderSnapshot> leg1Fills = new ArrayList<>();
        List<DashboardSnapshot.OrderSnapshot> leg2Fills = new ArrayList<>();

        if (snap.leg1.orders != null) {
            for (DashboardSnapshot.OrderSnapshot o : snap.leg1.orders) {
                if ("TRADED".equals(o.status) && o.doneQty > 0) leg1Fills.add(o);
            }
        }
        if (snap.leg2.orders != null) {
            for (DashboardSnapshot.OrderSnapshot o : snap.leg2.orders) {
                if ("TRADED".equals(o.status) && o.doneQty > 0) leg2Fills.add(o);
            }
        }

        // 尝试配对: Leg1 BUY ↔ Leg2 SELL, Leg1 SELL ↔ Leg2 BUY
        int pairs = Math.min(leg1Fills.size(), leg2Fills.size());
        for (int i = 0; i < pairs; i++) {
            DashboardSnapshot.OrderSnapshot f1 = leg1Fills.get(i);
            DashboardSnapshot.OrderSnapshot f2 = leg2Fills.get(i);
            SpreadTradeRow row = new SpreadTradeRow();
            row.modelFile = snap.modelFile;
            row.side = "BUY".equals(f1.side) ? "BUY" : "SELL";
            row.qty = Math.min(f1.doneQty, f2.doneQty);
            row.spread = f2.price - f1.price;
            row.time = f1.time != null && !f1.time.isEmpty() ? f1.time : f2.time;
            row.pro = pro;
            target.add(row);
        }
    }

    /**
     * 从 symbol 中提取品种代码 (ag2603 → ag)。
     */
    static String extractProduct(String symbol) {
        if (symbol == null || symbol.isEmpty()) return "";
        StringBuilder sb = new StringBuilder();
        for (char c : symbol.toCharArray()) {
            if (Character.isLetter(c)) {
                sb.append(c);
            } else {
                break;
            }
        }
        return sb.toString().toUpperCase();
    }
}
