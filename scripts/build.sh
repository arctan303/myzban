#!/bin/bash
# 构建脚本 — 交叉编译 Linux 二进制文件
# 在本地 Windows/Mac 上构建 Linux 版本

set -e

VERSION=${1:-"dev"}
OUTPUT_DIR="./build"

mkdir -p "$OUTPUT_DIR"

echo "Building ProxyNode Manager v${VERSION}..."

# Linux AMD64
echo "  → linux/amd64"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o "${OUTPUT_DIR}/pnm-linux-amd64" \
    ./cmd/pnm/

# Linux ARM64
echo "  → linux/arm64"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
    -ldflags="-s -w" \
    -o "${OUTPUT_DIR}/pnm-linux-arm64" \
    ./cmd/pnm/

echo ""
echo "Build complete! Binaries in ${OUTPUT_DIR}/"
ls -lh "${OUTPUT_DIR}/"
