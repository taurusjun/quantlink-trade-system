# QuantlinkTrader Web 控制台使用说明

## 页面说明

| 文件 | 用途 | 说明 |
|------|------|------|
| `control.html` | 单策略控制台 | 适用于单策略模式，简洁直观 |
| `dashboard.html` | **多策略仪表板** | 适用于多策略模式，支持实时指标、条件高亮 |

---

## Dashboard (多策略仪表板) - 新增

### 功能特性

- **多策略总览** - 显示所有策略状态、PnL 汇总
- **实时指标** - Z-Score、相关性等指标实时展示
- **条件高亮** - 策略满足交易条件时闪烁提示
- **持仓展示** - 实时持仓列表及盈亏
- **一键激活/停止** - 每个策略独立控制
- **自动刷新** - 5秒自动刷新数据

### 打开方式

```bash
# macOS
open web/dashboard.html

# Linux
xdg-open web/dashboard.html

# 或直接双击文件
```

### API 端点依赖

Dashboard 使用以下 API：
- `GET /api/v1/dashboard/overview` - 总览数据
- `GET /api/v1/strategies` - 策略列表
- `GET /api/v1/indicators/realtime` - 实时指标
- `GET /api/v1/positions` - 持仓数据
- `POST /api/v1/strategies/{id}/activate` - 激活策略
- `POST /api/v1/strategies/{id}/deactivate` - 停止策略

### 技术栈

- Vue.js 3 (CDN)
- 纯前端，无需构建
- 响应式布局

---

## Control (单策略控制台) - 原有

### 包含内容

1. **API 并发保护** - 在 `pkg/trader/api.go` 中添加 `commandMu` 互斥锁
2. **Web UI** - 单个 HTML 文件，防抖 + Loading 状态
3. **并发安全** - 防止多人/多次重复操作

---

## 使用方法

### 1. 启动 Trader（确保 API 已启用）

```bash
# 方法 A: 使用脚本启动
./runTrade.sh 92201

# 方法 B: 直接启动
./bin/QuantlinkTrader --config config/trader.ag2502.ag2504.yaml
```

**确认 API 已启用**（配置文件中）：
```yaml
api:
  enabled: true
  port: 9201
  host: "localhost"
```

### 2. 打开 Web 控制台

**选项 A - 直接在浏览器打开**（推荐，最简单）：
```bash
# macOS
open web/control.html

# Linux
xdg-open web/control.html

# 或者直接双击 web/control.html 文件
```

**选项 B - 通过 HTTP 服务器**（如果需要远程访问）：
```bash
# 使用 Python 3
cd web
python3 -m http.server 8000

# 然后访问: http://localhost:8000/control.html
```

### 3. 配置 API 地址

在页面顶部的"API 配置"区域：
- **API 地址**: `localhost`（或服务器 IP）
- **端口**: `9201`（对应策略的 API 端口）

点击 **"🔄 连接并刷新状态"** 按钮。

### 4. 控制策略

- **🚀 激活策略** - 启动交易
- **🛑 停止策略** - 停止并平仓
- 状态会自动每 10 秒刷新一次

---

## 并发保护机制

### ✅ API 层保护

```go
// pkg/trader/api.go
type APIServer struct {
    commandMu sync.Mutex  // 命令互斥锁
}

func (a *APIServer) handleActivate() {
    a.commandMu.Lock()         // 🔒 加锁
    defer a.commandMu.Unlock() // 🔓 释放

    // 只有一个请求能执行
    a.trader.Strategy.Start()
}
```

**保护效果**：
- ✅ 多人同时点击"激活" → 只有第一个生效
- ✅ 快速重复点击 → 串行执行，不会重复
- ✅ 多个浏览器标签 → 互斥保护

### ✅ 前端防抖

```javascript
let isProcessing = false;

async function activateStrategy() {
    if (isProcessing) {
        alert('操作进行中，请稍候...');
        return;  // 🛑 阻止重复调用
    }

    isProcessing = true;
    // 按钮变为 loading 状态
    // 执行 API 调用
    isProcessing = false;
}
```

**防抖效果**：
- ✅ 按钮显示 Loading 动画
- ✅ 禁用所有按钮防止误操作
- ✅ 操作完成前无法再次点击

---

## 功能特性

### 1. 实时状态显示

