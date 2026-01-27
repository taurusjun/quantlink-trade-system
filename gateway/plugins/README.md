# Gateway 插件开发指南

本目录包含Gateway层的插件实现。每个交易机构（券商/期货公司）作为独立插件，通过统一接口与核心系统交互。

## 插件架构概览

```
gateway/
├── include/plugin/          # 插件统一接口
│   └── md_plugin_interface.h   # 行情插件接口
├── plugins/                 # 各机构插件实现
│   ├── ctp/                # CTP插件 (已完成)
│   ├── xtp/                # XTP插件 (计划中)
│   └── femas/              # 飞马插件 (计划中)
└── CMakeLists.txt          # 插件编译配置
```

## 已实现的插件

### CTP插件 (ctp_md_plugin)

**状态**: ✅ 已完成

**功能**:
- CTP行情接入
- 共享内存数据发布
- 断线重连
- 延迟监控

**编译**:
```bash
cd gateway/build
cmake .. -DBUILD_CTP_PLUGIN=ON
make ctp_md_plugin -j4
```

**运行**:
```bash
./plugins/ctp/ctp_md_plugin -c config/ctp/ctp_md.yaml
```

**配置文件**:
- `config/ctp/ctp_md.yaml` - 主配置（交易所地址、合约列表等）
- `config/ctp/ctp_md.secret.yaml` - 密码配置（不提交到git）

**文档**:
- [CTP插件配置示例](ctp/config/ctp_md.yaml.example)
- [BUILD_GUIDE.md](../../docs/BUILD_GUIDE.md) - 编译指南
- [USAGE.md](../../docs/USAGE.md) - 使用说明

---

## 开发新插件

### 步骤1：创建插件目录

```bash
cd gateway/plugins
mkdir -p my_broker/{include,src,config}
```

### 步骤2：实现IMDPlugin接口

创建 `my_broker/include/my_broker_md_plugin.h`:

```cpp
#pragma once

#include "plugin/md_plugin_interface.h"
#include "MyBrokerAPI.h"  // 你的SDK头文件

namespace hft {
namespace plugin {
namespace my_broker {

class MyBrokerMDPlugin : public IMDPlugin {
public:
    MyBrokerMDPlugin();
    virtual ~MyBrokerMDPlugin();

    // 实现IMDPlugin接口
    bool Initialize(const std::string& config_file) override;
    bool Start() override;
    void Stop() override;
    bool IsRunning() const override;

    bool Subscribe(const std::vector<std::string>& symbols) override;
    bool Unsubscribe(const std::vector<std::string>& symbols) override;

    bool IsConnected() const override;
    bool IsLoggedIn() const override;
    std::string GetPluginName() const override;
    std::string GetPluginVersion() const override;

    uint64_t GetMessageCount() const override;
    uint64_t GetDroppedCount() const override;

private:
    // 你的私有成员和方法
};

} // namespace my_broker
} // namespace plugin
} // namespace hft
```

### 步骤3：实现插件逻辑

创建 `my_broker/src/my_broker_md_plugin.cpp`:

```cpp
#include "my_broker_md_plugin.h"
#include "shm_queue.h"  // 共享内存队列
#include <iostream>

namespace hft {
namespace plugin {
namespace my_broker {

bool MyBrokerMDPlugin::Initialize(const std::string& config_file) {
    // 1. 加载配置文件
    // 2. 初始化SDK
    // 3. 打开共享内存队列
    return true;
}

bool MyBrokerMDPlugin::Start() {
    // 1. 连接到交易所
    // 2. 登录
    // 3. 订阅合约
    return true;
}

void MyBrokerMDPlugin::Stop() {
    // 1. 断开连接
    // 2. 释放资源
}

// ... 实现其他接口方法

} // namespace my_broker
} // namespace plugin
} // namespace hft
```

### 步骤4：创建主程序

创建 `my_broker/src/main_my_broker_md.cpp`:

