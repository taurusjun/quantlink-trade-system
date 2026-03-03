package com.quantlink.trader.strategy;

import com.quantlink.trader.api.AlertEvent;
import com.quantlink.trader.core.*;
import com.quantlink.trader.shm.Constants;
import com.quantlink.trader.shm.Types;

import java.io.*;
import java.lang.foreign.MemorySegment;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import java.util.logging.Logger;

/**
 * 双腿配对套利策略。
 * 迁移自: tbsrc/Strategies/include/PairwiseArbStrategy.h (line 14-76)
 *         tbsrc/Strategies/PairwiseArbStrategy.cpp (947 lines)
 *
 * C++ class PairwiseArbStrategy : public ExecutionStrategy
 * 第一腿(firstStrat)被动挂单，第二腿(secondStrat)对冲。
 */
public class PairwiseArbStrategy extends ExecutionStrategy {

    private static final Logger log = Logger.getLogger(PairwiseArbStrategy.class.getName());

    // ---- 双腿策略 ----
    // 迁移自: PairwiseArbStrategy.h:63-64
    public ExtraStrategy firstStrat;
    public ExtraStrategy secondStrat;

    // ---- 合约引用 ----
    // 迁移自: PairwiseArbStrategy.h:39-40
    public Instrument firstinstru;
    public Instrument secondinstru;

    // ---- 阈值引用（每次 sendOrder 刷新） ----
    // 迁移自: PairwiseArbStrategy.h:65-66
    public ThresholdSet thold_first;
    public ThresholdSet thold_second;

    // ---- 价格 Map 快照（每次 sendOrder 刷新） ----
    // 迁移自: PairwiseArbStrategy.h:67-70
    public Map<Double, OrderStats> bidMap1;
    public Map<Double, OrderStats> bidMap2;
    public Map<Double, OrderStats> askMap1;
    public Map<Double, OrderStats> askMap2;

    // ---- OrderMap 引用 ----
    // 迁移自: PairwiseArbStrategy.h:71-72 — OrderMap *m_ordMap1, *m_ordMap2
    public Map<Integer, OrderStats> ordMap1;
    public Map<Integer, OrderStats> ordMap2;

    // ---- 价差字段 ----
    // 迁移自: PairwiseArbStrategy.h:49-53
    public double avgSpreadRatio_ori;
    public double avgSpreadRatio;
    public double currSpreadRatio;
    public double currSpreadRatio_prev;
    public double expectedRatio;

    // ---- 最佳行情 ----
    // 迁移自: PairwiseArbStrategy.h:44-47
    public double i1_bestBid;
    public double i1_bestAsk;
    public double i2_bestBid;
    public double i2_bestAsk;

    // ---- 其他字段 ----
    // 迁移自: PairwiseArbStrategy.h:41-43, 54-62
    public double maxloss_limit;
    public double count;
    public double currTime;
    public double lastTime;
    public double tValue;
    public boolean is_valid_mkdata = true;
    public int netpos_agg1;
    public int netpos_agg2;
    public int agg_repeat = 1;  // C++: m_agg_repeat{1}
    public double second_ordIDstart;

    // daily_init 文件路径（由构造函数保存，handleSquareoff 关闭时回写）
    private String dailyInitPath;

    // ---- Overview 页面所需的元数据（由 TraderMain 初始化后赋值） ----
    public String modelFile = "";       // 模型文件名（从 ControlConfig.modelFile 获取）
    public String strategyType = "";    // 策略类型（从 ControlConfig.execStrat 获取，如 TB_PAIR_STRAT）
    public String controlFilePath = ""; // 控制文件路径

    /**
     * 构造函数。
     * 迁移自: PairwiseArbStrategy::PairwiseArbStrategy(CommonClient*, SimConfig*)
     * Ref: PairwiseArbStrategy.cpp:7-84
     *
     * @param client    CommonClient
     * @param simConfig SimConfig
     * @param dailyInitPath daily_init 文件路径（C++ 在构造函数中调用 LoadMatrix2）
     */
    public PairwiseArbStrategy(CommonClient client, SimConfig simConfig, String dailyInitPath) {
        super(client, simConfig);

        // C++: m_firstStrat = new ExtraStrategy(client, simConfig)
        // C++: m_secondStrat = new ExtraStrategy(client, simConfig)
        firstStrat = new ExtraStrategy(client, simConfig);
        secondStrat = new ExtraStrategy(client, simConfig);

        // C++: m_secondStrat->m_instru = m_secondStrat->m_instru_sec
        secondStrat.instru = secondStrat.instruSec;

        // C++: m_ordMap1 = &m_firstStrat->m_ordMap
        // C++: m_ordMap2 = &m_secondStrat->m_ordMap
        ordMap1 = firstStrat.ordMap;
        ordMap2 = secondStrat.ordMap;

        log.info("PairwiseArbStrategy strategyID:" + strategyID
                + ",firstStrat:" + firstStrat.strategyID
                + ",secondStrat:" + secondStrat.strategyID);

        // C++: m_firstStrat->callSquareOff = false; m_secondStrat->callSquareOff = false
        firstStrat.callSquareOff = false;
        secondStrat.callSquareOff = false;

        // C++: m_firstStrat->m_targetBidPNL = new double[5]{1,1,1,1,1}
        firstStrat.targetBidPNL = new double[]{1, 1, 1, 1, 1};
        firstStrat.targetAskPNL = new double[]{1, 1, 1, 1, 1};
        secondStrat.targetBidPNL = new double[]{1, 1, 1, 1, 1};
        secondStrat.targetAskPNL = new double[]{1, 1, 1, 1, 1};

        // C++: m_firstinstru = m_firstStrat->m_instru; m_secondinstru = m_secondStrat->m_instru
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        expectedRatio = 0;
        currSpreadRatio = 0;
        currSpreadRatio_prev = 0;
        count = 100000;
        currTime = 0;
        lastTime = 0;
        second_ordIDstart = 10;

        // ---- LoadMatrix2 + 昨仓初始化 ----
        // C++: PairwiseArbStrategy.cpp:18-62 — 构造函数中直接调用 LoadMatrix2
        this.dailyInitPath = dailyInitPath;
        if (dailyInitPath != null && !dailyInitPath.isEmpty()) {
            loadDailyInitData(dailyInitPath);
        }
    }

