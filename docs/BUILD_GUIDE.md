# QuantLink Trade System - 构建指南

## 📋 前置要求

### 必需工具

| 工具 | 最低版本 | 安装命令 |
|------|---------|---------|
| Go | 1.19+ | `brew install go` |
| CMake | 3.15+ | `brew install cmake` |
| protoc | 3.21+ | `brew install protobuf` |
| protoc-gen-go | latest | `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` |
| protoc-gen-go-grpc | latest | `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest` |

### 可选工具

| 工具 | 用途 | 安装命令 |
|------|------|---------|
| NATS Server | 消息总线 | `brew install nats-server` |
| delve | Go 调试器 | `go install github.com/go-delve/delve/cmd/dlv@latest` |

---

## 🚀 快速构建

### 一键构建所有组件

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system

# 1. 生成 Protobuf 代码
./scripts/generate_proto.sh

# 2. 构建 C++ Gateway
./scripts/build_gateway.sh

# 3. 构建 Golang 应用
./scripts/build_golang.sh
```

---

## 📦 分步构建

### 步骤 1: 生成 Protobuf 代码

**重要**: 必须首先生成 protobuf 代码，否则 Go 程序无法编译。

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system
./scripts/generate_proto.sh
```

**输出**:
```
✅ Generated 6 protobuf files
  - pkg/proto/ors/order.pb.go
  - pkg/proto/ors/order_grpc.pb.go
  - pkg/proto/common/common.pb.go
  - pkg/proto/common/common_grpc.pb.go
  - pkg/proto/md/market_data.pb.go
  - pkg/proto/md/market_data_grpc.pb.go
```

---

### 步骤 2: 构建 C++ Gateway 层

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system
./scripts/build_gateway.sh
```

**生成的可执行文件**:
```
gateway/build/
├── md_gateway        # 行情网关
├── md_simulator      # 行情模拟器
├── md_benchmark      # 性能测试工具
├── ors_gateway       # 订单网关
└── counter_gateway   # 交易所网关
```

**验证构建**:
```bash
ls -lh gateway/build/
# 应该看到 5 个可执行文件
```

---

### 步骤 3: 构建 Golang 应用层

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system/golang

# 构建所有 Demo 程序
go build -o bin/indicator_demo ./cmd/indicator_demo
go build -o bin/strategy_demo ./cmd/strategy_demo
go build -o bin/all_strategies_demo ./cmd/all_strategies_demo
go build -o bin/integrated_demo ./cmd/integrated_demo
go build -o bin/md_client ./cmd/md_client
go build -o bin/ors_client ./cmd/ors_client
```

**或使用脚本**:
```bash
./scripts/build_golang.sh
```

**生成的可执行文件**:
```
golang/bin/
├── indicator_demo        # 指标演示
├── strategy_demo         # 策略演示
├── all_strategies_demo   # 所有策略演示
├── integrated_demo       # 集成系统演示
├── md_client            # 行情客户端
└── ors_client           # 订单客户端
```

---

## ✅ 验证构建

### 1. 验证 Protobuf 生成

```bash
# 检查生成的文件
find golang/pkg/proto -name "*.pb.go"

# 应该输出 6 个文件
```

### 2. 验证 Gateway 构建

```bash
# 测试 MD Gateway
./gateway/build/md_gateway --help

# 测试 ORS Gateway
./gateway/build/ors_gateway --help
```

### 3. 验证 Golang 程序

```bash
# 运行指标演示
./golang/bin/indicator_demo

# 应该看到指标计算输出
```

---

## 🔧 常见构建问题

### 问题 1: protoc 命令未找到

**错误信息**:
```
protoc: command not found
```

**解决方法**:
```bash
brew install protobuf
protoc --version  # 验证安装
```

---

### 问题 2: protoc-gen-go 未找到

**错误信息**:
```
protoc-gen-go: program not found or is not executable
```

