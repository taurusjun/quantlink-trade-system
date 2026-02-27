package com.quantlink.trader.core;

import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.*;

/**
 * Watch 全局时钟单例单元测试。
 * 验证与 C++ Watch 的行为一致性。
 */
class WatchTest {

    @BeforeEach
    void setup() {
        ConfigParams.resetInstance();
        Watch.resetInstance();
    }

    @AfterEach
    void cleanup() {
        Watch.resetInstance();
        ConfigParams.resetInstance();
    }

    // =======================================================================
    //  单例创建/获取
    // =======================================================================

    @Test
    void test_createInstance_returnsNewWatch() {
        Watch w = Watch.createInstance(0);
        assertNotNull(w);
        assertSame(w, Watch.getInstance());
    }

    @Test
    void test_createInstance_onlyCreatesOnce() {
        // C++: if (!unique_instance_) unique_instance_ = new Watch(t_time);
        Watch w1 = Watch.createInstance(100);
        Watch w2 = Watch.createInstance(200);
        assertSame(w1, w2);
        assertEquals(100, w1.getCurrentTime()); // 保留第一次创建的时间
    }

    @Test
    void test_getInstance_beforeCreate_returnsNull() {
        // C++: return unique_instance_ (初始为 NULL)
        assertNull(Watch.getInstance());
    }

    @Test
    void test_resetInstance_clearsState() {
        Watch.createInstance(100);
        assertNotNull(Watch.getInstance());
        Watch.resetInstance();
        assertNull(Watch.getInstance());
    }

    // =======================================================================
    //  构造函数初始化
    // =======================================================================

    @Test
    void test_constructor_initialValues() {
        Watch w = Watch.createInstance(500_000_000L);
        assertEquals(500_000_000L, w.getCurrentTime());
        // C++: next_update_time_ = t_time + MINTIMEINCREMENT
        // currentTimePrint = 0 → getCurrentTimePrint() 返回 System.nanoTime() (非零)
    }

    @Test
    void test_constructor_zeroTime() {
        Watch w = Watch.createInstance(0);
        assertEquals(0, w.getCurrentTime());
    }

    // =======================================================================
    //  updateTime — 单调递增保护
    // =======================================================================

    @Test
    void test_updateTime_forwardTime() {
        // C++: if (t_time > current_time_) current_time_ = t_time
        Watch w = Watch.createInstance(100);
        w.updateTime(200, "test");
        assertEquals(200, w.getCurrentTime());
    }

    @Test
    void test_updateTime_backwardTime_ignored() {
        // C++: if (t_time > current_time_ || t_time == 0) — 单调递增保护
        Watch w = Watch.createInstance(200);
        w.updateTime(100, "test");
        assertEquals(200, w.getCurrentTime()); // 不回退
    }

    @Test
    void test_updateTime_equalTime_ignored() {
        Watch w = Watch.createInstance(200);
        w.updateTime(200, "test");
        assertEquals(200, w.getCurrentTime()); // 等于时也不更新（> 不是 >=）
    }

    @Test
    void test_updateTime_zeroResets() {
        // C++: if (t_time > current_time_ || t_time == 0) — t_time==0 允许重置
        Watch w = Watch.createInstance(1000);
        w.updateTime(0, "reset");
        assertEquals(0, w.getCurrentTime());
    }

    @Test
    void test_updateTime_simMode_updatesTimePrint() {
        // C++: if (ModeType_Sim) current_time_print_ = t_time
        ConfigParams.getInstance().modeType = 1; // Sim mode
        Watch w = Watch.createInstance(0);
        w.updateTime(12345L, "test");
        assertEquals(12345L, w.getCurrentTimePrint());
    }

    @Test
    void test_updateTime_liveMode_doesNotUpdateTimePrint() {
        ConfigParams.getInstance().modeType = 2; // Live mode
        Watch w = Watch.createInstance(0);
        w.updateTime(12345L, "test");
        // currentTimePrint remains 0 → getCurrentTimePrint() returns System.nanoTime()
        // We can't test the exact value, but it should not be 12345
        assertNotEquals(12345L, w.getCurrentTimePrint());
    }

