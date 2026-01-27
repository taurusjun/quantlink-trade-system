#!/bin/bash
# CTP持仓查询和平仓工具（使用Python封装）

cat > /tmp/query_position.py << 'PYTHON_SCRIPT'
import sys
import time

def print_positions_help():
    print("=" * 50)
    print("CTP持仓查询和平仓工具")
    print("=" * 50)
    print()
    print("由于当前测试程序的限制，建议使用以下方法：")
    print()
    print("1. 查询持仓：")
    print("   - 通过CTP交易客户端查看")
    print("   - 或使用 SimNow 网页版查看")
    print()
    print("2. 平仓方法：")
    print("   已实现的测试程序会自动：")
    print("   - 先开仓1手")
    print("   - 然后立即平仓")
    print()
    print("3. 手动平仓脚本示例：")
    print("   创建一个简单的平仓订单，类似开仓但方向相反")
    print()
    print("=" * 50)

print_positions_help()
PYTHON_SCRIPT

python3 /tmp/query_position.py
