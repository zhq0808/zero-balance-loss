#!/bin/bash

echo "================================"
echo "Zero Balance Loss - 快速启动"
echo "================================"
echo

# 检查Docker服务是否运行
echo "[1/4] 检查Docker服务..."
if ! docker ps > /dev/null 2>&1; then
    echo "❌ Docker服务未运行，请先启动Docker"
    exit 1
fi
echo "✅ Docker服务正常"

# 检查数据库容器
echo
echo "[2/4] 检查MySQL容器..."
if ! docker ps | grep mysql > /dev/null 2>&1; then
    echo "⚠️ MySQL容器未运行，正在启动..."
    cd "$(dirname "$0")/.." || exit
    docker-compose up -d mysql
    echo "⏳ 等待MySQL初始化（15秒）..."
    sleep 15
fi
echo "✅ MySQL容器已就绪"

# 编译程序
echo
echo "[3/4] 编译Go程序..."
go build -o zero-balance-loss
if [ $? -ne 0 ]; then
    echo "❌ 编译失败"
    exit 1
fi
echo "✅ 编译成功"

# 启动程序
echo
echo "[4/4] 启动应用服务..."
echo
echo "========================================"
echo "🚀 应用已启动！"
echo "========================================"
echo
echo "📍 Web界面: http://localhost:8080"
echo "📍 API文档: http://localhost:8080/ping"
echo
echo "💡 使用说明："
echo "  1. 打开浏览器访问 http://localhost:8080"
echo "  2. 右上角代码控制台可切换加锁/无锁模式"
echo "  3. 点击\"发起并发攻击\"观察余额变化"
echo "  4. 对比两种模式的行为差异"
echo
echo "🛑 按 Ctrl+C 停止服务"
echo "========================================"
echo

./zero-balance-loss
