## Context

当前 data_new 使用 common/controls 存放所有环境的 control 文件，control 内 model 路径统一为 `./models/model.*`。部署时 build_deploy_java.sh 将 controls 和 models 扁平复制到 deploy_java/，start_strategy.sh 在运行时根据 gateway_mode 把环境子目录的模型覆盖到 models/ 根目录。这种间接方式容易出错（如忘记覆盖、路径混乱）。

## Goals / Non-Goals

**Goals**:
- data_new 中 controls 按环境隔离，与 models/data 同层
- deploy_java 直接反映环境隔离结构（live/sim 各自独立）
- control 文件直接引用正确的 model 路径，无需运行时覆盖
- CTP 实盘风控参数匹配实际持仓规模

**Non-Goals**:
- 不修改 Java 代码（仅配置和脚本）
- 不改变 common/config 的共享机制

## Decisions

1. **目录结构**: `data_new/{live,sim}/` 各自包含 `controls/{day,night}/`、`models/`、`data/`。deploy_java 同构。
   - 替代方案：controls 保留在 common 但运行时按模式选择 → 仍需运行时逻辑，不如直接分离
2. **model 路径**: control 文件中使用 `./live/models/model.*` 或 `./sim/models/model.*`（相对于 deploy_java 工作目录）。
   - 替代方案：相对于 control 文件的 `../models/model.*` → Java 解析 modelFile 时基于工作目录，不适用
3. **common/config 保留**: config 仍从 common 复制，环境专属 config 叠加覆盖。

## Risks / Trade-offs

- [Risk] 现有 deploy_java 目录需要重建 → 已实施，验证通过
- [Risk] 脚本中硬编码路径需全部更新 → build_deploy_java.sh 内嵌脚本已统一修改

## Migration Plan

1. 创建 data_new/{live,sim}/controls/{day,night}/ 目录
2. 从 common/controls 复制并修改 model 路径
3. 删除 common/controls
4. 更新 build_deploy_java.sh 合并逻辑和所有内嵌脚本
5. 重建 deploy_java 目录结构
6. 验证策略启动正常

## Open Questions

无
