#!/bin/bash

# CLIProxyAPI Linux 编译脚本

# 检查是否安装了 Go
if ! command -v go &> /dev/null; then
    echo "错误: Go 未安装，请先安装 Go 1.26 或更高版本"
    exit 1
fi

# 检查 Go 版本
GO_VERSION=$(go version | cut -d ' ' -f 3 | sed 's/go//')
MIN_VERSION="1.26"

if [ "$(printf '%s\n' "$MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$MIN_VERSION" ]; then
    echo "错误: 需要 Go $MIN_VERSION 或更高版本，当前版本: $GO_VERSION"
    exit 1
fi

echo "检测到 Go 版本: $GO_VERSION"

# 设置输出目录
OUTPUT_DIR="./bin"
mkdir -p "$OUTPUT_DIR"

# 获取版本信息（如果有 git）
if command -v git &> /dev/null && [ -d .git ]; then
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
else
    VERSION="dev"
    COMMIT="unknown"
fi
BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "版本信息:"
echo "  Version: $VERSION"
echo "  Commit: $COMMIT"
echo "  Build Date: $BUILD_DATE"

# 设置环境变量以交叉编译为 Linux
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

echo "开始编译 CLIProxyAPI 为 Linux..."

# 编译主程序
go build -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT' -X 'main.BuildDate=$BUILD_DATE' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Version=$VERSION' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Commit=$COMMIT' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.BuildDate=$BUILD_DATE'" -o "$OUTPUT_DIR/CLIProxyAPI-linux-amd64" ./cmd/server/

if [ $? -eq 0 ]; then
    echo "编译成功!"
    echo "Linux 可执行文件已生成: $OUTPUT_DIR/CLIProxyAPI-linux-amd64"
    ls -la "$OUTPUT_DIR/"
else
    echo "编译失败!"
    exit 1
fi

# 如果需要其他架构，取消注释以下行
# echo "编译其他架构..."
# 
# # ARM64
# export GOARCH=arm64
# go build -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.Commit=$COMMIT' -X 'main.BuildDate=$BUILD_DATE' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Version=$VERSION' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Commit=$COMMIT' -X 'github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.BuildDate=$BUILD_DATE'" -o "$OUTPUT_DIR/CLIProxyAPI-linux-arm64" ./cmd/server/
# 
# export GOARCH=amd64  # 重置回 amd64