**解决方法**:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 确保 $GOPATH/bin 在 PATH 中
export PATH=$PATH:$(go env GOPATH)/bin
```

---

### 问题 3: Go 编译错误 "cannot find package"

**错误信息**:
```
package github.com/yourusername/quantlink-trade-system/pkg/proto/ors: cannot find package
```

**原因**: 未生成 protobuf 代码

**解决方法**:
```bash
./scripts/generate_proto.sh
go mod tidy
```

---

### 问题 4: C++ 编译错误

**错误信息**:
```
fatal error: grpc++/grpc++.h: No such file or directory
```

**解决方法**:
```bash
# 安装 gRPC 和依赖
brew install grpc
brew install protobuf

# 清理并重新构建
rm -rf gateway/build
./scripts/build_gateway.sh
```

---

## 📁 构建输出目录结构

```
quantlink-trade-system/
├── gateway/
│   └── build/               ← C++ 可执行文件
│       ├── md_gateway
│       ├── ors_gateway
│       ├── counter_gateway
│       ├── md_simulator
│       └── md_benchmark
├── golang/
│   ├── bin/                 ← Go 可执行文件
│   │   ├── indicator_demo
│   │   ├── strategy_demo
│   │   ├── all_strategies_demo
│   │   ├── integrated_demo
│   │   ├── md_client
│   │   └── ors_client
│   └── pkg/proto/           ← 生成的 protobuf 代码
│       ├── ors/
│       │   ├── order.pb.go
│       │   └── order_grpc.pb.go
│       ├── common/
│       │   ├── common.pb.go
│       │   └── common_grpc.pb.go
│       └── md/
│           ├── market_data.pb.go
│           └── market_data_grpc.pb.go
└── scripts/
    ├── generate_proto.sh    ← Protobuf 生成脚本
    ├── build_gateway.sh     ← Gateway 构建脚本
    └── build_golang.sh      ← Golang 构建脚本
```

---

## 🔄 重新构建

### 完全清理

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system

# 清理 C++ 构建
rm -rf gateway/build

# 清理 Go 构建
rm -rf golang/bin/*

# 清理生成的 protobuf 文件
rm -rf golang/pkg/proto/*/
```

### 完全重新构建

```bash
# 1. 清理
rm -rf gateway/build golang/bin/* golang/pkg/proto/*/

# 2. 重新生成和构建
./scripts/generate_proto.sh
./scripts/build_gateway.sh
./scripts/build_golang.sh
```

---

## 📊 构建时间参考

| 步骤 | 预计时间 | 备注 |
|------|---------|------|
| Protobuf 生成 | ~5秒 | 首次运行 |
| C++ Gateway 构建 | ~30-60秒 | 取决于 CPU |
| Go 程序构建 | ~10-20秒 | 每个程序 |
| **总计** | **~2-3分钟** | 完全构建 |

---

## 💡 开发建议

### 1. 增量构建

只重新构建修改的组件：

```bash
# 只重新构建某个 Go 程序
cd golang
go build -o bin/ors_client ./cmd/ors_client

# 只重新构建某个 C++ 程序
cd gateway/build
make ors_gateway
```

### 2. 开发模式

使用 `go run` 避免每次都构建：

```bash
# 直接运行，无需构建
go run ./cmd/ors_client -gateway localhost:50052
```

### 3. 调试构建

```bash
# C++ Debug 模式
cd gateway/build
cmake -DCMAKE_BUILD_TYPE=Debug ..
make

# Go 调试模式（保留符号）
go build -gcflags="all=-N -l" -o bin/ors_client ./cmd/ors_client
```

---

## 📞 获取帮助

如果构建遇到问题：

1. **查看详细错误信息**
2. **检查前置工具是否正确安装**
3. **查看本文档的常见问题部分**
4. **参考项目文档**: `docs/系统启动_20260120.md`

---

## ✅ 快速检查清单

构建前检查：

- [ ] Go 版本 >= 1.19
- [ ] CMake 版本 >= 3.15
- [ ] protoc 已安装
- [ ] protoc-gen-go 已安装
- [ ] protoc-gen-go-grpc 已安装
- [ ] $GOPATH/bin 在 PATH 中

构建后检查：

- [ ] 6 个 protobuf 文件已生成
- [ ] 5 个 C++ 可执行文件已生成
- [ ] 6 个 Go 可执行文件已生成
- [ ] 所有程序可以正常运行
