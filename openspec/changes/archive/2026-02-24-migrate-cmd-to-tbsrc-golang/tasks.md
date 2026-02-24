# Tasks: 迁移 webserver 到 tbsrc-golang

> backtest / backtest_optimize 依赖 protobuf 类型体系，需要重写，本次不迁移。

## 迁移文件

- [x] 复制 `golang/cmd/webserver/main.go` 到 `tbsrc-golang/cmd/webserver/main.go`

## 验证

- [x] `cd tbsrc-golang && go build ./cmd/webserver/...`
- [x] 运行 `build_deploy_new.sh` 完整编译通过（backtest 可选跳过即可）