    /**
     * 加载 daily_init 数据。
     * 从构造函数调用，匹配 C++ PairwiseArbStrategy 构造函数中的 LoadMatrix2 逻辑。
     */
    private void loadDailyInitData(String dailyInitPath) {
        Map<Integer, Map<String, String>> mx = loadMatrix2(dailyInitPath);

        if (!mx.containsKey(strategyID)) {
            throw new RuntimeException("daily_init ERROR! Missing strategyID " + strategyID);
        }

        String name1 = firstStrat.instru.origBaseName;
        String name2 = secondStrat.instru.origBaseName;
        if (name1.equals(name2)) {
            throw new RuntimeException("daily_init ERROR! origBaseName1:" + name1
                    + " origBaseName2:" + name2);
        }

        Map<String, String> row = mx.get(strategyID);
        avgSpreadRatio_ori = Double.parseDouble(row.getOrDefault("avgPx", "0"));
        avgSpreadRatio = avgSpreadRatio_ori;

        int netpos_ytd1 = Integer.parseInt(row.getOrDefault("ytd1", "0"));
        int netpos_2day1 = Integer.parseInt(row.getOrDefault("2day", "0"));
        int netpos_agg2_val = Integer.parseInt(row.getOrDefault("ytd2", "0"));

        // C++: m_firstStrat->m_netpos_pass_ytd = netpos_ytd1
        firstStrat.netposPassYtd = netpos_ytd1;
        firstStrat.netpos = netpos_ytd1 + netpos_2day1;
        firstStrat.netposPass = netpos_ytd1 + netpos_2day1;
        secondStrat.netpos = netpos_agg2_val;
        secondStrat.netposAgg = netpos_agg2_val;

        log.info("avgSpreadRatio_ori:" + avgSpreadRatio_ori
                + " origBaseName1:" + name1 + " netpos_ytd1:" + netpos_ytd1
                + " netpos_2day1:" + netpos_2day1 + " netpos1:" + firstStrat.netpos
                + " origBaseName2:" + name2 + " netpos_agg2:" + netpos_agg2_val);
    }

    // =======================================================================
    //  LoadMatrix2 / SaveMatrix2
    // =======================================================================

    /**
     * 加载 daily_init 格式文件。
     * 迁移自: PairwiseArbStrategy::LoadMatrix2(string filepath)
     * Ref: PairwiseArbStrategy.cpp:112-144
     *
     * 文件格式: 首行为 header，后续行以空格分隔。
     * 第一列为 strategyID，后续列按 header 名映射为 string value。
     */
    public Map<Integer, Map<String, String>> loadMatrix2(String filepath) {
        Map<Integer, Map<String, String>> mx = new LinkedHashMap<>();
        try (BufferedReader reader = new BufferedReader(new FileReader(filepath))) {
            String headerLine = reader.readLine();
            if (headerLine == null) return mx;
            String[] headers = headerLine.trim().split("\\s+");

            String line;
            while ((line = reader.readLine()) != null) {
                line = line.trim();
                if (line.isEmpty()) continue;
                String[] tokens = line.split("\\s+");
                int sid = Integer.parseInt(tokens[0]);
                Map<String, String> rowMap = new LinkedHashMap<>();
                for (int i = 1; i < Math.min(tokens.length, headers.length); i++) {
                    rowMap.put(headers[i], tokens[i]);
                }
                mx.put(sid, rowMap);
            }
        } catch (IOException e) {
            log.severe("Failed to load daily_init from " + filepath + ": " + e.getMessage());
        }
        return mx;
    }

    /**
     * 保存 daily_init 文件。
     * 迁移自: PairwiseArbStrategy::SaveMatrix2(string filepath)
     * Ref: PairwiseArbStrategy.cpp:653-686
     */
    public void saveMatrix2(String filepath) {
        try (PrintWriter out = new PrintWriter(new FileWriter(filepath))) {
            // C++: const string Head = "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 "
            out.println("StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2");
            out.println(strategyID + " 0 " + avgSpreadRatio_ori
                    + " " + firstStrat.instru.origBaseName
                    + " " + secondStrat.instru.origBaseName
                    + " " + firstStrat.netposPass
                    + " " + secondStrat.netposAgg);
        } catch (IOException e) {
            log.severe("Failed to save daily_init to " + filepath + ": " + e.getMessage());
        }
    }

    // =======================================================================
    //  SendOrder — 被动挂单 + 对冲逻辑
    // =======================================================================

