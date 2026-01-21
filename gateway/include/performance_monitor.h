#pragma once

#include <atomic>
#include <chrono>
#include <string>
#include <vector>
#include <map>
#include <mutex>
#include <cmath>

namespace hft {
namespace perf {

// 延迟统计
struct LatencyStats {
    uint64_t count = 0;
    uint64_t sum_ns = 0;
    uint64_t min_ns = UINT64_MAX;
    uint64_t max_ns = 0;
    uint64_t p50_ns = 0;
    uint64_t p95_ns = 0;
    uint64_t p99_ns = 0;
    uint64_t p999_ns = 0;

    double GetAvg() const {
        return count > 0 ? static_cast<double>(sum_ns) / count : 0.0;
    }

    void AddSample(uint64_t latency_ns) {
        count++;
        sum_ns += latency_ns;
        if (latency_ns < min_ns) min_ns = latency_ns;
        if (latency_ns > max_ns) max_ns = latency_ns;
    }
};

// 吞吐量统计
struct ThroughputStats {
    uint64_t total_count = 0;
    uint64_t interval_count = 0;
    double instant_rate = 0.0;  // msg/s
    double average_rate = 0.0;  // msg/s
    std::chrono::steady_clock::time_point start_time;
    std::chrono::steady_clock::time_point last_update;

    ThroughputStats() {
        start_time = std::chrono::steady_clock::now();
        last_update = start_time;
    }

    void AddMessage() {
        total_count++;
        interval_count++;
    }

    void UpdateRates() {
        auto now = std::chrono::steady_clock::now();
        auto total_duration = std::chrono::duration<double>(now - start_time).count();
        auto interval_duration = std::chrono::duration<double>(now - last_update).count();

        if (total_duration > 0) {
            average_rate = total_count / total_duration;
        }

        if (interval_duration > 0) {
            instant_rate = interval_count / interval_duration;
            interval_count = 0;
            last_update = now;
        }
    }
};

// 性能监控器
class PerformanceMonitor {
public:
    PerformanceMonitor(const std::string& name, size_t histogram_size = 10000)
        : m_name(name), m_histogram_size(histogram_size) {
        m_latency_samples.reserve(histogram_size);
    }

    // 记录延迟样本
    void RecordLatency(uint64_t latency_ns) {
        std::lock_guard<std::mutex> lock(m_mutex);

        m_latency_stats.AddSample(latency_ns);

        // 保存样本用于计算百分位数
        if (m_latency_samples.size() < m_histogram_size) {
            m_latency_samples.push_back(latency_ns);
        } else {
            // 环形缓冲区
            m_latency_samples[m_sample_index % m_histogram_size] = latency_ns;
            m_sample_index++;
        }
    }

    // 记录消息
    void RecordMessage() {
        m_throughput_stats.AddMessage();
    }

    // 更新统计
    void Update() {
        std::lock_guard<std::mutex> lock(m_mutex);

        // 更新吞吐量
        m_throughput_stats.UpdateRates();

        // 计算百分位数
        if (!m_latency_samples.empty()) {
            std::vector<uint64_t> sorted = m_latency_samples;
            std::sort(sorted.begin(), sorted.end());

            size_t n = sorted.size();
            m_latency_stats.p50_ns = sorted[n * 50 / 100];
            m_latency_stats.p95_ns = sorted[n * 95 / 100];
            m_latency_stats.p99_ns = sorted[n * 99 / 100];
            m_latency_stats.p999_ns = sorted[std::min(n * 999 / 1000, n - 1)];
        }
    }

    // 获取统计信息
    LatencyStats GetLatencyStats() const {
        std::lock_guard<std::mutex> lock(m_mutex);
        return m_latency_stats;
    }

    ThroughputStats GetThroughputStats() const {
        return m_throughput_stats;
    }

    // 打印报告
    void PrintReport() const {
        std::lock_guard<std::mutex> lock(m_mutex);

        printf("\n╔══════════════════════════════════════════════════════╗\n");
        printf("║  Performance Report: %-31s ║\n", m_name.c_str());
        printf("╠══════════════════════════════════════════════════════╣\n");

        // 延迟统计
        printf("║ Latency Statistics:                                  ║\n");
        printf("║   Count:      %-38llu ║\n", m_latency_stats.count);
        printf("║   Avg:        %-33.2f μs ║\n", m_latency_stats.GetAvg() / 1000.0);
        printf("║   Min:        %-33.2f μs ║\n", m_latency_stats.min_ns / 1000.0);
        printf("║   Max:        %-33.2f μs ║\n", m_latency_stats.max_ns / 1000.0);
        printf("║   P50:        %-33.2f μs ║\n", m_latency_stats.p50_ns / 1000.0);
        printf("║   P95:        %-33.2f μs ║\n", m_latency_stats.p95_ns / 1000.0);
        printf("║   P99:        %-33.2f μs ║\n", m_latency_stats.p99_ns / 1000.0);
        printf("║   P999:       %-33.2f μs ║\n", m_latency_stats.p999_ns / 1000.0);
        printf("╠══════════════════════════════════════════════════════╣\n");

        // 吞吐量统计
        printf("║ Throughput Statistics:                               ║\n");
        printf("║   Total Messages: %-35llu ║\n", m_throughput_stats.total_count);
        printf("║   Instant Rate:   %-28.2f msg/s ║\n", m_throughput_stats.instant_rate);
        printf("║   Average Rate:   %-28.2f msg/s ║\n", m_throughput_stats.average_rate);
        printf("╚══════════════════════════════════════════════════════╝\n\n");
    }

    // 重置统计
    void Reset() {
        std::lock_guard<std::mutex> lock(m_mutex);
        m_latency_stats = LatencyStats();
        m_throughput_stats = ThroughputStats();
        m_latency_samples.clear();
        m_sample_index = 0;
    }

private:
    std::string m_name;
    size_t m_histogram_size;
    size_t m_sample_index = 0;

    mutable std::mutex m_mutex;
    LatencyStats m_latency_stats;
    ThroughputStats m_throughput_stats;
    std::vector<uint64_t> m_latency_samples;
};

} // namespace perf
} // namespace hft
