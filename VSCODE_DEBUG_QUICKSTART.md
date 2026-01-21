# VSCode 调试快速开始

**5分钟学会在VSCode中调试 MD Client**

---

## ✅ 第一步: 安装必要的扩展

1. 打开VSCode
2. 按 `Cmd+Shift+X` 打开扩展面板
3. 搜索并安装: **Go** (by Go Team at Google)
4. 重启VSCode

---

## ✅ 第二步: 安装调试工具

在终端运行:

```bash
# 安装Delve调试器
go install github.com/go-delve/delve/cmd/dlv@latest

# 验证安装
dlv version
```

应该看到类似输出: `Delve Debugger Version: 1.x.x`

---

## ✅ 第三步: 打开项目

```bash
cd /Users/user/PWorks/RD/hft-poc
code .
```

VSCode会自动识别 `.vscode/` 目录中的配置。

---

## ✅ 第四步: 启动调试 MD Client

### 🎯 最简单的方法

1. **打开文件**: `golang/cmd/md_client/main.go`

2. **设置断点**:
   - 点击第78行行号左侧（行号和代码之间的空白区域）
   - 会出现一个红点 🔴

3. **启动调试**:
   - 按 `F5` 键
   - 或点击顶部菜单: Run → Start Debugging

4. **选择配置**:
   - 如果弹出选择框，选择: `Debug MD Client (gRPC)`

5. **观察结果**:
   - 程序会在断点处暂停
   - 左侧面板显示变量、调用栈等信息

---

## 🎮 调试控制

程序暂停后，使用以下按键：

| 快捷键 | 功能 | 说明 |
|--------|------|------|
| `F5` | 继续 | 运行到下一个断点 |
| `F10` | 单步跳过 | 执行当前行，不进入函数内部 |
| `F11` | 单步进入 | 进入函数内部 |
| `Shift+F11` | 单步跳出 | 跳出当前函数 |
| `Shift+F5` | 停止 | 停止调试 |

---

## 📊 查看变量

调试时，左侧会自动显示：

- **VARIABLES** (变量面板)
  - `Local` - 当前函数的局部变量
  - `Global` - 全局变量

- **WATCH** (监视面板)
  - 点击 `+` 添加要监视的表达式
  - 示例: `md.Symbol`, `latency.Microseconds()`

- **CALL STACK** (调用栈)
  - 查看函数调用链
  - 点击可跳转到不同的栈帧

---

## ⚠️ 注意事项

### MD Client 需要 MD Gateway 运行

**如果Gateway未启动**，调试时会报错: `connection refused`

**解决方法1**: 启动Gateway (终端运行)

```bash
# 终端1: 启动模拟器
cd gateway/build
./md_simulator 1000 queue

# 终端2: 启动Gateway
cd gateway/build
./md_gateway queue
```

**解决方法2**: 调试不依赖Gateway的程序

在调试面板选择: `Debug Strategy Demo` 或 `Debug All Strategies Demo`

---

## 🎯 推荐的调试流程

### 1. 先调试不依赖Gateway的程序

```
步骤:
1. 按 Cmd+Shift+D 打开调试面板
2. 选择: Debug Strategy Demo
3. 按 F5 启动
4. 熟悉调试操作
```

### 2. 再调试需要Gateway的程序

```
步骤:
1. 启动Gateway (参见上文)
2. 选择: Debug MD Client (gRPC)
3. 设置断点在 main.go:78 (接收行情)
4. 按 F5 启动
5. 观察行情数据
```

---

## 🔍 常用断点位置

### MD Client (main.go)

```go
// Line 52: 进入gRPC客户端
func runGRPCClient(ctx context.Context, cancel context.CancelFunc) {
    // 在这里设置断点 ◄─── 推荐

// Line 78: 接收行情
for {
    md, err := stream.Recv()  // 在这里设置断点 ◄─── 推荐

// Line 95: 延迟计算
latency := time.Since(...)  // 在这里设置断点 ◄─── 查看延迟
```

### Strategy Demo (main.go)

```go
// Line 102: 模拟行情生成
func simulateMarketData(s strategy.Strategy) {
    // 在这里设置断点 ◄─── 推荐

// Line 134: 策略收到行情
s.OnMarketData(md)  // 在这里设置断点 ◄─── 推荐
```

---

## 🎨 进阶技巧

### 条件断点

只在特定条件下中断:

1. 右键断点
2. 选择 "Edit Breakpoint..."
3. 选择 "Expression"
4. 输入条件，例如: `md.Symbol == "ag2412"`

### 日志断点

不停止执行，只输出日志:

1. 右键断点
2. 选择 "Edit Breakpoint..."
3. 选择 "Log Message"
4. 输入消息，例如: `Received {md.Symbol} at {md.BidPrice[0]}`

### 使用调试控制台

在 Debug Console 中执行表达式:

```
> len(md.BidPrice)
5

> md.Symbol
"ag2412"

> md.BidPrice[0]
7950.5
```

---

## 🚨 常见问题

### Q: 按F5没反应？

**A**:
1. 确保打开的是 `.go` 文件
2. 或先打开调试面板 (Cmd+Shift+D)，选择配置后点击播放按钮

### Q: 提示找不到dlv？

**A**:
```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

### Q: 断点是灰色的？

**A**:
1. 代码可能未保存，先保存文件
2. 断点设置在无效位置（空行、注释），换一行试试

### Q: 变量显示 "optimized out"？

**A**: 这是正常的，调试模式下某些变量会被优化。继续执行到该变量被使用的地方。

---

## 📚 完整文档

更详细的说明请查看: [.vscode/README.md](.vscode/README.md)

---

## 🎉 总结

**最简单的调试步骤**:

```
1. 打开 main.go
2. 点击行号左侧设置断点 🔴
3. 按 F5
4. 程序停在断点处
5. 按 F10 单步执行
6. 观察左侧变量面板
7. 按 F5 继续运行
```

**就这么简单！** 🚀

---

**提示**: 第一次调试可能需要几秒钟编译，之后会很快。