    /**
     * 发单逻辑 — 第一腿被动挂单 + 第二腿对冲。
     * 迁移自: PairwiseArbStrategy::SendOrder()
     * Ref: PairwiseArbStrategy.cpp:146-385
     */
    @Override
    public void sendOrder() {
        OrderStats.HitType ordType = OrderStats.HitType.STANDARD;

        // C++: 刷新合约/价格/阈值引用
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        bidMap1 = firstStrat.bidMap;
        bidMap2 = secondStrat.bidMap;
        askMap1 = firstStrat.askMap;
        askMap2 = secondStrat.askMap;

        thold_first = firstStrat.thold;
        thold_second = secondStrat.thold;

        i1_bestBid = firstinstru.bidPx[0];
        i1_bestAsk = firstinstru.askPx[0];
        i2_bestBid = secondinstru.bidPx[0];
        i2_bestAsk = secondinstru.askPx[0];
        maxloss_limit = thold_first.MAX_LOSS;

        setThresholds();

        // C++: 撤销 firstStrat 中 CROSS/MATCH 类型的订单
        for (OrderStats order : new ArrayList<>(ordMap1.values())) {
            if (order.ordType == OrderStats.HitType.CROSS || order.ordType == OrderStats.HitType.MATCH) {
                firstStrat.sendCancelOrder(firstinstru, order.orderID);
            }
        }
        // C++: 撤销 secondStrat 中 CROSS/MATCH 类型的订单
        for (OrderStats order : new ArrayList<>(ordMap2.values())) {
            if (order.ordType == OrderStats.HitType.CROSS || order.ordType == OrderStats.HitType.MATCH) {
                secondStrat.sendCancelOrder(secondinstru, order.orderID);
            }
        }

        // C++: 检查阈值有效
        if (firstStrat.tholdBidPlace != -1 && firstStrat.tholdBidRemove != -1
                && firstStrat.tholdAskPlace != -1 && firstStrat.tholdAskRemove != -1) {

            // C++: 撤销价差超出范围的买单
            for (OrderStats order : new ArrayList<>(bidMap1.values())) {
                double longSpread = order.price - secondinstru.bidPx[0];
                if ((longSpread > avgSpreadRatio - firstStrat.tholdBidRemove)
                        && (order.status == OrderStats.Status.NEW_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_REJECT)) {
                    firstStrat.sendCancelOrder(firstinstru, order.orderID);
                }
            }
            // C++: 撤销价差超出范围的卖单
            for (OrderStats order : new ArrayList<>(askMap1.values())) {
                double shortSpread = order.price - secondinstru.askPx[0];
                if ((shortSpread < avgSpreadRatio + firstStrat.tholdAskRemove)
                        && (order.status == OrderStats.Status.NEW_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_CONFIRM
                        || order.status == OrderStats.Status.MODIFY_REJECT)) {
                    firstStrat.sendCancelOrder(firstinstru, order.orderID);
                }
            }

            // C++: 行情无效检查
            if (firstinstru.bidPx[0] == 0 || firstinstru.askPx[0] == 0
                    || secondinstru.bidPx[0] == 0 || secondinstru.askPx[0] == 0) {
                return;
            }

            // C++: 第一腿多层被动挂单
            // Ref: PairwiseArbStrategy.cpp:235-346
            for (int level = 0; level < thold_first.MAX_QUOTE_LEVEL; level++) {
                double longSpreadRatio1 = firstinstru.bidPx[level] - secondinstru.bidPx[0];
                double shortSpreadRatio1 = firstinstru.askPx[level] - secondinstru.askPx[0];

                // 卖单逻辑
                if (shortSpreadRatio1 > avgSpreadRatio + firstStrat.tholdAskPlace) {
                    double passive_sellprice1 = firstinstru.askPx[level];
                    passive_sellprice1 = getAskPriceFirst(passive_sellprice1, level);

                    if (firstStrat.netposPass * -1 < firstStrat.tholdAskMaxPos) {
                        if (firstStrat.sellOpenOrders > firstStrat.thold.SUPPORTING_ORDERS
                                || firstStrat.sellOpenQty + -1 * firstStrat.netposPass >= firstStrat.tholdAskMaxPos) {
                            // 找最远卖单，如果新价格更优则撤销最远单
                            double askHigh1 = findHighestConfirmedAskPrice(askMap1);
                            if (!askMap1.containsKey(passive_sellprice1) && passive_sellprice1 < askHigh1) {
                                if (askHigh1 != 0) {
                                    OrderStats farOrder = askMap1.get(askHigh1);
                                    if (farOrder != null) {
                                        firstStrat.sendCancelOrder(firstinstru, farOrder.orderID);
                                    }
                                }
                            }
                        } else {
                            firstStrat.sendAskOrder2(firstinstru, level, passive_sellprice1, ordType, 0);
                        }
                    } else {
                        // 超过最大卖仓，撤销所有卖单
                        for (OrderStats order : new ArrayList<>(askMap1.values())) {
                            firstStrat.sendCancelOrder(firstinstru, order.orderID);
                        }
                    }
                }

                // 买单逻辑
                if (longSpreadRatio1 < avgSpreadRatio - firstStrat.tholdBidPlace) {
                    double passive_buyprice1 = firstinstru.bidPx[level];
                    passive_buyprice1 = getBidPriceFirst(passive_buyprice1, level);

                    if (firstStrat.netposPass < firstStrat.tholdBidMaxPos) {
                        if (firstStrat.buyOpenOrders > firstStrat.thold.SUPPORTING_ORDERS
                                || firstStrat.buyOpenQty + firstStrat.netposPass >= firstStrat.tholdBidMaxPos) {
                            double bidLow1 = findLowestConfirmedBidPrice(bidMap1);
                            if (!bidMap1.containsKey(passive_buyprice1) && passive_buyprice1 > bidLow1) {
                                if (bidLow1 != 0) {
                                    OrderStats farOrder = bidMap1.get(bidLow1);
                                    if (farOrder != null) {
                                        firstStrat.sendCancelOrder(firstinstru, farOrder.orderID);
                                    }
                                }
                            }
                        } else {
                            firstStrat.sendBidOrder2(firstinstru, level, passive_buyprice1, ordType, 0);
                        }
                    } else {
                        for (OrderStats order : new ArrayList<>(bidMap1.values())) {
                            firstStrat.sendCancelOrder(firstinstru, order.orderID);
                        }
                    }
                }
            }

            // C++: 对冲检查
            // Ref: PairwiseArbStrategy.cpp:348-375
            int pending_netpos_agg2 = calcPendingNetposAgg();
            long now_ts = System.nanoTime() / 1000; // microseconds

            int netExposure = firstStrat.netposPass + secondStrat.netposAgg + pending_netpos_agg2;

            if (netExposure > 0
                    && secondStrat.sellAggOrder <= firstStrat.thold.SUPPORTING_ORDERS
                    && (secondStrat.lastAggSide != Constants.SIDE_SELL
                    || (secondStrat.lastAggSide == Constants.SIDE_SELL
                    && now_ts / 1000 - secondStrat.lastAggTime > 100))) {
                // 价差净多头，第二腿卖出对冲
                secondStrat.sendAskOrder2(secondinstru, 0,
                        secondinstru.bidPx[0] - secondStrat.instru.tickSize,
                        OrderStats.HitType.CROSS, netExposure);
                secondStrat.sellAggOrder++;
                secondStrat.lastAggTime = now_ts / 1000;
                secondStrat.lastAggSide = Constants.SIDE_SELL;
            } else if (netExposure < 0
                    && secondStrat.buyAggOrder <= firstStrat.thold.SUPPORTING_ORDERS
                    && (secondStrat.lastAggSide != Constants.SIDE_BUY
                    || (secondStrat.lastAggSide == Constants.SIDE_BUY
                    && now_ts / 1000 - secondStrat.lastAggTime > 100))) {
                // 价差净空头，第二腿买入对冲
                secondStrat.sendBidOrder2(secondinstru, 0,
                        secondinstru.askPx[0] + secondStrat.instru.tickSize,
                        OrderStats.HitType.CROSS, -netExposure);
                secondStrat.buyAggOrder++;
                secondStrat.lastAggTime = now_ts / 1000;
                secondStrat.lastAggSide = Constants.SIDE_BUY;
            }
        }
    }

    // =======================================================================
    //  SendAggressiveOrder — 主动追单
    // =======================================================================