```cpp
#include "my_broker_md_plugin.h"
#include <iostream>
#include <signal.h>
#include <memory>
#include <thread>

using namespace hft::plugin::my_broker;

MyBrokerMDPlugin* g_plugin = nullptr;

void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << std::endl;
    if (g_plugin) {
        g_plugin->Stop();
    }
}

int main(int argc, char* argv[]) {
    std::string config_file = "config/my_broker/my_broker_md.yaml";

    // 解析命令行参数...

    try {
        auto plugin = std::make_unique<MyBrokerMDPlugin>();
        g_plugin = plugin.get();

        signal(SIGINT, SignalHandler);
        signal(SIGTERM, SignalHandler);

        if (!plugin->Initialize(config_file)) {
            std::cerr << "[Main] Failed to initialize plugin" << std::endl;
            return 1;
        }

        if (!plugin->Start()) {
            std::cerr << "[Main] Failed to start plugin" << std::endl;
            return 1;
        }

        std::cout << "[Main] Plugin running... (Press Ctrl+C to stop)" << std::endl;
        while (plugin->IsRunning()) {
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }

        return 0;
    } catch (const std::exception& e) {
        std::cerr << "[Main] Fatal error: " << e.what() << std::endl;
        return 1;
    }
}
```

### 步骤5：创建CMakeLists.txt

创建 `my_broker/CMakeLists.txt`:

```cmake
message(STATUS "Configuring MyBroker Plugin...")

# 查找SDK
find_path(MYBROKER_INCLUDE_DIR MyBrokerAPI.h
    PATHS ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/my_broker/include
)

if(NOT MYBROKER_INCLUDE_DIR)
    message(WARNING "MyBroker SDK not found - plugin will not be built")
    return()
endif()

# 可执行文件
add_executable(my_broker_md_plugin
    src/my_broker_md_plugin.cpp
    src/main_my_broker_md.cpp
)

# 包含目录
target_include_directories(my_broker_md_plugin PRIVATE
    ${CMAKE_CURRENT_SOURCE_DIR}/include
    ${CMAKE_CURRENT_SOURCE_DIR}/../../include  # 通用接口
    ${MYBROKER_INCLUDE_DIR}
)

# 链接库
target_link_libraries(my_broker_md_plugin
    ${CMAKE_CURRENT_SOURCE_DIR}/../../third_party/my_broker/lib/libMyBrokerAPI.so
    pthread
)

# 安装
install(TARGETS my_broker_md_plugin DESTINATION bin)

message(STATUS "MyBroker MD Plugin target added: my_broker_md_plugin")
```

### 步骤6：在主CMakeLists.txt中添加选项

编辑 `gateway/CMakeLists.txt`，添加:

```cmake
option(BUILD_MYBROKER_PLUGIN "Build MyBroker plugin" OFF)

if(BUILD_MYBROKER_PLUGIN)
    message(STATUS "Building MyBroker plugin...")
    add_subdirectory(plugins/my_broker)
endif()
```

### 步骤7：编译测试

```bash
cd gateway/build
cmake .. -DBUILD_MYBROKER_PLUGIN=ON
make my_broker_md_plugin -j4

# 运行测试
./plugins/my_broker/my_broker_md_plugin -c config/my_broker/my_broker_md.yaml
```

---

## IMDPlugin接口说明

### 必须实现的方法

#### 生命周期管理

```cpp
// 初始化插件（加载配置、创建SDK实例、打开共享内存）
bool Initialize(const std::string& config_file);

// 启动插件（连接交易所、登录）
bool Start();

// 停止插件（断开连接、释放资源）
void Stop();

// 查询运行状态
bool IsRunning() const;
```

#### 订阅管理

```cpp
// 订阅行情（支持批量）
bool Subscribe(const std::vector<std::string>& symbols);

// 取消订阅
bool Unsubscribe(const std::vector<std::string>& symbols);
```

#### 状态查询

```cpp
// 连接状态
bool IsConnected() const;
bool IsLoggedIn() const;

// 插件信息
std::string GetPluginName() const;      // 例如: "CTP", "XTP", "MyBroker"
std::string GetPluginVersion() const;   // 例如: "1.0.0"
```

