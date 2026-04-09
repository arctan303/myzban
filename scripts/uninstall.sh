#!/bin/bash
# ProxyNode Manager — 卸载脚本

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

echo -e "${RED}即将卸载 ProxyNode Manager${NC}"
echo ""

# Stop services
systemctl stop pnm 2>/dev/null || true
systemctl disable pnm 2>/dev/null || true

# Remove files
rm -f /usr/local/bin/pnm
rm -f /etc/systemd/system/pnm.service
systemctl daemon-reload

echo ""
read -p "是否同时删除数据库和配置? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -rf /etc/pnm
    echo -e "${GREEN}配置和数据库已删除${NC}"
fi

echo -e "${GREEN}✅ ProxyNode Manager 已卸载${NC}"
echo ""
echo "注意: 已安装的 Xray 和 Hysteria2 未被卸载。"
echo "如需卸载代理，请先运行: pnm uninstall all"