    /**
     * 主动追单 — 在敞口持续时重复追单。
     * 迁移自: PairwiseArbStrategy::SendAggressiveOrder()
     * Ref: PairwiseArbStrategy.cpp:701-800
     */
    public void sendAggressiveOrder() {
        firstinstru = firstStrat.instru;
        secondinstru = secondStrat.instru;

        int pending_netpos_agg2 = calcPendingNetposAgg();
        long now_ts = System.nanoTime() / 1000; // microseconds
        int netExposure = firstStrat.netposPass + secondStrat.netposAgg + pending_netpos_agg2;

        if (netExposure > 0 && secondStrat.sellAggOrder <= secondStrat.thold.SUPPORTING_ORDERS) {
            if (secondStrat.lastAggSide != Constants.SIDE_SELL
                    || (secondStrat.lastAggSide == Constants.SIDE_SELL
                    && now_ts / 1000 - secondStrat.lastAggTime > 500)) {
                // 首次或超过500ms，按市场行情发单
                secondStrat.sendAskOrder2(secondinstru, 0,
                        secondinstru.bidPx[0], OrderStats.HitType.CROSS, netExposure);
                secondStrat.sellAggOrder++;
                secondStrat.lastAggTime = now_ts / 1000;
                secondStrat.lastAggSide = Constants.SIDE_SELL;
                log.info(String.format("[AGG-ORDER] leg=secondStrat side=SELL price=%.1f qty=%d aggRepeat=%d exposure=%d",
                        secondinstru.bidPx[0], netExposure, agg_repeat, netExposure));
            } else {
                if (agg_repeat > 3) {
                    log.warning("Reach max agg_repeat, deactive Strategy");
                    handleSquareoff();
                } else {
                    // C++: agg_repeat < 3 ? bidPx[0] - tickSize * agg_repeat : bidPx[0] - tickSize * SLOP
                    double aggPrice = agg_repeat < 3
                            ? secondinstru.bidPx[0] - secondStrat.instru.tickSize * agg_repeat
                            : secondinstru.bidPx[0] - secondStrat.instru.tickSize * secondStrat.thold.SLOP;
                    boolean ret = secondStrat.sendAskOrder2(secondinstru, 0, aggPrice,
                            OrderStats.HitType.CROSS, netExposure);
                    if (ret) {
                        agg_repeat++;
                        secondStrat.sellAggOrder++;
                        secondStrat.lastAggTime = now_ts / 1000;
                        secondStrat.lastAggSide = Constants.SIDE_SELL;
                    }
                }
            }
        } else if (netExposure < 0 && secondStrat.buyAggOrder <= secondStrat.thold.SUPPORTING_ORDERS) {
            if (secondStrat.lastAggSide != Constants.SIDE_BUY
                    || (secondStrat.lastAggSide == Constants.SIDE_BUY
                    && now_ts / 1000 - secondStrat.lastAggTime > 500)) {
                secondStrat.sendBidOrder2(secondinstru, 0,
                        secondinstru.askPx[0], OrderStats.HitType.CROSS, -netExposure);
                secondStrat.buyAggOrder++;
                secondStrat.lastAggTime = now_ts / 1000;
                secondStrat.lastAggSide = Constants.SIDE_BUY;
                log.info(String.format("[AGG-ORDER] leg=secondStrat side=BUY price=%.1f qty=%d aggRepeat=%d exposure=%d",
                        secondinstru.askPx[0], -netExposure, agg_repeat, netExposure));
            } else {
                if (agg_repeat > 3) {
                    log.warning("Reach max agg_repeat, deactive Strategy");
                    handleSquareoff();
                } else {
                    double aggPrice = agg_repeat < 3
                            ? secondinstru.askPx[0] + secondStrat.instru.tickSize * agg_repeat
                            : secondinstru.askPx[0] + secondStrat.instru.tickSize * secondStrat.thold.SLOP;
                    boolean ret = secondStrat.sendBidOrder2(secondinstru, 0, aggPrice,
                            OrderStats.HitType.CROSS, -netExposure);
                    if (ret) {
                        agg_repeat++;
                        secondStrat.buyAggOrder++;
                        secondStrat.lastAggTime = now_ts / 1000;
                        secondStrat.lastAggSide = Constants.SIDE_BUY;
                    }
                }
            }
        }
    }

    // =======================================================================
    //  SetThresholds — 覆盖基类
    // =======================================================================

    /**
     * 设置双腿阈值（线性插值）。
     * 迁移自: PairwiseArbStrategy::SetThresholds()
     * Ref: PairwiseArbStrategy.cpp:902-947
     */
    @Override
    public void setThresholds() {
        // C++: if (m_firstinstru->m_sendInLots && m_secondinstru->m_sendInLots)
        if (firstinstru.sendInLots && secondinstru.sendInLots) {
            firstStrat.tholdMaxPos = Math.max(thold_first.BID_MAX_SIZE, thold_first.ASK_MAX_SIZE);
            firstStrat.tholdBeginPos = thold_first.BEGIN_SIZE;
            firstStrat.tholdSize = thold_first.SIZE;

            firstStrat.tholdBidSize = thold_first.BID_SIZE;
            firstStrat.tholdBidMaxPos = thold_first.BID_MAX_SIZE;
            firstStrat.tholdAskSize = thold_first.ASK_SIZE;
            firstStrat.tholdAskMaxPos = thold_first.ASK_MAX_SIZE;
        } else {
            firstStrat.tholdMaxPos = (int) (thold_first.MAX_SIZE * firstinstru.lotSize);
            firstStrat.tholdBeginPos = (int) (thold_first.BEGIN_SIZE * firstinstru.lotSize);
            firstStrat.tholdSize = (int) (thold_first.SIZE * firstinstru.lotSize);
        }

        // C++: 线性插值阈值
        double longPlaceDiff = thold_first.LONG_PLACE - thold_first.BEGIN_PLACE;
        double shortPlaceDiff = thold_first.BEGIN_PLACE - thold_first.SHORT_PLACE;
        double longRemoveDiff = thold_first.LONG_REMOVE - thold_first.BEGIN_REMOVE;
        double shortRemoveDiff = thold_first.BEGIN_REMOVE - thold_first.SHORT_REMOVE;

        if (firstStrat.netposPass == 0) {
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE;
        } else if (firstStrat.netposPass > 0) {
            // C++: m_tholdBidPlace = BEGIN_PLACE + longPlaceDiff * netpos_pass / tholdMaxPos
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE + longPlaceDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE + longRemoveDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE - shortPlaceDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE - shortRemoveDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
        } else {
            // netpos_pass < 0
            firstStrat.tholdBidPlace = thold_first.BEGIN_PLACE + shortPlaceDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdBidRemove = thold_first.BEGIN_REMOVE + shortRemoveDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskPlace = thold_first.BEGIN_PLACE - longPlaceDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
            firstStrat.tholdAskRemove = thold_first.BEGIN_REMOVE - longRemoveDiff * firstStrat.netposPass / firstStrat.tholdMaxPos;
        }
    }

    // =======================================================================
    //  ORS 回调
    // =======================================================================

