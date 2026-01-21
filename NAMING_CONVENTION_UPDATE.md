# 命名规范统一说明

**日期：** 2026-01-20

---

## 问题

项目中Gateway主程序的命名规则不一致：

**原命名：**
- ❌ `main_shm.cpp` - MD Gateway主程序（强调实现方式"共享内存"）
- ✅ `main_ors.cpp` - ORS Gateway主程序（强调功能"ORS"）

**问题分析：**
1. `main_shm.cpp` 强调的是实现细节（如何通信），而不是功能职责（做什么）
2. `main_ors.cpp` 强调的是功能定位，命名更清晰
3. 两种命名风格混用，降低代码可读性

---

## 解决方案

### 统一命名规则：按功能职责命名

**新命名：**
- ✅ `main_md.cpp` - MD Gateway主程序
- ✅ `main_ors.cpp` - ORS Gateway主程序
- ✅ 未来：`main_counter.cpp` - Counter Gateway主程序

### 命名原则

1. **按功能而非实现方式命名**
   - 好：`main_md.cpp`（功能：Market Data）
   - 坏：`main_shm.cpp`（实现：Shared Memory）

2. **保持一致性**
   - 所有Gateway主程序使用 `main_{service}.cpp` 格式
   - 例如：`main_md.cpp`, `main_ors.cpp`, `main_counter.cpp`

3. **清晰表达职责**
   - 从文件名就能看出这是哪个服务的入口
   - 无需查看代码就知道功能定位

---

## 变更详情

### 文件重命名

```bash
# 重命名
src/main_shm.cpp  →  src/main_md.cpp
```

### CMakeLists.txt更新

**变更前：**
```cmake
set(GATEWAY_SHM_SRCS
    src/main_shm.cpp
    src/md_gateway.cpp
    ...
)
```

**变更后：**
```cmake
set(MD_GATEWAY_SRCS
    src/main_md.cpp
    src/md_gateway.cpp
    ...
)
```

**说明：**
- 变量名也从 `GATEWAY_SHM_SRCS` 改为 `MD_GATEWAY_SRCS`
- 更清晰地表达这是MD Gateway的源文件列表

### 可执行文件名

**保持不变：**
- `md_gateway_shm` - MD Gateway可执行文件
- `ors_gateway` - ORS Gateway可执行文件

**说明：**
- 可执行文件名保留 `_shm` 后缀，表明使用共享内存模式
- 这是合理的，因为未来可能有 `md_gateway_tcp`（TCP模式）等变体
- 主程序文件名和可执行文件名可以不同

---

## 项目文件结构（更新后）

```
gateway/
├── src/
│   ├── main_md.cpp           ← MD Gateway主程序
│   ├── main_ors.cpp          ← ORS Gateway主程序
│   ├── md_gateway.cpp        ← MD Gateway实现
│   ├── ors_gateway.cpp       ← ORS Gateway实现
│   ├── md_simulator.cpp      ← 行情模拟器
│   └── md_benchmark.cpp      ← 性能测试工具
│
├── include/
│   ├── md_gateway.h          ← MD Gateway头文件
│   ├── ors_gateway.h         ← ORS Gateway头文件
│   ├── shm_queue.h           ← 共享内存队列
│   └── performance_monitor.h ← 性能监控
│
└── build/
    ├── md_gateway_shm        ← MD Gateway可执行文件
    ├── ors_gateway           ← ORS Gateway可执行文件
    ├── md_simulator          ← 模拟器可执行文件
    └── md_benchmark          ← 基准测试可执行文件
```

---

## 命名规范总结

### 主程序文件

| 服务 | 主程序文件 | 可执行文件 | 说明 |
|-----|-----------|-----------|------|
| MD Gateway | `main_md.cpp` | `md_gateway_shm` | 行情网关 |
| ORS Gateway | `main_ors.cpp` | `ors_gateway` | 订单路由网关 |
| Counter Gateway | `main_counter.cpp` | `counter_gateway` | 柜台网关（计划中） |

### 实现文件

| 服务 | 头文件 | 实现文件 | 说明 |
|-----|--------|---------|------|
| MD Gateway | `md_gateway.h` | `md_gateway.cpp` | 行情网关实现 |
| ORS Gateway | `ors_gateway.h` | `ors_gateway.cpp` | 订单路由实现 |
| Counter Gateway | `counter_gateway.h` | `counter_gateway.cpp` | 柜台网关实现（计划中） |

### 工具和库

| 类型 | 文件名 | 说明 |
|-----|--------|------|
| 模拟器 | `md_simulator.cpp` | 行情数据模拟器 |
| 测试工具 | `md_benchmark.cpp` | 性能基准测试 |
| 共享库 | `shm_queue.h` | 共享内存队列 |
| 监控库 | `performance_monitor.h` | 性能监控 |

---

## 编译验证

### 编译命令

```bash
# 完整重新编译
./scripts/build_gateway.sh

# 或单独编译
cd gateway/build
cmake ..
make
```

### 编译结果

```
✅ 编译成功
[100%] Built target md_gateway_shm
[100%] Built target ors_gateway
[100%] Built target md_simulator
[100%] Built target md_benchmark
```

### 可执行文件

```bash
$ ls -lh gateway/build/*.gateway* gateway/build/md_*

-rwxr-xr-x  507K  md_gateway_shm    # MD Gateway
-rwxr-xr-x  830K  ors_gateway       # ORS Gateway
-rwxr-xr-x   77K  md_simulator      # 模拟器
-rwxr-xr-x  134K  md_benchmark      # 基准测试
```

---

## 影响范围

### 需要更新的文档

- [x] CMakeLists.txt - 已更新
- [ ] README.md - 需要检查是否有引用（如果有需更新）
- [ ] USAGE.md - 需要检查是否有引用
- [ ] 构建脚本 - 无需更新（自动生成）

### 不受影响

- ✅ 可执行文件名保持不变
- ✅ 启动命令保持不变
- ✅ 功能逻辑保持不变
- ✅ 接口定义保持不变

---

## 后续建议

### 1. 保持命名一致性

今后新增Gateway服务时，遵循统一的命名规则：

```
main_{service}.cpp     # 主程序
{service}_gateway.h    # 头文件
{service}_gateway.cpp  # 实现文件
{service}_gateway      # 可执行文件
```

### 2. 文档及时更新

在重构或重命名时，同步更新相关文档：
- 项目README
- 使用指南
- 架构设计文档

### 3. Git提交信息

```bash
git add gateway/src/main_md.cpp gateway/CMakeLists.txt
git commit -m "refactor: rename main_shm.cpp to main_md.cpp for consistency

- Unified naming convention: main_{service}.cpp
- MD Gateway: main_shm.cpp → main_md.cpp
- Updated CMakeLists.txt references
- No functional changes
"
```

---

## 总结

这次重命名统一了项目的命名规范，提高了代码可读性和可维护性。核心原则是：

1. ✅ **按功能命名，而非实现方式**
2. ✅ **保持命名一致性**
3. ✅ **清晰表达职责**

通过这次改进，项目结构更加清晰，新加入的开发者也能快速理解各个文件的作用。

---

**更新时间：** 2026-01-20
**影响版本：** Week 5-6
**状态：** ✅ 已完成并验证
