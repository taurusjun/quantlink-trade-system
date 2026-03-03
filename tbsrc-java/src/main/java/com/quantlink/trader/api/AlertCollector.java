package com.quantlink.trader.api;

import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.ConcurrentLinkedDeque;
import java.util.concurrent.atomic.AtomicInteger;

/**
 * 告警事件环形缓冲区。
 * 线程安全，固定容量 100 条，超出时丢弃最旧事件。
 * add() 为 O(1)，不做任何 I/O。
 */
public class AlertCollector {

    private static final int MAX_CAPACITY = 100;

    private final ConcurrentLinkedDeque<AlertEvent> events = new ConcurrentLinkedDeque<>();
    private final AtomicInteger size = new AtomicInteger(0);

    /**
     * 添加告警事件。O(1) 操作，线程安全。
     */
    public void add(AlertEvent event) {
        events.addLast(event);
        if (size.incrementAndGet() > MAX_CAPACITY) {
            events.pollFirst();
            size.decrementAndGet();
        }
    }

    /**
     * 返回全量事件列表（按时间升序）。
     */
    public List<AlertEvent> getAll() {
        return new ArrayList<>(events);
    }

    /**
     * 当前事件数量。
     */
    public int count() {
        return size.get();
    }
}