    /**
     * 路由回报到 firstStrat 或 secondStrat。
     * 迁移自: PairwiseArbStrategy::ORSCallBack(ResponseMsg*)
     * Ref: PairwiseArbStrategy.cpp:428-477
     */
    @Override
    public void orsCallBack(MemorySegment response) {
        int orderID = (int) Types.RESP_ORDER_ID_VH.get(response, 0L);
        int responseType = (int) Types.RESP_RESPONSE_TYPE_VH.get(response, 0L);

        if (ordMap1.containsKey(orderID)) {
            // 第一腿回报
            log.info(String.format("[PAIR-ORS] orderID=%d routed=firstStrat type=%d", orderID, responseType));
            firstStrat.orsCallBack(response);
            if (responseType == Constants.RESP_TRADE_CONFIRM) {
                agg_repeat = 1;
            }
        } else if (ordMap2.containsKey(orderID)) {
            // 第二腿回报 — 先处理 aggOrder 计数再调用基类
            log.info(String.format("[PAIR-ORS] orderID=%d routed=secondStrat type=%d", orderID, responseType));
            handleAggOrder(response, ordMap2.get(orderID), secondStrat);
            secondStrat.orsCallBack(response);
            if (responseType == Constants.RESP_TRADE_CONFIRM) {
                agg_repeat = 1;
            }
        }

        // C++: m_rejectCount = m_firstStrat->m_rejectCount + m_secondStrat->m_rejectCount
        rejectCount = firstStrat.rejectCount + secondStrat.rejectCount;

        if (active) {
            sendAggressiveOrder();
        }
    }

    // =======================================================================
    //  MD 回调
    // =======================================================================

    /**
     * 行情回调 — 转发到两腿 + 计算价差。
     * 迁移自: PairwiseArbStrategy::MDCallBack(MarketUpdateNew*)
     * Ref: PairwiseArbStrategy.cpp:479-569
     */
    @Override
    public void mdCallBack(MemorySegment update) {
        // C++: m_onFlat = m_firstStrat->m_onFlat && m_secondStrat->m_onFlat
        onFlat = firstStrat.onFlat && secondStrat.onFlat;

        // C++: curr_time = Watch::GetUniqueInstance()->GetCurrentTime();
        // Ref: PairwiseArbStrategy.cpp:527
        // Watch 在 CommonClient.sendINDUpdate() 中已统一更新，此处同步 exchTS 字段以兼容遗留引用。
        exchTS = Watch.getInstance().getCurrentTime();

        // C++: 双腿合计PNL 检查 max_loss
        if (firstStrat.netPNL + secondStrat.netPNL < -1 * maxloss_limit) {
            firstStrat.callSquareOff = true;
            secondStrat.callSquareOff = true;
        }

        // C++: 转发到两腿
        firstStrat.mdCallBack(update);
        secondStrat.mdCallBack(update);

        // C++: 计算当前价差
        // C++: if (firstBid <= 0 || firstAsk <= 0 || secondBid <= 0 && secondAsk <= 0)
        // 注意: C++/Java 中 && 优先级高于 ||，所以 second leg 需要 bid AND ask 都 <= 0 才跳过。
        // 这是 C++ 原代码行为，保持一致。Ref: PairwiseArbStrategy.cpp:496
        if (firstStrat.instru.bidPx[0] <= 0 || firstStrat.instru.askPx[0] <= 0
                || secondStrat.instru.bidPx[0] <= 0 && secondStrat.instru.askPx[0] <= 0) {
            // currSpreadRatio 不变
        } else {
            currSpreadRatio = ((firstStrat.instru.bidPx[0] + firstStrat.instru.askPx[0]) / 2)
                    - ((secondStrat.instru.bidPx[0] + secondStrat.instru.askPx[0]) / 2);
            expectedRatio = currSpreadRatio;
        }

        // C++: AVG_SPREAD_AWAY 检查
        // [C++差异] currSpreadRatio 初始化为 0，行情未到齐时保持为 0，
        // 与 avgSpreadRatio(~360) 的差会远超阈值，导致启动即触发 deactivate。
        // C++ 中不存在此问题因为启动后立即 activate 且行情瞬间到齐。
        // 加入 currSpreadRatio != 0 守卫，确保至少收到过一次完整行情后再做检查。
        if (currSpreadRatio != 0 && Math.abs(currSpreadRatio - avgSpreadRatio)
                > firstStrat.instru.tickSize * firstStrat.thold.AVG_SPREAD_AWAY) {
            if (active) {
                // active 状态下触发退出（保持 C++ 原行为）
                is_valid_mkdata = false;
                log.warning("Error avgSpreadRatio, Exit Strategy. currSpread:" + currSpreadRatio
                        + " avgSpread:" + avgSpreadRatio + " AVG_SPREAD_AWAY:" + firstStrat.thold.AVG_SPREAD_AWAY);

                // 告警事件采集 — AVG_SPREAD_AWAY
                firstStrat.alertCollector.add(new AlertEvent(AlertEvent.LEVEL_CRITICAL,
                        AlertEvent.TYPE_AVG_SPREAD_AWAY,
                        String.format("AVG_SPREAD_AWAY triggered. currSpread=%.2f avgSpread=%.2f drift=%.2f threshold=%.0f",
                                currSpreadRatio, avgSpreadRatio,
                                Math.abs(currSpreadRatio - avgSpreadRatio),
                                firstStrat.instru.tickSize * firstStrat.thold.AVG_SPREAD_AWAY),
                        firstStrat.instru.origBaseName, strategyID));

                handleSquareoff();
                return;
            } else {
                // inactive 状态下仅 warning，不退出 — 等待激活时 handleSquareON() 自动重置 avgSpread
                log.warning(String.format("[AVG-SPREAD-DRIFT] inactive, 跳过exit. currSpread=%.2f avgSpread=%.2f drift=%.2f threshold=%.0f",
                        currSpreadRatio, avgSpreadRatio,
                        Math.abs(currSpreadRatio - avgSpreadRatio),
                        firstStrat.instru.tickSize * firstStrat.thold.AVG_SPREAD_AWAY));
            }
        }

        // C++: 收到第一腿行情时更新 avgSpreadRatio_ori (EWA)
        // Ref: PairwiseArbStrategy.cpp:519-523
        // 读取行情中的 symbol 进行比对
        byte[] symBytes = new byte[32];
        MemorySegment symSeg = update.asSlice(Types.MDH_SYMBOL_OFFSET, 32);
        symSeg.asByteBuffer().get(symBytes);
        String symbol = new String(symBytes, StandardCharsets.US_ASCII).trim().replace("\0", "");

        if (symbol.equals(firstStrat.instru.symbol)) {
            avgSpreadRatio_ori = (1 - firstStrat.thold.ALPHA) * avgSpreadRatio_ori
                    + firstStrat.thold.ALPHA * currSpreadRatio;
            avgSpreadRatio = avgSpreadRatio_ori + tValue;
        }
        currSpreadRatio_prev = currSpreadRatio;

        is_valid_mkdata = true;

        // C++: 时间限制检查 — endTimeAggEpoch / endTimeEpoch
        // C++: Watch::GetUniqueInstance()->GetCurrentTime() 返回交易所时间戳（纳秒 epoch）
        // Ref: PairwiseArbStrategy.cpp:547, ExecutionStrategy.cpp:2152
        // [C++差异] Java 策略在 CTP 模式下需等待手动激活（active=false）。
        // 如果策略在 endTime 之后才启动，C++ 的 endTime 检查会立即触发 HandleSquareoff，
        // 导致对昨仓发出平仓订单（SELL 82 / BUY 83 flag=OPEN）。
        // C++ 不存在此问题因为策略启动时 endTime 尚未到达。
        // 守卫：active=false 时跳过 endTime 检查，等待用户激活后正常运行。
        long currentTime = Watch.getInstance().getCurrentTime();
        if (active) {
            if (currentTime >= endTimeAggEpoch && !aggFlat) {
                aggFlat = true;
                onExit = true;
                onCancel = true;
                onFlat = true;
                log.warning("Exchange Time Limit reached. Aggressive flat!");
                handleSquareoff();
            }

            if (currentTime >= endTimeEpoch && !onExit) {
                onExit = true;
                onCancel = true;
                onFlat = true;
                log.warning("END TIME limit reached. Square off called.");
                handleSquareoff();
            }
        }
    }

