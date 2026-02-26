package com.quantlink.trader.api;

import java.time.LocalTime;
import java.time.format.DateTimeFormatter;
import java.util.*;

/**
 * 订单历史追踪器 — 保留最近 N 条订单（包括已完成的）。
 * 对齐: tbsrc-golang/pkg/api/snapshot.go — OrderHistoryTracker
 *
 * OrdMap 中的订单在成交后会被移除，模拟器填单极快（~150ms），
 * 但快照每秒才采集一次，大部分订单会被错过。
 * 此追踪器维护一个环形缓冲区，保留最近的订单。
 */
public class OrderHistoryTracker {

    private static final DateTimeFormatter TIME_FMT = DateTimeFormatter.ofPattern("HH:mm:ss");

    private final List<DashboardSnapshot.OrderSnapshot> history;
    private final int maxSize;
    private final Set<Integer> seen; // 已经记录过的 orderID

    public OrderHistoryTracker(int maxSize) {
        this.maxSize = maxSize;
        this.history = new ArrayList<>(maxSize);
        this.seen = new HashSet<>();
    }

    /**
     * 接收当前快照中的活跃订单，合并到历史中，返回完整列表（最新在前）。
     * 对齐: tbsrc-golang/pkg/api/snapshot.go:OrderHistoryTracker.Update()
     */
    public List<DashboardSnapshot.OrderSnapshot> update(List<DashboardSnapshot.OrderSnapshot> liveOrders) {
        String now = LocalTime.now().format(TIME_FMT);

        if (liveOrders == null) liveOrders = List.of();

        // 更新已有订单状态，添加新订单
        for (DashboardSnapshot.OrderSnapshot o : liveOrders) {
            o.time = now;
            if (!seen.contains(o.orderID)) {
                seen.add(o.orderID);
                history.add(copyOrder(o));
            } else {
                // 已有订单，更新状态
                for (int i = 0; i < history.size(); i++) {
                    if (history.get(i).orderID == o.orderID) {
                        history.set(i, copyOrder(o));
                        break;
                    }
                }
            }
        }

        // 标记不在 liveOrders 中的订单为 TRADED
        // 对齐 Go: 已从 OrdMap 移除 = 已成交
        Set<Integer> liveSet = new HashSet<>();
        for (DashboardSnapshot.OrderSnapshot o : liveOrders) {
            liveSet.add(o.orderID);
        }
        for (DashboardSnapshot.OrderSnapshot h : history) {
            if (!liveSet.contains(h.orderID)
                    && !"TRADED".equals(h.status)
                    && !"CANCEL_CONFIRM".equals(h.status)
                    && !"NEW_REJECT".equals(h.status)) {
                h.status = "TRADED";
                h.time = now;
            }
        }

        // 裁剪：保留最近 maxSize 条
        if (history.size() > maxSize) {
            int removeCount = history.size() - maxSize;
            for (int i = 0; i < removeCount; i++) {
                seen.remove(history.get(i).orderID);
            }
            history.subList(0, removeCount).clear();
        }

        // 返回倒序副本（最新在前）
        List<DashboardSnapshot.OrderSnapshot> result = new ArrayList<>(history.size());
        for (int i = history.size() - 1; i >= 0; i--) {
            result.add(copyOrder(history.get(i)));
        }
        return result;
    }

    private static DashboardSnapshot.OrderSnapshot copyOrder(DashboardSnapshot.OrderSnapshot src) {
        DashboardSnapshot.OrderSnapshot dst = new DashboardSnapshot.OrderSnapshot();
        dst.orderID = src.orderID;
        dst.side = src.side;
        dst.price = src.price;
        dst.openQty = src.openQty;
        dst.doneQty = src.doneQty;
        dst.status = src.status;
        dst.ordType = src.ordType;
        dst.time = src.time;
        return dst;
    }
}
