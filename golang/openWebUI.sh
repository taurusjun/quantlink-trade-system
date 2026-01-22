#!/bin/bash
# 快速打开 Web 控制台

echo "════════════════════════════════════════════════════════════"
echo "QuantlinkTrader Web 控制台"
echo "════════════════════════════════════════════════════════════"
echo ""

# 检查 web/control.html 是否存在
if [ ! -f "web/control.html" ]; then
    echo "❌ 错误: web/control.html 不存在"
    exit 1
fi

echo "打开方式:"
echo "  1. 直接在浏览器打开 (推荐)"
echo "  2. 启动本地 HTTP 服务器"
echo ""
read -p "请选择 [1/2]: " choice

case $choice in
    1)
        echo ""
        echo "正在打开浏览器..."

        # 检测操作系统并打开浏览器
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS
            open web/control.html
        elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
            # Linux
            if command -v xdg-open &> /dev/null; then
                xdg-open web/control.html
            else
                echo "请手动打开: $(pwd)/web/control.html"
            fi
        else
            echo "请手动打开: $(pwd)/web/control.html"
        fi

        echo "✓ 浏览器已打开"
        echo ""
        echo "如果没有自动打开，请手动访问:"
        echo "  file://$(pwd)/web/control.html"
        ;;

    2)
        echo ""
        echo "启动 HTTP 服务器..."
        echo ""

        # 检查 Python 版本
        if command -v python3 &> /dev/null; then
            PYTHON_CMD="python3"
        elif command -v python &> /dev/null; then
            PYTHON_CMD="python"
        else
            echo "❌ 错误: 未找到 Python"
            exit 1
        fi

        PORT=8000
        echo "服务器地址: http://localhost:${PORT}/control.html"
        echo ""
        echo "按 Ctrl+C 停止服务器"
        echo "════════════════════════════════════════════════════════════"
        echo ""

        cd web
        $PYTHON_CMD -m http.server $PORT
        ;;

    *)
        echo "无效选择"
        exit 1
        ;;
esac

echo ""
echo "════════════════════════════════════════════════════════════"
echo "使用提示:"
echo "  1. 确保 QuantlinkTrader 正在运行"
echo "  2. 检查 API 配置（地址和端口）"
echo "  3. 点击 '连接并刷新状态' 按钮"
echo "════════════════════════════════════════════════════════════"