    // =======================================================================
    //  HandleSquareON — 激活策略
    // =======================================================================

    /**
     * 激活策略 — 重置退出标志 + 用当前 spread 重新初始化 avgSpreadRatio。
     * 迁移自: PairwiseArbStrategy::HandleSquareON()
     * Ref: PairwiseArbStrategy.cpp:571-584, main.cpp:140-148
     *
     * C++ HandleSquareON 重置 onExit/onCancel/onFlat/aggFlat 标志。
     * Java 额外增加: 将 avgSpreadRatio_ori 重置为当前 spread，
     * 避免 daily_init 中的旧 avgPx 导致 AVG_SPREAD_AWAY 立即触发 deactivate。
     */
    public void handleSquareON() {
        // C++: ExecutionStrategy::HandleSquareON()
        // Ref: ExecutionStrategy.h:47-51
        onExit = false;
        onCancel = false;
        onFlat = false;
        aggFlat = false;

        // C++: m_agg_repeat = 1
        agg_repeat = 1;

        // C++: 重置双腿标志
        firstStrat.onExit = false;
        firstStrat.onCancel = false;
        firstStrat.onFlat = false;
        secondStrat.onExit = false;
        secondStrat.onCancel = false;
        secondStrat.onFlat = false;

        // [C++差异] C++ HandleSquareON 不重置 avgSpreadRatio，
        // 但 C++ 正常运行时 daily_init 的 avgPx 是前一交易日收盘时保存的正确值。
        // Java 在开发/测试阶段 daily_init 可能包含过时的 avgPx，
        // 导致 AVG_SPREAD_AWAY 检查在 activate 后立即触发 deactivate。
        // 因此在 activate 时用当前实时 spread 重新初始化 avgSpreadRatio_ori。
        double bid1 = firstStrat.instru.bidPx[0];
        double ask1 = firstStrat.instru.askPx[0];
        double bid2 = secondStrat.instru.bidPx[0];
        double ask2 = secondStrat.instru.askPx[0];
        if (bid1 > 0 && ask1 > 0 && bid2 > 0 && ask2 > 0) {
            double liveSpread = ((bid1 + ask1) / 2) - ((bid2 + ask2) / 2);
            double oldAvg = avgSpreadRatio_ori;
            avgSpreadRatio_ori = liveSpread;
            avgSpreadRatio = avgSpreadRatio_ori + tValue;

            double drift = Math.abs(oldAvg - liveSpread);
            double threshold = firstStrat.instru.tickSize * firstStrat.thold.AVG_SPREAD_AWAY;
            if (drift > threshold) {
                log.warning(String.format("[AVG-SPREAD-DRIFT] 检测到跨天漂移，自动修复: oldAvg=%.4f -> newAvg=%.4f (drift=%.2f, threshold=%.0f)",
                        oldAvg, avgSpreadRatio, drift, threshold));
            } else {
                log.info(String.format("[HandleSquareON] avgSpreadRatio 重置: %.4f -> %.4f (liveSpread=%.4f tValue=%.4f)",
                        oldAvg, avgSpreadRatio, liveSpread, tValue));
            }
        }

        log.info("[HandleSquareON] 策略已激活, onExit=false aggFlat=false");
    }

    // =======================================================================
    //  HandleSquareoff — 双腿平仓
    // =======================================================================

    /**
     * 双腿平仓 — 设置退出标志 + 撤销所有订单 + 保存 daily_init。
     * 迁移自: PairwiseArbStrategy::HandleSquareoff()
     * Ref: PairwiseArbStrategy.cpp:586-626
     */
    @Override
    public void handleSquareoff() {
        // C++: 设置双腿退出标志
        firstStrat.onExit = true;
        firstStrat.onCancel = true;
        firstStrat.onFlat = true;
        secondStrat.onExit = true;
        secondStrat.onCancel = true;
        secondStrat.onFlat = true;

        // C++: 撤销两腿所有订单
        for (OrderStats order : new ArrayList<>(ordMap1.values())) {
            firstStrat.sendCancelOrder(firstinstru, order.orderID);
        }
        for (OrderStats order : new ArrayList<>(ordMap2.values())) {
            secondStrat.sendCancelOrder(secondinstru, order.orderID);
        }

        active = false;

        log.warning(String.format("[PAIR-EXIT] active=false avgSpread=%.4f ytd1=%d ytd2=%d netpos1=%d netpos2=%d",
                avgSpreadRatio_ori, firstStrat.netposPassYtd, secondStrat.netposPassYtd,
                firstStrat.netpos, secondStrat.netpos));

        // C++: SaveMatrix2("../data/daily_init." + strategyID)
        // [C++差异] C++ 硬编码 "../data/daily_init."，Java 使用构造时传入的 dailyInitPath，
        // 确保保存路径与加载路径一致（sim → ./data/sim/, ctp → ./data/live/）。
        if (dailyInitPath != null && !dailyInitPath.isEmpty()) {
            saveMatrix2(dailyInitPath);
        }
    }

    // =======================================================================
    //  CalcPendingNetposAgg
    // =======================================================================

    /**
     * 计算第二腿挂起的 CROSS/MATCH 订单净仓位。
     * 迁移自: PairwiseArbStrategy::CalcPendingNetposAgg()
     * Ref: PairwiseArbStrategy.cpp:688-698
     */
    public int calcPendingNetposAgg() {
        int netpos_agg_pending = 0;
        for (OrderStats order : ordMap2.values()) {
            if (order.ordType == OrderStats.HitType.CROSS || order.ordType == OrderStats.HitType.MATCH) {
                if (order.side == Constants.SIDE_BUY) {
                    netpos_agg_pending += order.openQty;
                } else {
                    netpos_agg_pending -= order.openQty;
                }
            }
        }
        return netpos_agg_pending;
    }

