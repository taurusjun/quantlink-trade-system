// hftbase_shm.h — SysV MWMR queue (binary-compatible with hftbase)
//
// Minimal reimplementation of hftbase SysV shared memory and
// MultiWriterMultiReaderShmQueue, without depending on hftbase itself.
// Memory layout is 100% compatible with:
//   - hftbase/Ipc/include/multiwritermultireadershmqueue.h
//   - tbsrc-golang/pkg/shm/mwmr_queue.go
//
// C++ sources:
//   hftbase/Ipc/include/sharedmemory.h
//   hftbase/Ipc/include/multiwritermultireadershmqueue.h
//   hftbase/Ipc/include/locklessshmclientstore.h

#pragma once

#include <atomic>
#include <cstring>
#include <cstdint>
#include <sys/ipc.h>
#include <sys/shm.h>
#include <unistd.h>
#include <stdexcept>
#include <iostream>

namespace hftbase_compat {

// ============================================================
// 1. SysV shared memory wrapper
// ============================================================

// Create SysV SHM segment (server side — counter_bridge calls this)
// C++ source: hftbase/Ipc/include/sharedmemory.h
inline void* shm_create(int key, size_t size) {
    // Page-align (same as hftbase)
    long page_size = sysconf(_SC_PAGESIZE);
    if (page_size > 0) {
        size = size + page_size - (size % page_size);
    }

    int shmid = shmget(key, size, IPC_CREAT | 0666);
    if (shmid < 0) {
        throw std::runtime_error("shmget create failed, key=" + std::to_string(key));
    }
    void* addr = shmat(shmid, nullptr, 0);
    if (addr == (void*)-1) {
        throw std::runtime_error("shmat failed, key=" + std::to_string(key));
    }
    return addr;
}

// Open existing SysV SHM segment (client side)
inline void* shm_open_existing(int key, size_t size) {
    long page_size = sysconf(_SC_PAGESIZE);
    if (page_size > 0) {
        size = size + page_size - (size % page_size);
    }

    int shmid = shmget(key, size, 0666);
    if (shmid < 0) {
        throw std::runtime_error("shmget open failed, key=" + std::to_string(key));
    }
    void* addr = shmat(shmid, nullptr, 0);
    if (addr == (void*)-1) {
        throw std::runtime_error("shmat failed, key=" + std::to_string(key));
    }
    return addr;
}

inline void shm_detach(void* addr) {
    if (addr && addr != (void*)-1) {
        shmdt(addr);
    }
}

// ============================================================
// 2. MWMR Queue (binary-compatible with hftbase)
// ============================================================

// C++ source: hftbase/Ipc/include/multiwritermultireadershmqueue.h

// Header: 8 bytes, initial value = 1
// C++ source: MultiWriterMultiReaderShmHeader
struct MWMRHeader {
    std::atomic<int64_t> head;
};

// QueueElem: data first, seqNo last
// C++ source: QueueElem<T>
template<typename T>
struct QueueElem {
    T data;
    uint64_t seqNo;
};

// Round up to next power of 2
// C++ source: getMinHighestPowOf2()
inline int64_t next_pow2(int64_t v) {
    if (v <= 0) return 1;
    v--;
    v |= v >> 1;
    v |= v >> 2;
    v |= v >> 4;
    v |= v >> 8;
    v |= v >> 16;
    v |= v >> 32;
    return v + 1;
}

template<typename T>
class MWMRQueue {
public:
    // Create queue (server side — creates SHM)
    static MWMRQueue<T>* Create(int shmkey, int64_t requested_size) {
        int64_t size = next_pow2(requested_size);
        size_t total = sizeof(MWMRHeader) + size * sizeof(QueueElem<T>);

        void* addr = shm_create(shmkey, total);
        auto* q = new MWMRQueue<T>();
        q->init(addr, size);

        // Initialize header: head = 1 (C++ ctor)
        q->header()->head.store(1, std::memory_order_relaxed);
        std::memset(q->m_updates, 0, size * sizeof(QueueElem<T>));

        std::cout << "[MWMR] Created queue: key=0x" << std::hex << shmkey << std::dec
                  << " size=" << size << " elemSize=" << sizeof(QueueElem<T>)
                  << " totalBytes=" << total << std::endl;

        return q;
    }

