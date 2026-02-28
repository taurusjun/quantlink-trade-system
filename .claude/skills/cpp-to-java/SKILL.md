---
name: cpp-to-java
description: 将 tbsrc C++ 原代码翻译为 Java。当用户提到翻译、translate、迁移到Java、转Java时使用。
---

# C++ to Java 翻译

## 源与目标

| | 路径 |
|---|------|
| C++ 原代码 | `/Users/user/PWorks/RD/tbsrc/` |
| C++ 头文件 | `/Users/user/PWorks/RD/tbsrc/Strategies/include/` |
| HFT 基础库 | `/Users/user/PWorks/RD/hftbase/` |
| Java 目标 | `tbsrc-java/src/main/java/com/quantlink/trader/` |

## 包映射

| C++ 目录 | Java 包 |
|---------|---------|
| `tbsrc/Strategies/` | `com.quantlink.trader.strategy` |
| `tbsrc/Strategies/include/` | `com.quantlink.trader.strategy`（类型/接口） |
| `tbsrc/common/` | `com.quantlink.trader.common` |
| `tbsrc/main/` | `com.quantlink.trader` |
| `hftbase/CommonUtils/` | `com.quantlink.trader.core` |
| `hftbase/` (SHM/IPC) | `com.quantlink.trader.shm` |

## 核心原则

- **完整翻译** — 禁止省略任何 C++ 逻辑，无法翻译时停下来问用户
- **先读后写** — 翻译前必须读取 C++ 原文件 + 相关头文件
- **增量模式** — 目标 Java 文件已存在时，只翻译缺失的方法

## 流程

### 1. 意图路由

- 含文件名/类名 → 进入步骤 2 翻译该文件
- 含 `status` / `进度` → 扫描已翻译文件，输出覆盖率表格后结束
- 含 `diff` / `对比` → 对比 C++ 方法列表与 Java 已有方法，输出差异后结束
- 无参数 → 列出 `tbsrc/Strategies/` 下所有 .cpp 文件及翻译状态，让用户选择

### 2. 定位源文件

用户输入可以是：文件名（`PairwiseArbStrategy.cpp`）、类名（`PairwiseArbStrategy`）、方法名（`SendOrder`）。

搜索顺序：
1. `tbsrc/Strategies/<name>.cpp`
2. `tbsrc/Strategies/include/<name>.h`
3. `tbsrc/common/` 递归搜索
4. `hftbase/` 递归搜索

找不到则报错，不猜测。

### 3. 读取依赖链

```
读取 .cpp → 提取 #include → 读取对应 .h
                           → 识别基类 → 读取基类 .cpp + .h
                           → 识别引用类型 → 读取相关头文件
```

**必须读完依赖链再开始翻译。** 特别是 `ExecutionStrategy.h` 中的成员变量定义。

### 4. 检查目标文件

读取目标 Java 文件（如果存在）：
- **已存在** → 提取已翻译的方法列表，进入增量模式（只翻译缺失方法）
- **不存在** → 完整翻译模式

### 5. 翻译

#### 命名转换（机械规则，不可更改）

| C++ | Java | 示例 |
|-----|------|------|
| 类名 | 保持不变 | `PairwiseArbStrategy` → `PairwiseArbStrategy` |
| 方法名 | 首字母小写 | `SendOrder()` → `sendOrder()` |
| 成员变量 | 去 `m_` + 驼峰 | `m_netpos_pass` → `netposPass` |
| 常量/宏 | `static final` | `MAX_SIZE` → `static final int MAX_SIZE` |
| `std::vector<T>` | `List<T>` | |
| `std::map<K,V>` | `Map<K,V>` | |
| `std::string` | `String` | |
| 指针 `T*` | Java 引用 `T` | |
| function pointer | 接口 / lambda | |

#### 必须生成的注释

每个方法上方：
```java
// 迁移自: tbsrc/Strategies/文件.cpp:方法名() (L起始-L结束)
```

关键逻辑行：
```java
// C++: 原始C++代码
double longPlaceDiff = firstThold.longPlace - firstThold.beginPlace;
```

结构差异处：
```java
// [C++差异] C++ 使用 function pointer callback，Java 使用接口回调。
// 参考: tbsrc/Strategies/include/ExecutionStrategy.h:85
```

#### 禁止事项

- 省略任何 C++ 逻辑（无论是否"当前场景不需要"）
- 自行重命名方法或变量（必须按上述机械规则）
- 跳过调用链路（C++ 的 A→B→C 不可在 Java 中把 C 塞进 A）
- 标注 "TODO"/"待补齐" 来掩盖省略
- 自设默认值（所有参数必须来自配置）

遇到无法直接翻译的逻辑 → **停下来问用户**，说明原因和可选方案。

### 6. 输出

翻译完成后输出摘要：

| 项目 | 内容 |
|------|------|
| 源文件 | `tbsrc/Strategies/xxx.cpp` (N 行) |
| 目标文件 | `tbsrc-java/.../Xxx.java` |
| 方法数 | 新增 M 个 / 已有 K 个 |
| C++差异 | 逐条列出 |
| 需确认 | 无法自动翻译的部分（如有） |
