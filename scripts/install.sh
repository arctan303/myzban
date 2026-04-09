#!/bin/bash
# ProxyNode Manager — 一键安装脚本
# 用法: curl -fsSL https://your-domain/install.sh | bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}╔════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║  ProxyNode Manager — 安装脚本          ║${NC}"
echo -e "${YELLOW}╚════════════════════════════════════════╝${NC}"

# Check root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}请使用 root 用户运行！${NC}"
  exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)   GOARCH="amd64" ;;
    aarch64)  GOARCH="arm64" ;;
    arm64)    GOARCH="arm64" ;;
    *)        echo -e "${RED}不支持的架构: $ARCH${NC}"; exit 1 ;;
esac
echo -e "${GREEN}架构: ${ARCH} (${GOARCH})${NC}"

# Download binary
echo -e "${GREEN}[1/3] 下载 pnm...${NC}"
# TODO: Replace with actual release URL
DOWNLOAD_URL="https://github.com/pnm/proxy-node-manager/releases/latest/download/pnm-linux-${GOARCH}"
wget -O /usr/local/bin/pnm "$DOWNLOAD_URL" 2>/dev/null || {
    echo -e "${YELLOW}下载失败，尝试从本地构建...${NC}"
    # If Go is installed, build from source
    if command -v go &> /dev/null; then
        echo "从源码构建..."
        TEMP_DIR=$(mktemp -d)
        cd "$TEMP_DIR"
        git clone https://github.com/pnm/proxy-node-manager.git .
        CGO_ENABLED=1 go build -o /usr/local/bin/pnm ./cmd/pnm/
        cd /
        rm -rf "$TEMP_DIR"
    else
        echo -e "${RED}需要 Go 编译器或有效的下载链接${NC}"
        exit 1
    fi
}
chmod +x /usr/local/bin/pnm

# Create config directory
echo -e "${GREEN}[2/3] 初始化配置...${NC}"
mkdir -p /etc/pnm

# Create systemd service
echo -e "${GREEN}[3/3] 创建系统服务...${NC}"
cat > /etc/systemd/system/pnm.service <<EOF
[Unit]
Description=ProxyNode Manager
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/local/bin/pnm serve
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable pnm
systemctl start pnm

echo ""
echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ✅ ProxyNode Manager 安装成功！       ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "快速开始："
echo -e "  ${YELLOW}pnm install vless${NC}      # 安装 VLESS Reality"
echo -e "  ${YELLOW}pnm install hy2${NC}        # 安装 Hysteria2"
echo -e "  ${YELLOW}pnm user add alice${NC}     # 添加用户"
echo -e "  ${YELLOW}pnm user info alice${NC}    # 查看用户配置 (含Clash YAML)"
echo -e "  ${YELLOW}pnm user list${NC}          # 列出所有用户"
echo -e "  ${YELLOW}pnm proxy status${NC}       # 查看代理状态"
echo -e "  ${YELLOW}pnm traffic show${NC}       # 查看流量统计"
echo ""
echo -e "API 接口: http://127.0.0.1:9090/api/v1/status"
echo ""
