// Panel database — stores node registry and panel user accounts
const Database = require('better-sqlite3');
const path = require('path');
const fs = require('fs');
const crypto = require('crypto');

const DB_PATH = process.env.DB_PATH || '/data/panel.db';

let db = null;

function getDb() {
  if (db) return db;

  const dir = path.dirname(DB_PATH);
  if (!fs.existsSync(dir)) {
    fs.mkdirSync(dir, { recursive: true });
  }

  db = new Database(DB_PATH);
  db.pragma('journal_mode = WAL');

  // Node registry
  db.exec(`
    CREATE TABLE IF NOT EXISTS nodes (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      name TEXT NOT NULL,
      address TEXT NOT NULL,
      admin_token TEXT NOT NULL,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )
  `);

  // Panel settings
  db.exec(`
    CREATE TABLE IF NOT EXISTS settings (
      key TEXT PRIMARY KEY,
      value TEXT NOT NULL
    )
  `);

  // Panel user accounts (for login)
  db.exec(`
    CREATE TABLE IF NOT EXISTS panel_users (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      username TEXT NOT NULL UNIQUE,
      password_hash TEXT NOT NULL,
      role TEXT NOT NULL DEFAULT 'user',
      proxy_username TEXT,
      sub_token TEXT,
      created_at DATETIME DEFAULT CURRENT_TIMESTAMP
    )
  `);

  // Simple migration to add sub_token to existing panel_users
  try {
    const cols = db.prepare("PRAGMA table_info(panel_users)").all();
    if (!cols.find(c => c.name === 'sub_token')) {
      db.exec("ALTER TABLE panel_users ADD COLUMN sub_token TEXT");
    }
  } catch(e) { console.error('DB Migration error:', e); }

  // Generate missing sub_tokens
  const usersWithoutTokens = db.prepare("SELECT id FROM panel_users WHERE sub_token IS NULL OR sub_token = ''").all();
  if (usersWithoutTokens.length > 0) {
    const updateToken = db.prepare("UPDATE panel_users SET sub_token = ? WHERE id = ?");
    usersWithoutTokens.forEach(u => {
      const token = crypto.randomBytes(16).toString('hex');
      updateToken.run(token, u.id);
    });
  }

  // Seed default YAML template if not exists
  const tmplExists = db.prepare("SELECT key FROM settings WHERE key = 'system_yaml_template'").get();
  if (!tmplExists) {
    const defaultTemplate = \`# Clash Meta (Mihomo) 统一全局配置
# 自动通过 Panel 融合节点生成
port: 7890
socks-port: 7891
mixed-port: 7890
allow-lan: true
mode: rule
log-level: info
ipv6: true
external-controller: 127.0.0.1:9090

tun:
  enable: true
  stack: system
  auto-route: true
  auto-detect-interface: true
  strict-route: true
  dns-hijack:
    - any:53
    - tcp://any:53

dns:
  enable: true
  ipv6: true
  listen: :53
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16
  respect-rules: true
  
  fake-ip-filter:
    - "*.lan"
    - "*.local"
    - "+.arctan.top"

  default-nameserver:
    - 223.5.5.5
    - 119.29.29.29

  proxy-server-nameserver:
    - 223.5.5.5
    - 119.29.29.29

  nameserver-policy:
    "geosite:cn":
      - 223.5.5.5
      - 119.29.29.29

  nameserver:
    - https://1.1.1.1/dns-query
    - https://8.8.8.8/dns-query

proxies:
<__PROXIES__>
  - {name: CF官方优选, server: 104.19.61.188, port: 2083, type: vless, uuid: ebaf52dd-4e2c-4c0c-8b69-97a5b6504f9f, tls: true, skip-cert-verify: false, servername: arctan.asia, client-fingerprint: chrome, network: ws, ws-opts: {path: /, headers: {Host: arctan.asia}}}
  - {name: CF官方优选 2, server: 104.18.35.255, port: 2096, type: vless, uuid: ebaf52dd-4e2c-4c0c-8b69-97a5b6504f9f, tls: true, skip-cert-verify: false, servername: arctan.asia, client-fingerprint: chrome, network: ws, ws-opts: {path: /, headers: {Host: arctan.asia}}}
  - {name: CF官方优选 3, server: 108.162.198.123, port: 443, type: vless, uuid: ebaf52dd-4e2c-4c0c-8b69-97a5b6504f9f, tls: true, skip-cert-verify: false, servername: arctan.asia, client-fingerprint: chrome, network: ws, ws-opts: {path: /, headers: {Host: arctan.asia}}}

proxy-groups:
  - name: 🚀 节点选择
    type: select
    proxies:
      - ⚡ 自动选择
      - ✨ 优选节点
<__PROXY_NAMES__>

  - name: ⚡ 自动选择
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies:
<__PROXY_NAMES__>

  - name: ✨ 优选节点
    type: url-test
    url: http://www.gstatic.com/generate_204
    interval: 300
    tolerance: 50
    proxies:
      - CF官方优选
      - CF官方优选 2
      - CF官方优选 3

rules:
  - IP-CIDR,192.168.0.0/16,DIRECT,no-resolve
  - IP-CIDR,10.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR,172.16.0.0/12,DIRECT,no-resolve
  - IP-CIDR,127.0.0.0/8,DIRECT,no-resolve
  - IP-CIDR6,::1/128,DIRECT,no-resolve
  - DOMAIN-SUFFIX,arctan.top,DIRECT
  - GEOSITE,google,🚀 节点选择
  - GEOSITE,youtube,🚀 节点选择
  - GEOSITE,telegram,🚀 节点选择
  - GEOSITE,twitter,🚀 节点选择
  - GEOSITE,github,🚀 节点选择
  - GEOSITE,gfw,🚀 节点选择
  - GEOSITE,steam,🚀 节点选择
  - GEOSITE,openai,🚀 节点选择
  - GEOSITE,netflix,🚀 节点选择
  - DOMAIN-SUFFIX,steamcommunity.com,🚀 节点选择
  - DOMAIN-SUFFIX,netflix.com,🚀 节点选择
  - DOMAIN-SUFFIX,apple.com,🚀 节点选择
  - DOMAIN-SUFFIX,itunes.apple.com,DIRECT
  - DOMAIN-SUFFIX,mzstatic.com,DIRECT
  - DOMAIN-SUFFIX,icloud.com,DIRECT
  - GEOSITE,cn,DIRECT
  - GEOIP,LAN,DIRECT
  - GEOIP,CN,DIRECT
  - DOMAIN-SUFFIX,126.com,DIRECT
  - DOMAIN-SUFFIX,163.com,DIRECT
  - DOMAIN-SUFFIX,baidu.com,DIRECT
  - DOMAIN-SUFFIX,bilibili.com,DIRECT
  - MATCH,🚀 节点选择
\`;
    db.prepare("INSERT INTO settings (key, value) VALUES (?, ?)")
      .run('system_yaml_template', defaultTemplate);
  }

  // Seed default admin account if none exists
  const adminExists = db.prepare('SELECT id FROM panel_users WHERE role = ?').get('admin');
  if (!adminExists) {
    const bcrypt = require('bcryptjs');
    const hash = bcrypt.hashSync('admin', 10);
    db.prepare(
      'INSERT INTO panel_users (username, password_hash, role) VALUES (?, ?, ?)'
    ).run('admin', hash, 'admin');
  }

  return db;
}

module.exports = { getDb };