    // Open existing queue (client side)
    static MWMRQueue<T>* Open(int shmkey, int64_t requested_size) {
        int64_t size = next_pow2(requested_size);
        size_t total = sizeof(MWMRHeader) + size * sizeof(QueueElem<T>);

        void* addr = shm_open_existing(shmkey, total);
        auto* q = new MWMRQueue<T>();
        q->init(addr, size);

        // tail catches up to current head (skip history)
        q->m_tail = q->header()->head.load(std::memory_order_relaxed);

        return q;
    }

    // Enqueue — multi-producer safe
    // C++ source: multiwritermultireadershmqueue.h:118-133
    void enqueue(const T& value) {
        int64_t myHead = header()->head.fetch_add(1, std::memory_order_acq_rel);
        QueueElem<T>* slot = m_updates + (myHead & m_mask);
        std::memcpy(&(slot->data), &value, sizeof(T));
        asm volatile("" ::: "memory");  // compiler barrier
        slot->seqNo = myHead;
    }

    // IsEmpty — check if new data is available
    // C++ source: multiwritermultireadershmqueue.h:245-249
    bool isEmpty() const {
        return (m_updates + (m_tail & m_mask))->seqNo < (uint64_t)m_tail;
    }

    // Dequeue — single consumer mode
    // C++ source: multiwritermultireadershmqueue.h:204-211
    void dequeuePtr(T* data) {
        QueueElem<T>* slot = m_updates + (m_tail & m_mask);
        std::memcpy(data, &(slot->data), sizeof(T));
        m_tail = slot->seqNo + 1;
    }

    void close() {
        if (m_base) {
            shm_detach(m_base);
            m_base = nullptr;
        }
    }

private:
    void init(void* addr, int64_t size) {
        m_base = addr;
        m_updates = reinterpret_cast<QueueElem<T>*>(
            static_cast<char*>(addr) + sizeof(MWMRHeader));
        m_size = size;
        m_mask = size - 1;
        m_tail = 1;  // default; Open() overrides
    }

    MWMRHeader* header() {
        return reinterpret_cast<MWMRHeader*>(m_base);
    }

    const MWMRHeader* header() const {
        return reinterpret_cast<const MWMRHeader*>(m_base);
    }

    void* m_base = nullptr;
    QueueElem<T>* m_updates = nullptr;
    int64_t m_size = 0;
    int64_t m_mask = 0;
    int64_t m_tail = 1;  // process-local, not in SHM
};

// ============================================================
// 3. ClientStore (binary-compatible with hftbase LocklessShmClientStore)
// ============================================================

// C++ source: hftbase/Ipc/include/locklessshmclientstore.h
struct ClientStoreData {
    std::atomic<int64_t> data;     // current counter
    int64_t firstClientId;         // initial value
};

class ClientStore {
public:
    static ClientStore* Create(int shmkey, int64_t initial_value = 0) {
        void* addr = shm_create(shmkey, sizeof(ClientStoreData));
        auto* cs = new ClientStore();
        cs->m_data = reinterpret_cast<ClientStoreData*>(addr);
        cs->m_data->data.store(initial_value, std::memory_order_relaxed);
        cs->m_data->firstClientId = initial_value;

        std::cout << "[ClientStore] Created: key=0x" << std::hex << shmkey << std::dec
                  << " initial=" << initial_value << std::endl;
        return cs;
    }

    static ClientStore* Open(int shmkey) {
        void* addr = shm_open_existing(shmkey, sizeof(ClientStoreData));
        auto* cs = new ClientStore();
        cs->m_data = reinterpret_cast<ClientStoreData*>(addr);
        return cs;
    }

    int64_t getClientIdAndIncrement() {
        return m_data->data.fetch_add(1, std::memory_order_acq_rel);
    }

    int64_t getClientId() const {
        return m_data->data.load(std::memory_order_acquire);
    }

    void close() {
        // No-op: detach handled externally or at process exit
    }

private:
    ClientStoreData* m_data = nullptr;
};

} // namespace hftbase_compat
