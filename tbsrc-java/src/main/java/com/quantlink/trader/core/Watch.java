package com.quantlink.trader.core;

import java.util.ArrayList;
import java.util.Calendar;
import java.util.List;
import java.util.TimeZone;
import java.util.logging.Logger;

/**
 * 全局行情驱动时钟单例。
 * 迁移自: tbsrc/common/include/Watch.hpp (line 1-57)
 *         tbsrc/common/Watch.cpp (line 1-69)
 *
 * C++ class Watch — 核心方法:
 *   Watch(uint64_t t_time)
 *   CreateUniqueInstance(uint64_t t_time), GetUniqueInstance()
 *   GetCurrentTime(), GetCurrentTimePrint()
 *   UpdateTime(uint64_t t_time, string debugstr)
 *   SubscribeTimeUpdates(TimeListener*), UpdateTimeListeners()
 *   GetNanoSecsFromEpoch(uint64_t t_date, uint64_t t_time)
 *
 * C++ 调用链:
 *   1. main.cpp:650 — Watch::CreateUniqueInstance(0)
 *   2. CommonClient.cpp:412-415 — Watch::GetUniqueInstance()->UpdateTime(exchTS * 1000000, symbol)
 *   3. ExecutionStrategy.cpp / PairwiseArbStrategy.cpp — Watch::GetUniqueInstance()->GetCurrentTime()
 */
public class Watch {

    private static final Logger log = Logger.getLogger(Watch.class.getName());

    // ---- 单例 ----
    // 迁移自: Watch.hpp:20 — static Watch *unique_instance_
    private static Watch instance;

    // ---- 时间字段 ----
    // 迁移自: Watch.hpp:17-19
    // C++: uint64_t current_time_
    private long currentTime;
    // C++: uint64_t current_time_print_
    private long currentTimePrint;
    // C++: uint64_t next_update_time_
    private long nextUpdateTime;

    // ---- TimeListener 列表 ----
    // 迁移自: Watch.hpp:21 — vector<TimeListener*> time_listener_vector
    private final List<TimeListener> listeners = new ArrayList<>();

    // ---- 常量 ----
    // 迁移自: Watch.hpp:6 — #define MINTIMEINCREMENT 1000000000  // 1 sec = 10^9 nanos
    static final long MIN_TIME_INCREMENT = 1_000_000_000L;
    // 迁移自: Watch.hpp:7 — #define BIGTIMEINCREMENT 10000000000 // 10 sec
    static final long BIG_TIME_INCREMENT = 10_000_000_000L;

    /**
     * TimeListener 接口。
     * 迁移自: Watch.hpp:9-13 — class TimeListener { virtual void OnTimeUpdate() {} }
     */
    public interface TimeListener {
        void onTimeUpdate();
    }

    /**
     * 构造函数。
     * 迁移自: Watch.cpp:7-13
     *
     * C++: Watch::Watch(uint64_t t_time) {
     *          time_listener_vector.clear();
     *          next_update_time_ = t_time + MINTIMEINCREMENT;
     *          current_time_print_ = 0;
     *          current_time_ = t_time;
     *      }
     */
    public Watch(long time) {
        // C++: time_listener_vector.clear()
        listeners.clear();
        // C++: next_update_time_ = t_time + MINTIMEINCREMENT
        nextUpdateTime = time + MIN_TIME_INCREMENT;
        // C++: current_time_print_ = 0
        currentTimePrint = 0;
        // C++: current_time_ = t_time
        currentTime = time;
    }

    /**
     * 创建全局单例。
     * 迁移自: Watch.cpp:15-20
     *
     * C++: Watch *Watch::CreateUniqueInstance(uint64_t t_time) {
     *          if (!unique_instance_) unique_instance_ = new Watch(t_time);
     *          return unique_instance_;
     *      }
     */
    public static synchronized Watch createInstance(long time) {
        if (instance == null) {
            instance = new Watch(time);
        }
        return instance;
    }

    /**
     * 获取全局单例。
     * 迁移自: Watch.cpp:22-25
     *
     * C++: Watch *Watch::GetUniqueInstance() { return unique_instance_; }
     */
    public static Watch getInstance() {
        return instance;
    }

    /**
     * 重置单例（测试用）。
     * C++ 中无对应方法 — Java 测试需要在 @BeforeEach 中重置全局状态。
     */
    public static void resetInstance() {
        instance = null;
    }

    /**
     * 获取当前时间。
     * 迁移自: Watch.hpp:40-43
     *
     * C++: uint64_t GetCurrentTime() { return current_time_; }
     */
    public long getCurrentTime() {
        return currentTime;
    }

    /**
     * 获取打印用时间。
     * 迁移自: Watch.hpp:45-48
     *
     * C++: uint64_t GetCurrentTimePrint() {
     *          return current_time_print_ != 0 ? current_time_print_ : getcurtime();
     *      }
     *
     * [C++差异] C++ 使用 getcurtime() 返回系统当前纳秒时间。
     * Java 使用 System.nanoTime() 作为等价实现。
     */
    public long getCurrentTimePrint() {
        return currentTimePrint != 0 ? currentTimePrint : System.nanoTime();
    }

    /**
     * 获取时间片大小。
     * 迁移自: Watch.hpp:30-33
     *
     * C++: uint64_t GetTimeSlice() { return MINTIMEINCREMENT; }
     */
    public long getTimeSlice() {
        return MIN_TIME_INCREMENT;
    }

