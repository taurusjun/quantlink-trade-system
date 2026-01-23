#!/bin/bash
# QuantlinkTrader 快捷启动脚本
# Usage: ./start_trader.sh [strategy_name]

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

# ═══════════════════════════════════════════════════════════
# 函数: 获取策略配置文件
# ═══════════════════════════════════════════════════════════
get_config_file() {
    local strategy=$1
    case "$strategy" in
        "prod")         echo "config/trader.yaml" ;;
        "test")         echo "config/trader.test.yaml" ;;
        "passive")      echo "config/trader.yaml" ;;
        "pairwise")     echo "config/trader.pairwise.yaml" ;;
        "aggressive")   echo "config/trader.aggressive.yaml" ;;
        "ag")           echo "config/trader.ag2502.ag2504.yaml" ;;
        "al")           echo "config/trader.al2502.al2503.yaml" ;;
        "rb")           echo "config/trader.rb2505.rb2510.yaml" ;;
        *)              echo "" ;;
    esac
}

# ═══════════════════════════════════════════════════════════
# 函数: 显示使用帮助
# ═══════════════════════════════════════════════════════════
show_help() {
    cat << EOF
${BLUE}═══════════════════════════════════════════════════════════${NC}
${GREEN}QuantlinkTrader 快捷启动脚本${NC}
${BLUE}═══════════════════════════════════════════════════════════${NC}

Usage: ./start_trader.sh [strategy_name]

${YELLOW}Available Strategies:${NC}

  ${GREEN}prod${NC}       - 生产环境（Passive策略）
  ${GREEN}test${NC}       - 测试环境（Pairwise Arb，全天运行）
  ${GREEN}passive${NC}    - 被动做市策略
  ${GREEN}pairwise${NC}   - 配对套利策略
  ${GREEN}aggressive${NC} - 激进交易策略
  ${GREEN}ag${NC}         - 白银配对（ag2502/ag2504）
  ${GREEN}al${NC}         - 铝配对（al2502/al2503）
  ${GREEN}rb${NC}         - 螺纹钢配对（rb2505/rb2510）

${YELLOW}Examples:${NC}

  ./start_trader.sh test              # 启动测试环境
  ./start_trader.sh ag                # 启动白银配对交易
  ./start_trader.sh                   # 显示此帮助信息

${YELLOW}Other Commands:${NC}

  ./start_trader.sh stop              # 停止所有trader
  ./start_trader.sh status            # 查看运行状态
  ./start_trader.sh logs [strategy]   # 查看日志

${BLUE}═══════════════════════════════════════════════════════════${NC}
EOF
}

# ═══════════════════════════════════════════════════════════
# 函数: 停止所有trader
# ═══════════════════════════════════════════════════════════
stop_all_traders() {
    echo -e "${YELLOW}停止所有trader进程...${NC}"

    if pgrep -f "bin/trader" > /dev/null; then
        pkill -TERM -f "bin/trader"
        sleep 2

        # 检查是否还有残留进程
        if pgrep -f "bin/trader" > /dev/null; then
            echo -e "${YELLOW}强制杀死残留进程...${NC}"
            pkill -9 -f "bin/trader"
        fi

        echo -e "${GREEN}✓ 所有trader已停止${NC}"
    else
        echo -e "${BLUE}没有运行中的trader${NC}"
    fi
}

# ═══════════════════════════════════════════════════════════
# 函数: 查看运行状态
# ═══════════════════════════════════════════════════════════
show_status() {
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}Trader 运行状态${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

    if pgrep -f "bin/trader" > /dev/null; then
        echo -e "${GREEN}运行中的Trader进程:${NC}"
        ps aux | grep "bin/trader" | grep -v grep | while read line; do
            pid=$(echo "$line" | awk '{print $2}')
            config=$(echo "$line" | grep -o 'config/[^ ]*' || echo "未知配置")
            echo -e "  PID: ${YELLOW}$pid${NC}  Config: ${BLUE}$config${NC}"
        done

        echo ""
        echo -e "${GREEN}API端口监听:${NC}"
        lsof -i :9201 2>/dev/null | grep LISTEN || echo "  端口9201: 未监听"
        lsof -i :9202 2>/dev/null | grep LISTEN || echo "  端口9202: 未监听"
        lsof -i :9203 2>/dev/null | grep LISTEN || echo "  端口9203: 未监听"
    else
        echo -e "${YELLOW}没有运行中的trader${NC}"
    fi

    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
}