    // =======================================================================
    //  TimeListener — 每 1 秒触发
    // =======================================================================

    @Test
    void test_timeListener_triggeredAfterOneSecond() {
        // C++: MINTIMEINCREMENT = 10^9 ns = 1 sec
        Watch w = Watch.createInstance(0);
        AtomicInteger callCount = new AtomicInteger(0);
        w.subscribeTimeUpdates(callCount::incrementAndGet);

        // 更新到 0.5s — 不触发
        w.updateTime(500_000_000L, "test");
        assertEquals(0, callCount.get());

        // 更新到 1.1s — 触发（current > next_update = 0 + 10^9）
        w.updateTime(1_100_000_000L, "test");
        assertEquals(1, callCount.get());
    }

    @Test
    void test_timeListener_triggeredMultipleTimes() {
        Watch w = Watch.createInstance(0);
        AtomicInteger callCount = new AtomicInteger(0);
        w.subscribeTimeUpdates(callCount::incrementAndGet);

        // 1.1s — 第一次触发
        w.updateTime(1_100_000_000L, "test");
        assertEquals(1, callCount.get());

        // 1.5s — 不触发 (next = 1.1 + 1.0 = 2.1)
        w.updateTime(1_500_000_000L, "test");
        assertEquals(1, callCount.get());

        // 2.2s — 第二次触发
        w.updateTime(2_200_000_000L, "test");
        assertEquals(2, callCount.get());
    }

    @Test
    void test_timeListener_multipleListeners() {
        Watch w = Watch.createInstance(0);
        AtomicInteger count1 = new AtomicInteger(0);
        AtomicInteger count2 = new AtomicInteger(0);
        w.subscribeTimeUpdates(count1::incrementAndGet);
        w.subscribeTimeUpdates(count2::incrementAndGet);

        w.updateTime(1_100_000_000L, "test");
        assertEquals(1, count1.get());
        assertEquals(1, count2.get());
    }

    @Test
    void test_timeListener_notTriggeredOnBackwardTime() {
        Watch w = Watch.createInstance(2_000_000_000L);
        AtomicInteger callCount = new AtomicInteger(0);
        w.subscribeTimeUpdates(callCount::incrementAndGet);

        // 回退时间不触发
        w.updateTime(1_000_000_000L, "test");
        assertEquals(0, callCount.get());
    }

    // =======================================================================
    //  getTimeSlice
    // =======================================================================

    @Test
    void test_getTimeSlice() {
        Watch w = Watch.createInstance(0);
        assertEquals(1_000_000_000L, w.getTimeSlice());
    }

    // =======================================================================
    //  getNanoSecsFromEpoch
    // =======================================================================

    @Test
    void test_getNanoSecsFromEpoch_knownDate() {
        // 2026-02-27 00:00 UTC → 验证非零结果
        ConfigParams.getInstance().useExchTS = false;
        long result = Watch.getNanoSecsFromEpoch(20260227, 0);
        assertTrue(result > 0);
    }

    @Test
    void test_getNanoSecsFromEpoch_withTime() {
        ConfigParams.getInstance().useExchTS = false;
        long at0000 = Watch.getNanoSecsFromEpoch(20260227, 0);
        long at0100 = Watch.getNanoSecsFromEpoch(20260227, 100); // 01:00
        // 1 hour = 3600 * 10^9 ns
        assertEquals(3600_000_000_000L, at0100 - at0000);
    }

    @Test
    void test_getNanoSecsFromEpoch_useExchTS_offset() {
        // C++: if (m_bUseExchTS) retVal -= 315532800000000000
        ConfigParams.getInstance().useExchTS = false;
        long normal = Watch.getNanoSecsFromEpoch(20260227, 0);

        ConfigParams.getInstance().useExchTS = true;
        long withOffset = Watch.getNanoSecsFromEpoch(20260227, 0);

        assertEquals(315_532_800_000_000_000L, normal - withOffset);
    }
}