#### 统计信息

```cpp
// 已接收的行情消息数量
uint64_t GetMessageCount() const;

// 因队列满而丢弃的消息数量
uint64_t GetDroppedCount() const;
```

### 数据发布

插件接收到行情数据后，需要转换为统一的`MarketDataRaw`格式并推送到共享内存队列：

```cpp
#include "shm_queue.h"

// 打开共享内存队列
hft::shm::ShmManager::Queue* m_queue =
    hft::shm::ShmManager::CreateOrOpen("md_queue");

// 推送数据
hft::shm::MarketDataRaw raw_md = {};
// ... 填充raw_md字段
if (!m_queue->Push(raw_md)) {
    // 队列满，丢弃数据
    m_md_dropped++;
}
```

---

## 配置文件规范

### 主配置文件 (`config/{broker}/{broker}_md.yaml`)

```yaml
{broker}:
  # 连接信息
  front_addr: "tcp://xxx.xxx.xxx.xxx:xxxxx"
  broker_id: "xxxx"

  # 用户信息（从secret文件读取）
  user_id: ""
  password: ""

  # 订阅合约
  instruments:
    - "symbol1"
    - "symbol2"

# 共享内存配置
shm:
  queue_name: "md_queue"
  queue_size: 10000

# 重连配置
reconnect:
  interval_sec: 5
  max_attempts: -1

# 日志配置
log:
  level: "info"
  file: "log/{broker}_md_gateway.log"
  console: true
```

### 密码配置文件 (`config/{broker}/{broker}_md.secret.yaml`)

```yaml
credentials:
  user_id: "YOUR_USER_ID"
  password: "YOUR_PASSWORD"
```

**⚠️ 重要**:
- 密码文件必须添加到`.gitignore`
- 提供`.example`模板供用户参考

---

## 编译选项

### 编译单个插件

```bash
cd gateway/build
cmake .. -DBUILD_CTP_PLUGIN=ON -DBUILD_XTP_PLUGIN=OFF
make ctp_md_plugin
```

### 编译所有插件

```bash
cmake .. -DBUILD_CTP_PLUGIN=ON -DBUILD_XTP_PLUGIN=ON -DBUILD_FEMAS_PLUGIN=ON
make -j4
```

### 查看可用插件

```bash
cmake .. -L | grep BUILD_.*_PLUGIN
```

---

## 调试技巧

### 1. 日志级别

在配置文件中设置`log.level: "debug"`以查看详细日志。

### 2. 共享内存检查

```bash
# 查看共享内存
ipcs -m

# 清理共享内存
ipcs -m | grep user | awk '{print $2}' | xargs ipcrm -m
```

### 3. 使用gdb调试

```bash
gdb ./plugins/ctp/ctp_md_plugin
(gdb) run -c config/ctp/ctp_md.yaml
```

---

## 常见问题

### Q: 如何处理SDK的回调线程？

A: SDK的回调通常在单独的线程中执行，需要确保线程安全：
- 使用`std::atomic`保护状态变量
- 使用`std::mutex`保护订阅列表等共享数据

### Q: 如何处理断线重连？

A: 在`OnFrontDisconnected`回调中：
1. 更新连接状态
2. 等待一段时间（避免频繁重连）
3. 重新创建API实例并连接

### Q: 如何优化延迟？

A:
- 使用共享内存队列（已实现）
- 避免在回调中进行耗时操作
- 使用原子操作而不是锁

### Q: 如何测试插件？

A:
1. 使用仿真环境（如CTP SimNow）
2. 编写单元测试
3. 使用md_simulator模拟行情数据

---

## 参考资料

- [CTP插件实现](ctp/) - 完整参考实现
- [插件化架构设计](../../docs/gateway/gateway_插件化架构设计_2026-01-27-12_00.md) - 详细设计文档
- [BUILD_GUIDE.md](../../docs/BUILD_GUIDE.md) - 构建指南
- [USAGE.md](../../docs/USAGE.md) - 使用说明

---

**最后更新**: 2026-01-27
**维护者**: 开发团队