    // =======================================================================
    //  HandleAggOrder — 对冲订单处理
    // =======================================================================

    /**
     * 处理对冲订单的回报计数。
     * 迁移自: PairwiseArbStrategy::HandleAggOrder(ResponseMsg*, OrderStats*, ExtraStrategy*)
     * Ref: PairwiseArbStrategy.cpp:402-426
     */
    private void handleAggOrder(MemorySegment response, OrderStats order, ExtraStrategy strat) {
        int responseType = (int) Types.RESP_RESPONSE_TYPE_VH.get(response, 0L);
        int tradeQty = (int) Types.RESP_QUANTITY_VH.get(response, 0L);

        // C++: 在特定回报类型时递减 aggOrder 计数
        if (responseType == Constants.RESP_NEW_ORDER_FREEZE
                || responseType == Constants.RESP_ORDER_ERROR
                || responseType == Constants.RESP_CANCEL_ORDER_CONFIRM
                || (responseType == Constants.RESP_TRADE_CONFIRM && order.openQty == tradeQty)) {
            if (order.side == Constants.SIDE_BUY) {
                strat.buyAggOrder--;
            } else {
                strat.sellAggOrder--;
            }
        }
    }

    // =======================================================================
    //  GetBidPrice / GetAskPrice — 价格优化
    // =======================================================================

    /**
     * 获取第一腿买价（考虑隐藏订单簿优化）。
     * 迁移自: PairwiseArbStrategy::GetBidPrice_first(double&, OrderHitType&, int32_t&)
     * Ref: PairwiseArbStrategy.cpp:802-820
     *
     * 逻辑:
     * 1. 检测 bidPx[level] 与 bidPx[level-1] 之间是否有超过 1 tick 的间隙
     * 2. 计算假设上移 1 tick 后的 spread（bidInv）
     * 3. 如果 bidInv 仍满足 BEGIN_PLACE 阈值且当前档位有 quantAhead > lotSize，则上移 1 tick
     */
    public double getBidPriceFirst(double price, int level) {
        // C++: price = m_firstStrat->m_instru->bidPx[level];
        // (price 已由调用方设置为 firstinstru.bidPx[level])

        // C++: if (m_configParams->m_bUseInvisibleBook && level != 0
        //      && price < m_firstStrat->m_instru->bidPx[level - 1] - m_firstStrat->m_instru->m_tickSize)
        if (configParams.bUseInvisibleBook && level != 0
                && price < firstStrat.instru.bidPx[level - 1] - firstStrat.instru.tickSize) {
            // C++: double bidInv = m_firstStrat->m_instru->bidPx[level] - m_secondStrat->m_instru->bidPx[0]
            //                    + m_firstStrat->m_instru->m_tickSize;
            double bidInv = firstStrat.instru.bidPx[level] - secondStrat.instru.bidPx[0]
                    + firstStrat.instru.tickSize;

            // C++: if (bidInv <= avgSpreadRatio - m_firstStrat->m_thold->BEGIN_PLACE)
            if (bidInv <= avgSpreadRatio - firstStrat.thold.BEGIN_PLACE) {
                // C++: PriceMapIter iter = m_bidMap1.find(price);
                OrderStats existing = bidMap1.get(price);
                // C++: if (iter != m_bidMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize)
                if (existing != null && existing.quantAhead > firstinstru.lotSize) {
                    log.fine("1st One"); // C++: TBLOG << "1st One" << endl;
                    // C++: price = m_firstStrat->m_instru->bidPx[level] + m_firstStrat->m_instru->m_tickSize;
                    price = firstStrat.instru.bidPx[level] + firstStrat.instru.tickSize;
                }
            }
        }
        return price;
    }

    /**
     * 获取第一腿卖价（考虑隐藏订单簿优化）。
     * 迁移自: PairwiseArbStrategy::GetAskPrice_first(double&, OrderHitType&, int32_t&)
     * Ref: PairwiseArbStrategy.cpp:822-840
     *
     * 逻辑:
     * 1. 检测 askPx[level] 与 askPx[level-1] 之间是否有超过 1 tick 的间隙
     * 2. 计算假设下移 1 tick 后的 spread（askInv）
     * 3. 如果 askInv 仍满足 BEGIN_PLACE 阈值且当前档位有 quantAhead > lotSize，则下移 1 tick
     */
    public double getAskPriceFirst(double price, int level) {
        // C++: price = m_firstStrat->m_instru->askPx[level];
        // (price 已由调用方设置为 firstinstru.askPx[level])

        // C++: if (m_configParams->m_bUseInvisibleBook && level != 0
        //      && price > m_firstStrat->m_instru->askPx[level - 1] + m_firstStrat->m_instru->m_tickSize)
        if (configParams.bUseInvisibleBook && level != 0
                && price > firstStrat.instru.askPx[level - 1] + firstStrat.instru.tickSize) {
            // C++: double askInv = m_firstStrat->m_instru->askPx[level] - m_secondStrat->m_instru->askPx[0]
            //                    - m_firstStrat->m_instru->m_tickSize;
            double askInv = firstStrat.instru.askPx[level] - secondStrat.instru.askPx[0]
                    - firstStrat.instru.tickSize;

            // C++: if (askInv >= avgSpreadRatio + m_firstStrat->m_thold->BEGIN_PLACE)
            if (askInv >= avgSpreadRatio + firstStrat.thold.BEGIN_PLACE) {
                // C++: PriceMapIter iter = m_askMap1.find(price);
                OrderStats existing = askMap1.get(price);
                // C++: if (iter != m_askMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize)
                if (existing != null && existing.quantAhead > firstinstru.lotSize) {
                    log.fine("2nd One"); // C++: TBLOG << "2nd One" << endl;
                    // C++: price = m_firstStrat->m_instru->askPx[level] - m_firstStrat->m_instru->m_tickSize;
                    price = firstStrat.instru.askPx[level] - firstStrat.instru.tickSize;
                }
            }
        }
        return price;
    }