- **策略 ID** - 当前策略编号
- **运行状态** - 运行中/已停止
- **激活状态** - 已激活/未激活
- **模式** - live/simulation/backtest
- **品种** - 交易品种列表
- **持仓** - 当前净持仓
- **盈亏** - 已实现和未实现盈亏

### 2. 自动刷新

- 页面加载时自动获取状态
- 每 10 秒自动刷新一次
- 手动点击"连接并刷新状态"按钮

### 3. 操作反馈

- ✅ 成功消息：绿色提示框
- ❌ 错误消息：红色提示框
- 🔄 Loading 动画：操作进行中
- 自动消失：5 秒后自动隐藏

### 4. 二次确认

停止策略时会弹出确认对话框：
```
确认停止策略并平仓？
[确定] [取消]
```

### 5. 键盘快捷键

- `Ctrl + A` - 激活策略
- `Ctrl + D` - 停止策略
- `Ctrl + R` - 刷新状态

---

## 多策略管理

如果有多个策略同时运行，可以打开多个浏览器标签页：

```
标签页 1: localhost:9201  →  策略 92201 (ag2502-ag2504)
标签页 2: localhost:9301  →  策略 93201 (al2502-al2503)
标签页 3: localhost:4101  →  策略 41231 (rb2505-rb2510)
```

每个标签页独立控制一个策略。

---

## 安全建议

### 开发/本地环境

✅ 当前配置已足够：
- API 绑定到 `localhost`
- 无需认证
- 单机使用

### 生产环境

如果需要远程访问，建议：

1. **使用 VPN**
   - 通过 VPN 连接到服务器
   - 继续使用 localhost

2. **SSH 隧道**
   ```bash
   ssh -L 9201:localhost:9201 user@server
   # 本地访问 localhost:9201
   ```

3. **添加认证**（未来增强）
   - API Key 或 JWT Token
   - IP 白名单
   - HTTPS (nginx 反向代理)

---

## 测试并发保护

### 测试 1: 多次快速点击

1. 打开控制台
2. 疯狂点击"激活策略"按钮 10 次
3. ✅ 预期：只执行一次，其他请求被阻止

### 测试 2: 多人同时操作

1. 打开两个浏览器标签页（或不同浏览器）
2. 同时点击"激活策略"
3. ✅ 预期：只有一个成功，另一个等待

### 测试 3: 查看日志

```bash
tail -f log/trader.*.92201.log
```

观察日志中只有一次 "Activating strategy" 记录。

---

## 故障排查

### 1. 无法连接到 API

**错误**: `连接失败: Failed to fetch`

**检查**:
```bash
# 1. 确认 Trader 正在运行
ps aux | grep QuantlinkTrader

# 2. 确认 API 端口
netstat -an | grep 9201

# 3. 检查配置文件
cat config/trader.ag2502.ag2504.yaml | grep -A 3 "api:"
```

### 2. API 地址/端口错误

**错误**: `❌ 获取状态失败`

**解决**: 检查页面顶部的 API 配置是否正确。

### 3. 策略不响应

**错误**: 点击按钮后无反应

**检查**:
```bash
# 查看 API 日志
tail -f log/trader.*.92201.log | grep API

# 手动测试 API
curl -X POST http://localhost:9201/api/v1/strategy/activate
```

---

## 与其他控制方式对比

| 控制方式 | 优点 | 缺点 | 适用场景 |
|---------|------|------|---------|
| **Web UI** | 直观、易用、实时状态 | 需要浏览器 | 日常操作 |
| **Unix 信号** | 快速、脚本化 | 需要 SSH | 自动化脚本 |
| **API 脚本** | 灵活、可编程 | 需要命令行 | 批量操作 |

---

## 下一步增强（可选）

如果需要更多功能，可以考虑：

1. **多策略一览** - 在一个页面管理所有策略
2. **历史记录** - 显示操作历史和日志
3. **图表展示** - PNL 曲线图
4. **告警通知** - 浏览器推送通知
5. **认证登录** - 用户名密码
6. **WebSocket** - 实时推送状态更新

---

## 总结

✅ **并发安全** - API 加锁 + 前端防抖
✅ **简单易用** - 单文件 HTML，开箱即用
✅ **实时监控** - 自动刷新状态
✅ **操作保护** - 二次确认 + 错误提示

现在可以安全地让多人使用 Web UI 控制策略，不用担心重复操作！
