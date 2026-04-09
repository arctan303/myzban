# ProxyNode Manager (PNM)

多用户代理节点管理工具，支持 **VLESS Reality (Xray)** 和 **Hysteria2** 协议。

## ✨ 功能特性

- 🚀 **一键安装**代理协议（VLESS Reality / Hysteria2）
- 👥 **多用户管理** — 每用户独立 UUID / 密码，自动生成凭证
- 📊 **流量统计** — 实时统计每用户上下行流量，支持限额自动停用
- 🔒 **用户启停** — 启用 / 禁用用户，实时生效
- 📋 **配置输出** — 为每个用户输出 Clash / Meta 格式的 YAML 配置
- 🌐 **REST API** — 完整的 HTTP API，为未来面板远控打基础
- ⏰ **自动过期** — 支持流量限额和时间过期自动禁用

## 📦 快速开始

### 安装 PNM

```bash
# 一键安装 (需要 root)
curl -fsSL https://your-domain/install.sh | bash
```

### 安装代理

```bash
pnm install vless        # 安装 VLESS Reality (Xray)
pnm install hy2           # 安装 Hysteria2
pnm install all           # 全部安装
```

### 用户管理

```bash
pnm user add alice        # 添加用户（自动生成 UUID 和密码）
pnm user list             # 列出所有用户
pnm user info alice       # 查看用户详情 + 客户端配置 YAML
pnm user enable alice     # 启用用户
pnm user disable alice    # 禁用用户
pnm user delete alice     # 删除用户
```

### 查看用户配置

```bash
$ pnm user info alice

╔════════════════════════════════════════╗
║  User: alice                          ║
╚════════════════════════════════════════╝
  Username:      alice
  Email:         alice@pnm
  VLESS UUID:    a1b2c3d4-...
  Hy2 Password:  e5f6a7b8...
  Enabled:       ✅
  Upload:        12.34 GB
  Download:      56.78 GB

── Clash / Meta 客户端配置 (YAML) ──────────
proxies:
  - name: "1.2.3.4-TCP"
    type: vless
    server: 1.2.3.4
    port: 443
    uuid: a1b2c3d4-...
    ...
  - name: "1.2.3.4-UDP"
    type: hysteria2
    server: 1.2.3.4
    port: 8443
    password: e5f6a7b8...
    ...
─────────────────────────────────────────────
```

### 流量统计

```bash
pnm traffic show           # 所有用户流量
pnm traffic show alice     # 指定用户流量 + 历史记录
pnm traffic reset alice    # 重置流量
```

### 代理管理

```bash
pnm proxy status           # 查看代理状态
pnm proxy start vless      # 启动
pnm proxy stop hy2         # 停止
```

### Daemon 模式

```bash
pnm serve                  # 启动 API 服务器 + 流量收集器
```

## 🏗️ 架构

```
┌─────────────────────────────────────────┐
│              CLI (pnm)                  │
├─────────────────────────────────────────┤
│           REST API (:9090)               │
├──────────────┬──────────────────────────┤
│  User Service │  Traffic Collector      │
├──────────────┴──────────────────────────┤
│  Xray Manager  │  Hy2 Manager          │
├─────────────────────────────────────────┤
│           SQLite Database               │
└─────────────────────────────────────────┘
        ↕                    ↕
   Xray (gRPC)      Hysteria2 (HTTP Auth)
```

### 核心设计

- **Xray 用户管理**：通过生成包含所有活跃用户的 `config.json`，启用 `StatsService` 和 `HandlerService` 进行流量统计
- **Hy2 用户管理**：使用 HTTP Auth 模式，PNM 自身作为认证后端（`/hy2/auth`），用户增删即时生效无需重启

## 🔧 REST API

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/v1/status` | 系统状态 |
| `POST` | `/api/v1/proxy/install` | 安装代理 |
| `POST` | `/api/v1/proxy/start` | 启动代理 |
| `POST` | `/api/v1/proxy/stop` | 停止代理 |
| `GET` | `/api/v1/users` | 用户列表 |
| `POST` | `/api/v1/users` | 创建用户 |
| `GET` | `/api/v1/users/{name}` | 用户详情 |
| `DELETE` | `/api/v1/users/{name}` | 删除用户 |
| `POST` | `/api/v1/users/{name}/enable` | 启用 |
| `POST` | `/api/v1/users/{name}/disable` | 禁用 |
| `GET` | `/api/v1/users/{name}/config` | 客户端配置 YAML |
| `GET` | `/api/v1/users/{name}/traffic` | 流量历史 |
| `POST` | `/api/v1/users/{name}/reset-traffic` | 重置流量 |

## 📁 项目结构

```
proxy-node-manager/
├── cmd/pnm/main.go              # CLI 入口
├── internal/
│   ├── config/config.go          # 全局配置
│   ├── db/
│   │   ├── db.go                 # SQLite + CRUD
│   │   └── models.go             # 数据模型
│   ├── proxy/
│   │   ├── manager.go            # 代理管理接口
│   │   ├── xray.go               # Xray VLESS 管理
│   │   └── hysteria2.go          # Hysteria2 管理
│   ├── installer/
│   │   ├── installer.go          # 公共安装工具
│   │   ├── xray_installer.go     # Xray 安装
│   │   └── hy2_installer.go      # Hy2 安装
│   ├── traffic/collector.go      # 流量采集
│   ├── user/service.go           # 用户业务逻辑
│   └── api/server.go             # REST API + Hy2 Auth
├── scripts/
│   ├── install.sh                # 一键安装
│   ├── uninstall.sh              # 卸载
│   └── build.sh                  # 交叉编译
├── go.mod
└── README.md
```

## 🛣️ Roadmap

- [x] Phase 1: 节点端 CLI + API + 多用户管理
- [ ] Phase 2: 订阅链接生成 / 流量限额自动续期
- [ ] Phase 3: Web 面板 + 多节点远程管理

## 📄 License

MIT
