#!/bin/bash
# 启动简单的 HTTP 服务器来提供 Web UI

PORT=8000

echo "════════════════════════════════════════════════════════════"
echo "启动 Web UI HTTP 服务器"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "服务器地址: http://localhost:${PORT}/control.html"
echo ""
echo "按 Ctrl+C 停止服务器"
echo "════════════════════════════════════════════════════════════"
echo ""

cd web
python3 -m http.server $PORT