    /**
     * 获取第二腿买价（含隐藏订单簿逻辑）。
     * 迁移自: PairwiseArbStrategy::GetBidPrice_second(double &price, OrderHitType &ordType, int32_t &level)
     * Ref: PairwiseArbStrategy.cpp:842-861
     *
     * [C++差异] C++ 使用引用参数返回 price/ordType/level，Java 使用 double[1] 包装器。
     *
     * @param priceRef  priceRef[0] = 输出第二腿买价
     * @param ordTypeRef ordTypeRef[0] = 输出订单类型
     * @param levelRef  levelRef[0] = 输入 level
     */
    public void getBidPriceSecond(double[] priceRef, OrderStats.HitType[] ordTypeRef, int[] levelRef) {
        int level = levelRef[0];
        // C++: price = m_secondStrat->m_instru->bidPx[level];
        priceRef[0] = secondStrat.instru.bidPx[level];

        // C++: if (m_configParams->m_bUseInvisibleBook && level != 0 && price < m_secondStrat->m_instru->bidPx[level - 1] - m_secondStrat->m_instru->m_tickSize)
        if (configParams.bUseInvisibleBook && level != 0
                && priceRef[0] < secondStrat.instru.bidPx[level - 1] - secondStrat.instru.tickSize) {
            // C++: double bidInv = m_firstStrat->m_instru->bidPx[0] - m_secondStrat->m_instru->bidPx[level] - m_secondStrat->m_instru->m_tickSize;
            double bidInv = firstStrat.instru.bidPx[0] - secondStrat.instru.bidPx[level] - secondStrat.instru.tickSize;

            // C++: if (bidInv >= avgSpreadRatio + m_secondStrat->m_thold->BEGIN_PLACE)
            if (bidInv >= avgSpreadRatio + secondStrat.thold.BEGIN_PLACE) {
                // C++: PriceMapIter iter = m_bidMap2.find(price);
                OrderStats existing = bidMap2.get(priceRef[0]);
                // C++: if (iter != m_bidMap2.end() && iter->second->m_quantAhead > m_secondinstru->m_lotSize)
                if (existing != null && existing.quantAhead > secondinstru.lotSize) {
                    log.fine("3rd One"); // C++: TBLOG << "3rd One" << endl;
                    // C++: price = m_secondStrat->m_instru->bidPx[level] + m_secondStrat->m_instru->m_tickSize;
                    priceRef[0] = secondStrat.instru.bidPx[level] + secondStrat.instru.tickSize;
                }
            }
        }
    }

    /**
     * 获取第二腿卖价（含隐藏订单簿逻辑）。
     * 迁移自: PairwiseArbStrategy::GetAskPrice_second(double &price, OrderHitType &ordType, int32_t &level)
     * Ref: PairwiseArbStrategy.cpp:863-883
     *
     * @param priceRef  priceRef[0] = 输出第二腿卖价
     * @param ordTypeRef ordTypeRef[0] = 输出订单类型
     * @param levelRef  levelRef[0] = 输入 level
     */
    public void getAskPriceSecond(double[] priceRef, OrderStats.HitType[] ordTypeRef, int[] levelRef) {
        int level = levelRef[0];
        // C++: price = m_secondStrat->m_instru->askPx[level];
        priceRef[0] = secondStrat.instru.askPx[level];

        // C++: if (m_configParams->m_bUseInvisibleBook && level != 0 && price > m_secondStrat->m_instru->askPx[level - 1] + m_secondStrat->m_instru->m_tickSize)
        if (configParams.bUseInvisibleBook && level != 0
                && priceRef[0] > secondStrat.instru.askPx[level - 1] + secondStrat.instru.tickSize) {
            // C++: double askInv = m_firstStrat->m_instru->askPx[0] - m_secondStrat->m_instru->askPx[level] + m_instru->m_tickSize;
            double askInv = firstStrat.instru.askPx[0] - secondStrat.instru.askPx[level] + instru.tickSize;

            // C++: if (askInv <= avgSpreadRatio - m_secondStrat->m_thold->BEGIN_PLACE)
            if (askInv <= avgSpreadRatio - secondStrat.thold.BEGIN_PLACE) {
                // C++: PriceMapIter iter = m_askMap2.find(price);
                OrderStats existing = askMap2.get(priceRef[0]);
                // C++: if (iter != m_askMap2.end() && iter->second->m_quantAhead > m_secondinstru->m_lotSize)
                if (existing != null && existing.quantAhead > secondinstru.lotSize) {
                    log.fine("4th One"); // C++: TBLOG << "4th One" << endl;
                    // C++: price = m_secondStrat->m_instru->askPx[level] - m_secondStrat->m_instru->m_tickSize;
                    priceRef[0] = secondStrat.instru.askPx[level] - secondStrat.instru.tickSize;
                }
            }
        }
    }

    /**
     * 发送第一腿持仓到 TCache（共享变量缓存）。
     * 迁移自: PairwiseArbStrategy::SendTCacheLeg1Pos()
     * Ref: PairwiseArbStrategy.cpp:885-900
     *
     * [C++差异] C++ 使用 TCache（共享内存 KV store），Java 暂用日志输出占位。
     * TCache 功能需要后续实现对应的 Java 版本。
     */
    public void sendTCacheLeg1Pos() {
        // C++: if (!m_tcache) return;
        // [C++差异] Java 版本暂不实现 TCache 写入，仅记录日志
        try {
            // C++: m_tcache->store(std::to_string(m_strategyID) + "_pos_" + std::string(m_firstinstru->m_instrument), m_firstStrat->m_netpos_pass);
            String key = strategyID + "_pos_" + firstinstru.origBaseName;
            int pos = firstStrat.netposPass;
            log.info("Write Pos:" + key + " pos:" + firstinstru.origBaseName + " netPos:" + pos);
            // [C++差异-用户确认] TCache 为独立模块，不在当前策略迁移范围内。
            // 当 Java 版 TCache 可用时，在此处调用 tcache.store(key, pos)。
            // 参见 PairwiseArbStrategy.cpp:1320
        } catch (Exception e) {
            log.warning("Write Pos failed:" + e.getMessage());
        }
    }

    // =======================================================================
    //  辅助方法
    // =======================================================================

    /**
     * 找买单价格 Map 中已确认的最低价。
     */
    private double findLowestConfirmedBidPrice(Map<Double, OrderStats> bidPriceMap) {
        double lowest = 0;
        for (OrderStats order : bidPriceMap.values()) {
            if (order.status == OrderStats.Status.NEW_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_REJECT) {
                if (lowest == 0 || order.price < lowest) {
                    lowest = order.price;
                }
            }
        }
        return lowest;
    }

    /**
     * 找卖单价格 Map 中已确认的最高价。
     */
    private double findHighestConfirmedAskPrice(Map<Double, OrderStats> askPriceMap) {
        double highest = 0;
        for (OrderStats order : askPriceMap.values()) {
            if (order.status == OrderStats.Status.NEW_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_CONFIRM
                    || order.status == OrderStats.Status.MODIFY_REJECT) {
                if (highest == 0 || order.price > highest) {
                    highest = order.price;
                }
            }
        }
        return highest;
    }

    @Override
    public String toString() {
        return String.format("PairwiseArbStrategy[id=%d, spread=%.4f, avg=%.4f, firstNetpos=%d, secondNetpos=%d]",
                strategyID, currSpreadRatio, avgSpreadRatio,
                firstStrat != null ? firstStrat.netposPass : 0,
                secondStrat != null ? secondStrat.netposAgg : 0);
    }
}