    /**
     * 更新时钟（行情驱动）。
     * 迁移自: Watch.cpp:27-44
     *
     * C++: void Watch::UpdateTime(uint64_t t_time, string debugstr) {
     *          if (t_time > current_time_ || t_time == 0) {
     *              current_time_ = t_time;
     *              if (ModeType_Sim) current_time_print_ = t_time;
     *          }
     *          if (current_time_ > next_update_time_) {
     *              UpdateTimeListeners();
     *              next_update_time_ = current_time_ + MINTIMEINCREMENT;
     *          }
     *      }
     *
     * @param time     纳秒 epoch 时间戳
     * @param debugStr 调试信息（通常为 symbol 名称）
     */
    public void updateTime(long time, String debugStr) {
        // C++: if (t_time > current_time_ || t_time == 0)
        // Ref: Watch.cpp:32
        if (time > currentTime || time == 0) {
            // C++: current_time_ = t_time
            currentTime = time;
            // C++: if (ConfigParams::GetInstance()->m_modeType == ModeType_Sim)
            //          current_time_print_ = t_time;
            // Ref: Watch.cpp:35-36
            if (ConfigParams.getInstance().modeType == 1) { // ModeType_Sim = 1
                currentTimePrint = time;
            }
        }

        // C++: if (current_time_ > next_update_time_)
        // Ref: Watch.cpp:39
        if (currentTime > nextUpdateTime) {
            // C++: UpdateTimeListeners()
            updateTimeListeners();
            // C++: next_update_time_ = current_time_ + MINTIMEINCREMENT
            nextUpdateTime = currentTime + MIN_TIME_INCREMENT;
        }
    }

    /**
     * 订阅时间更新通知（每 1 秒触发一次）。
     * 迁移自: Watch.hpp:36-39
     *
     * C++: void SubscribeTimeUpdates(TimeListener *t_listener) {
     *          time_listener_vector.push_back(t_listener);
     *      }
     */
    public void subscribeTimeUpdates(TimeListener listener) {
        listeners.add(listener);
    }

    /**
     * 通知所有 TimeListener。
     * 迁移自: Watch.hpp:22-26
     *
     * C++: void UpdateTimeListeners() {
     *          for (unsigned int i = 0; i < time_listener_vector.size(); ++i)
     *              time_listener_vector[i]->OnTimeUpdate();
     *      }
     */
    private void updateTimeListeners() {
        for (TimeListener listener : listeners) {
            listener.onTimeUpdate();
        }
    }

    /**
     * 日期+时间 → 纳秒 epoch 转换。
     * 迁移自: Watch.cpp:46-64
     *
     * C++: uint64_t Watch::GetNanoSecsFromEpoch(uint64_t t_date, uint64_t t_time) {
     *          // t_date as yyyymmdd in int, t_time as GMT hhmm in int
     *          struct tm time_info;
     *          time_info.tm_year = t_date / 10000 - 1900;
     *          time_info.tm_mon = (t_date / 100) % 100 - 1;
     *          time_info.tm_mday = t_date % 100;
     *          time_info.tm_hour = t_time / 100;
     *          time_info.tm_min = t_time % 100;
     *          time_info.tm_sec = 0;
     *          time_info.tm_isdst = 0;
     *          retVal = ((int64_t)(mktime(&time_info) - timezone) * (int64_t)(1000000000));
     *          if (m_bUseExchTS) retVal -= 315532800000000000;
     *      }
     *
     * [C++差异] C++ 使用 mktime (local time) 然后减去 timezone 偏移得到 UTC epoch。
     * Java 使用 Calendar(UTC) 直接计算 UTC epoch，语义等价。
     *
     * @param date yyyymmdd 格式的日期整数
     * @param time GMT hhmm 格式的时间整数
     * @return 纳秒 epoch
     */
    public static long getNanoSecsFromEpoch(long date, long time) {
        // C++: time_info.tm_year = t_date / 10000 - 1900
        int year = (int) (date / 10000);
        // C++: time_info.tm_mon = (t_date / 100) % 100 - 1
        int month = (int) ((date / 100) % 100);
        // C++: time_info.tm_mday = t_date % 100
        int day = (int) (date % 100);
        // C++: time_info.tm_hour = t_time / 100
        int hour = (int) (time / 100);
        // C++: time_info.tm_min = t_time % 100
        int min = (int) (time % 100);

        // C++: mktime(&time_info) - timezone → UTC epoch seconds
        // Java: 直接使用 UTC Calendar
        Calendar cal = Calendar.getInstance(TimeZone.getTimeZone("UTC"));
        cal.set(Calendar.YEAR, year);
        cal.set(Calendar.MONTH, month - 1); // Calendar.MONTH is 0-based
        cal.set(Calendar.DAY_OF_MONTH, day);
        cal.set(Calendar.HOUR_OF_DAY, hour);
        cal.set(Calendar.MINUTE, min);
        cal.set(Calendar.SECOND, 0);
        cal.set(Calendar.MILLISECOND, 0);

        // C++: retVal = (mktime - timezone) * 1000000000
        long retVal = cal.getTimeInMillis() / 1000 * 1_000_000_000L;

        // C++: if (ConfigParams::GetInstance()->m_bUseExchTS) retVal -= 315532800000000000;
        // Ref: Watch.cpp:60-61
        if (ConfigParams.getInstance().useExchTS) {
            retVal -= 315_532_800_000_000_000L;
        }

        log.info("Watch: " + date + " " + time + " " + retVal);
        return retVal;
    }
}
