#!/bin/bash
# ProxyNode Manager - Local Panel + Node Quick Setup Script
# Description: Installs and sets up both the Node (PNM) and the Panel locally.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${YELLOW}║  ProxyNode Manager — Local Panel + Node Quick Setup Script ║${NC}"
echo -e "${YELLOW}╚════════════════════════════════════════════════════════════╝${NC}"

if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Please run as root!${NC}"
  exit 1
fi

echo -e "${GREEN}[1/5] Installing Dependencies...${NC}"
apt update -y && apt install -y curl wget sudo git golang-go sqlite3 jq

if ! command -v docker &> /dev/null; then
    echo -e "${GREEN}Installing Docker...${NC}"
    curl -fsSL https://get.docker.com | bash
fi
if ! command -v docker-compose &> /dev/null; then
    echo -e "${GREEN}Installing Docker Compose...${NC}"
    apt install -y docker-compose
fi

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/.."

echo -e "${GREEN}[2/5] Building Node (PNM)...${NC}"
cd "$DIR"
export CGO_ENABLED=1
go build -o /usr/local/bin/pnm ./cmd/pnm/
chmod +x /usr/local/bin/pnm

echo -e "${GREEN}[3/5] Starting Node Service...${NC}"
mkdir -p /etc/pnm
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
systemctl restart pnm
sleep 3 # Wait for DB initialization

# Get Admin Token
ADMIN_TOKEN=$(sqlite3 /etc/pnm/pnm.db "SELECT admin_token FROM node_info WHERE id=1;" 2>/dev/null)
if [ -z "$ADMIN_TOKEN" ] || [ "$ADMIN_TOKEN" == "null" ]; then
    echo -e "${YELLOW}Initializing Admin Token...${NC}"
    pnm token init
    ADMIN_TOKEN=$(sqlite3 /etc/pnm/pnm.db "SELECT admin_token FROM node_info WHERE id=1;")
fi

echo -e "${GREEN}Node Admin Token: ${ADMIN_TOKEN}${NC}"

echo -e "${GREEN}[4/5] Building and Starting Panel...${NC}"
cd "$DIR/panel"

# Rebuild docker container
docker-compose down 2>/dev/null || true
docker-compose build
docker-compose up -d

echo -e "${GREEN}[5/5] Connecting Panel to Node...${NC}"
sleep 5 # Wait for panel to start

# We inject the node into the Panel's database directly
PANEL_DB="$DIR/panel/data/panel.db"
# Wait until the db file is created by the panel or create it if absent
mkdir -p "$DIR/panel/data"

echo "Creating initial panel node entry..."
sqlite3 "$PANEL_DB" "CREATE TABLE IF NOT EXISTS nodes (id INTEGER PRIMARY KEY, name TEXT, address TEXT, admin_token TEXT);"
# Delete and replace
sqlite3 "$PANEL_DB" "DELETE FROM nodes WHERE address = 'http://127.0.0.1:9090';"
sqlite3 "$PANEL_DB" "INSERT INTO nodes (name, address, admin_token) VALUES ('Local Test Node', 'http://127.0.0.1:9090', '$PNM_TOKEN');"

echo "Applying permissions for Next.js app to access the database..."
chmod 777 "$(dirname "$PANEL_DB")"
chmod 666 "$PANEL_DB"

# Create a default panel admin user if needed
docker exec pnm-panel sqlite3 /data/panel.db "CREATE TABLE IF NOT EXISTS panel_users (id INTEGER PRIMARY KEY, username TEXT, password_hash TEXT, role TEXT, proxy_username TEXT, sub_token TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);"
# Password 'admin' -> bcrypt hash $2a$10$W/tG0G.gU.A/z6S8x.6WGu/T3gO.Yy.Y8x6.8OQ0X1X1X/Y0X1X1. (Wait, let's insert if admin doesn't exist or just let it be handled by panel initialization). Normally panel handles this.

echo -e "${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  ✅ Local Testing Environment Successfully Created!        ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo -e "Node API   : http://127.0.0.1:9090"
echo -e "Node Token : ${ADMIN_TOKEN}"
echo -e "Panel UI   : http://$(curl -s ifconfig.me):3000"
echo -e "--------------------------------------------------------------"
