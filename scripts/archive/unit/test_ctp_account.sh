#!/bin/bash
# CTP账号验证脚本

set -e

echo "=== CTP账号验证测试 ==="
echo ""

cd gateway

# 检查CTP SDK
if [ ! -d "third_party/ctp/thostmduserapi_se.framework" ]; then
    echo "❌ 错误: 未找到CTP SDK"
    echo "请先安装CTP SDK到 gateway/third_party/ctp/"
    exit 1
fi

# 编译测试程序
echo "正在编译测试程序..."
clang++ -std=c++11 test_ctp_login.cpp -o test_ctp_login \
    -Ithird_party/ctp/include \
    third_party/ctp/thostmduserapi_se.framework/Versions/A/thostmduserapi_se \
    -Wl,-rpath,third_party/ctp/thostmduserapi_se.framework/Versions/A

if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi

echo "✅ 编译成功"
echo ""

# 运行测试
if [ $# -eq 2 ]; then
    # 命令行参数提供账号
    ./test_ctp_login "$1" "$2"
else
    # 交互式输入
    ./test_ctp_login
fi

# 清理
rm -rf ctp_test_flow

exit_code=$?
cd ..

if [ $exit_code -eq 0 ]; then
    echo ""
    echo "🎉 测试通过！您可以开始开发了。"
    echo ""
    echo "下一步："
    echo "1. 编辑 config/ctp_md.yaml，填写您的账号信息"
    echo "2. 开始CTP网关代码开发"
else
    echo ""
    echo "❌ 测试失败，请检查账号信息。"
fi

exit $exit_code
