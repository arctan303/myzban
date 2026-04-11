# 🚀 ProxyNode Manager (PNM) 部署与配置指南

该系统采用松耦合架构：Go 编写的底层节点控制器 (PNM) + Next.js 编写的独立管理面板。

---

## 🌟 一、 架构速览
*   **节点端 (Node / PNM)**: 运行在每台充当代理的境外服务器上。负责管理 Xray (VLESS Reality) 和 Hysteria2 的启停、自动测流及下发配置。它暴露一个带有 Admin Token 鉴权的 API (默认 `:9090`)。
*   **面板端 (Panel)**: 运行在一台中央服务器上（甚至可以在同一台机器），提供可视化 UI。管理员可在面板上统筹全局，并连接绑定分布在各地的节点。

---

## 🌐 二、 节点端 (Node) 部署教程

**前期准备**：一台干净的 Ubuntu/Debian 境外 VPS。建议先解析一个你的域名并开放对应防火墙端口。

### 1. 一键安装 PNM (节点控制器)
以 `root` 用户登录节点服务器，执行项目提供的安全安装脚本：
```bash
curl -fsSL https://your-domain/install.sh | bash
```

### 2. 初始化核心安全凭证
系统安装成功后，守护进程 `pnm serve` 会在后台监听 `0.0.0.0:9090`，在面板连接前，我们需要生成用于 API 通讯的安全 Token：
```bash
pnm token init
```
*执行上述命令后，终端会打印出一长串随机字符串。**请务必将该 Token 复制并妥善保管**，后续面板连接节点时需要用到它。*

### 3. (可选) 安装基础代理组件
您可以通过面板可视化安装代理协议，也可以直接使用 PNM 命令行预装好协议内核：
```bash
pnm install vless     # 安装 Xray v1.8+ (VLESS Reality Protocol)
pnm install hy2       # 安装 Hysteria 2
```
*   **防火墙放行**：
    *   PNM API 端口：`9090` (默认 TCP)
    *   VLESS 默认端口：`8443` (TCP)
    *   Hysteria2 默认端口：`8443` (UDP)

---

## 💻 三、 面板端 (Panel) 部署教程

**前期准备**：面板所在的机器需要提前安装好 `Docker` 及 `Docker Compose` 环境。

### 1. 克隆代码并进入目录
如果您选择在独立机器上运行面板：
```bash
git clone https://github.com/arctan303/myzban.git
cd myzban/panel
```

### 2. 通过 Docker Compose 一键启动
在 `panel` 目录下，执行如下命令：

```bash
docker-compose up -d --build
```
**配置释义：**
*   使用了 `network_mode: host`，面板将直接监听宿主机的 `3000` 端口。
*   SQLite 数据被持久化挂载到了容器本地的 `/data/panel.db`。

### 3. 配置 Nginx 反向代理 (推荐)
为确保面板后期的安全性（避免中间人抓包窃取 Admin Token），建议使用 Nginx 为 `localhost:3000` 配置一层 HTTPS 访问。

```nginx
server {
    listen 80;
    server_name panel.your-domain.com;
    
    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        # ... 后续请套上 Let's Encrypt SSL 证书
    }
}
```

---

## 🔗 四、 面板与节点的连接联调

面板成功运行后，我们需要将刚才部署好的节点“挂载”到管理系统下。

1. **登录控制面板**
   * 在浏览器访问您配置的面板地址：`http://面板IP:3000` 或 `https://panel.your-domain.com`。
   * 使用系统默认生成的后台管理员账号登入。

2. **新增节点 (Add Node)**
   * 进入面板的【节点管理 / Node Manager】菜单，点击【新增节点 / Add Node】。
   * **填写信息：**
     * **Node Name / 备注**: (例：`US-洛杉矶-优化组`)
     * **Hostname / IP**: `http://您的节点IP:9090`
     * **Admin Token**: 粘贴您在**节端部署第2步**中用 `pnm token init` 生成的长串秘钥。

3. **测试连接 (Ping)**
   * 保存前测试连接。当您看到 `Connected` 或相应的探针在线数据被正常读取时，即代表**连接打通成功**。

4. **开启用户下发与流量管理**
   * 连接好节点后，您可以直接在可视化面板中点击【添加用户】。
   * 面板会向节点端的 API 下发 `POST /api/v1/users` 创建用户并自动提供对应的 `Clash / Meta` 节点配置文件供用户使用。