# ═══════════════════════════════════════════════════════════
# 函数: 查看日志
# ═══════════════════════════════════════════════════════════
show_logs() {
    local strategy=$1
    local log_file

    if [ -z "$strategy" ]; then
        # 没有指定策略，显示所有日志文件
        echo -e "${GREEN}可用的日志文件:${NC}"
        ls -lh log/trader*.log 2>/dev/null || echo "没有找到日志文件"
        return
    fi

    # 根据策略名查找日志文件
    case "$strategy" in
        "test")
            log_file="log/trader.test.log"
            ;;
        "prod"|"passive")
            log_file="log/trader.92201.log"
            ;;
        *)
            log_file="log/trader.$strategy.log"
            ;;
    esac

    if [ -f "$log_file" ]; then
        echo -e "${GREEN}查看日志: $log_file${NC}"
        echo -e "${YELLOW}(Ctrl+C 退出)${NC}"
        tail -f "$log_file"
    else
        echo -e "${RED}日志文件不存在: $log_file${NC}"
        echo ""
        show_logs
    fi
}

# ═══════════════════════════════════════════════════════════
# 函数: 启动策略
# ═══════════════════════════════════════════════════════════
start_strategy() {
    local strategy=$1
    local config_file=$(get_config_file "$strategy")

    if [ -z "$config_file" ]; then
        echo -e "${RED}错误: 未知的策略名称 '$strategy'${NC}"
        echo ""
        show_help
        exit 1
    fi

    if [ ! -f "$config_file" ]; then
        echo -e "${RED}错误: 配置文件不存在: $config_file${NC}"
        exit 1
    fi

    if [ ! -f "bin/trader" ]; then
        echo -e "${RED}错误: 找不到 bin/trader 可执行文件${NC}"
        echo -e "${YELLOW}请先编译: cd golang && go build -o bin/trader cmd/trader/main.go${NC}"
        exit 1
    fi

    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}启动策略: $strategy${NC}"
    echo -e "${BLUE}配置文件: $config_file${NC}"
    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"

    # 检查是否有运行中的trader
    if pgrep -f "bin/trader" > /dev/null; then
        echo -e "${YELLOW}检测到运行中的trader进程${NC}"
        read -p "是否停止现有进程并启动新策略? (y/N): " confirm
        if [[ $confirm == [yY] || $confirm == [yY][eE][sS] ]]; then
            stop_all_traders
            sleep 1
        else
            echo -e "${RED}已取消启动${NC}"
            exit 0
        fi
    fi

    # 启动trader
    echo -e "${GREEN}正在启动...${NC}"
    ./bin/trader -config "$config_file" > /tmp/trader_startup_$$.log 2>&1 &
    TRADER_PID=$!

    # 等待启动
    sleep 3

    # 检查进程是否存活
    if ps -p $TRADER_PID > /dev/null; then
        echo -e "${GREEN}✓ Trader启动成功!${NC}"
        echo -e "${BLUE}  PID: $TRADER_PID${NC}"

        # 检查API端口
        sleep 2
        if lsof -i :9201 > /dev/null 2>&1; then
            echo -e "${GREEN}✓ API服务器已启动 (端口 9201)${NC}"
            echo ""
            echo -e "${YELLOW}Web UI 地址:${NC}"
            echo -e "  ${BLUE}http://localhost:3000/?api=http://localhost:9201${NC}"
        else
            echo -e "${YELLOW}⚠ API服务器未检测到，请查看日志${NC}"
        fi

        echo ""
        echo -e "${YELLOW}查看日志:${NC}"
        echo -e "  ./start_trader.sh logs $strategy"
        echo ""
        echo -e "${YELLOW}停止策略:${NC}"
        echo -e "  ./start_trader.sh stop"
    else
        echo -e "${RED}✗ Trader启动失败!${NC}"
        echo -e "${YELLOW}启动日志:${NC}"
        cat /tmp/trader_startup_$$.log
        exit 1
    fi

    echo -e "${BLUE}═══════════════════════════════════════════════════════════${NC}"
}

# ═══════════════════════════════════════════════════════════
# 主程序
# ═══════════════════════════════════════════════════════════
main() {
    local command=$1

    case "$command" in
        "stop")
            stop_all_traders
            ;;
        "status")
            show_status
            ;;
        "logs")
            show_logs "$2"
            ;;
        "help"|"-h"|"--help"|"")
            show_help
            ;;
        *)
            start_strategy "$command"
            ;;
    esac
}

# 运行主程序
main "$@"
