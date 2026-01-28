#pragma once

#include <atomic>
#include <cstring>
#include <sys/mman.h>
#include <sys/stat.h>
#include <fcntl.h>
#include <unistd.h>
#include <string>
#include <stdexcept>

namespace hft {
namespace shm {

// 共享内存中的行情数据结构（简化版）
struct MarketDataRaw {
    char symbol[16];        // 合约代码
    char exchange[8];       // 交易所
    uint64_t timestamp;     // 时间戳（纳秒）

    double bid_price[10];   // 买价
    uint32_t bid_qty[10];   // 买量
    double ask_price[10];   // 卖价
    uint32_t ask_qty[10];   // 卖量

    double last_price;      // 最新价
    uint32_t last_qty;      // 最新量
    uint64_t total_volume;  // 总成交量

    uint64_t seq_num;       // 序列号（用于检测丢失）
};

// 无锁环形队列（单生产者单消费者）
template<typename T, size_t Size>
class SPSCQueue {
public:
    SPSCQueue() : m_head(0), m_tail(0) {}

    // 生产者：写入数据
    bool Push(const T& item) {
        size_t current_tail = m_tail.load(std::memory_order_relaxed);
        size_t next_tail = (current_tail + 1) % Size;

        // 队列满
        if (next_tail == m_head.load(std::memory_order_acquire)) {
            return false;
        }

        m_buffer[current_tail] = item;
        m_tail.store(next_tail, std::memory_order_release);
        return true;
    }

    // 消费者：读取数据
    bool Pop(T& item) {
        size_t current_head = m_head.load(std::memory_order_relaxed);

        // 队列空
        if (current_head == m_tail.load(std::memory_order_acquire)) {
            return false;
        }

        item = m_buffer[current_head];
        m_head.store((current_head + 1) % Size, std::memory_order_release);
        return true;
    }

    // 获取队列中元素数量（近似）
    size_t GetSize() const {
        size_t head = m_head.load(std::memory_order_acquire);
        size_t tail = m_tail.load(std::memory_order_acquire);
        if (tail >= head) {
            return tail - head;
        }
        return Size - head + tail;
    }

    bool Empty() const {
        return m_head.load(std::memory_order_acquire) ==
               m_tail.load(std::memory_order_acquire);
    }

private:
    alignas(64) std::atomic<size_t> m_head;  // 消费者索引（缓存行对齐）
    alignas(64) std::atomic<size_t> m_tail;  // 生产者索引
    T m_buffer[Size];
};

// 共享内存管理器
class ShmManager {
public:
    static constexpr size_t QUEUE_SIZE = 4096;  // 队列大小（保持原值，另寻方案优化）
    using Queue = SPSCQueue<MarketDataRaw, QUEUE_SIZE>;

    // 创建共享内存（生产者使用）
    static Queue* Create(const std::string& name) {
        std::string shm_name = "/hft_md_" + name;

        // 创建共享内存
        int fd = shm_open(shm_name.c_str(), O_CREAT | O_RDWR, 0666);
        if (fd == -1) {
            throw std::runtime_error("Failed to create shared memory: " + shm_name);
        }

        // 设置大小
        if (ftruncate(fd, sizeof(Queue)) == -1) {
            close(fd);
            throw std::runtime_error("Failed to set shared memory size");
        }

        // 映射到进程地址空间
        void* addr = mmap(nullptr, sizeof(Queue), PROT_READ | PROT_WRITE,
                         MAP_SHARED, fd, 0);
        close(fd);

        if (addr == MAP_FAILED) {
            throw std::runtime_error("Failed to map shared memory");
        }

        // 使用placement new初始化
        Queue* queue = new (addr) Queue();
        return queue;
    }

    // 创建或打开共享内存（支持任意启动顺序）
    static Queue* CreateOrOpen(const std::string& name, bool* is_creator = nullptr) {
        std::string shm_name = "/hft_md_" + name;

        // 先尝试创建（O_EXCL确保只有第一个创建成功）
        int fd = shm_open(shm_name.c_str(), O_CREAT | O_EXCL | O_RDWR, 0666);
        bool created = (fd != -1);

        if (!created) {
            // 创建失败（已存在），则打开
            fd = shm_open(shm_name.c_str(), O_RDWR, 0666);
            if (fd == -1) {
                throw std::runtime_error("Failed to open shared memory: " + shm_name);
            }
        } else {
            // 创建成功，设置大小
            if (ftruncate(fd, sizeof(Queue)) == -1) {
                close(fd);
                shm_unlink(shm_name.c_str());
                throw std::runtime_error("Failed to set shared memory size");
            }
        }

        // 映射到进程地址空间
        void* addr = mmap(nullptr, sizeof(Queue), PROT_READ | PROT_WRITE,
                         MAP_SHARED, fd, 0);
        close(fd);

        if (addr == MAP_FAILED) {
            throw std::runtime_error("Failed to map shared memory");
        }

        if (is_creator) {
            *is_creator = created;
        }

        // 如果是创建者，使用placement new初始化
        if (created) {
            return new (addr) Queue();
        } else {
            return reinterpret_cast<Queue*>(addr);
        }
    }

    // 打开共享内存（消费者使用）
    static Queue* Open(const std::string& name) {
        std::string shm_name = "/hft_md_" + name;

        // 打开已存在的共享内存
        int fd = shm_open(shm_name.c_str(), O_RDWR, 0666);
        if (fd == -1) {
            throw std::runtime_error("Failed to open shared memory: " + shm_name);
        }

        // 映射到进程地址空间
        void* addr = mmap(nullptr, sizeof(Queue), PROT_READ | PROT_WRITE,
                         MAP_SHARED, fd, 0);
        close(fd);

        if (addr == MAP_FAILED) {
            throw std::runtime_error("Failed to map shared memory");
        }

        return reinterpret_cast<Queue*>(addr);
    }

    // 关闭共享内存
    static void Close(Queue* queue) {
        if (queue) {
            munmap(queue, sizeof(Queue));
        }
    }

    // 删除共享内存（清理）
    static void Remove(const std::string& name) {
        std::string shm_name = "/hft_md_" + name;
        shm_unlink(shm_name.c_str());
    }
};

} // namespace shm
} // namespace hft
