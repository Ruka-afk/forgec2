# ForgeC2

[![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml)

[English](./README.md) | [中文](./README.zh.md)

**专业级 C2 框架，专为授权红队行动设计**

ForgeC2 是一个现代化的单二进制命令与控制框架，采用纯 Go 语言编写。内置完整的 Next.js Web 控制台、多传输信标、AI 助手、插件系统、OPSEC 守卫、脚本引擎、断路器以及 50+ 种植入任务类型——专为授权红队行动和安全研究打造。

**v2.3.0** — 前后端分离 · 跨域部署 · 异常恢复 · 40+ 新模块 · CSRF 防护

---

## 架构

ForgeC2 采用**前后端完全分离**的架构：

| 组件 | 技术栈 | 默认端口 | 路径 |
|------|--------|---------|------|
| **Web UI** | Next.js 16 + React 19 + Tailwind 4 + shadcn/ui | **3000** | `frontend/` |
| **API & C2** | Go (Gin, SQLite, WebSocket) | **8000** | `cmd/server/` |

- 浏览器访问 Next.js 的 `:3000` 端口，API 调用通过 `/api/go/path` 代理转发
- WebSocket 直连 Go 后端 `ws://host:8000`，基于 Token 认证
- **前后端可独立部署**在不同主机/域名上（参见 [跨域部署](#跨域部署)）

### 一键启动（Windows）

```powershell
powershell -ExecutionPolicy Bypass -File .\start.ps1
```

打开 **http://localhost:3000**（UI）和 **http://localhost:8000**（API/健康检查）

### 手动启动

```powershell
# 终端 1 — Go API
go build -o forgec2-server.exe ./cmd/server
.\forgec2-server.exe -config config.yaml

# 终端 2 — Next.js 开发服务器
cd frontend
npm install
npm run dev
```

生产环境构建 UI：`cd frontend && npm run build && npm start`

---

## v2.3.0 更新内容

### 前后端分离

前后端可独立运行在不同主机/域名：

```bash
# Frontend (.env.local)
NEXT_PUBLIC_API_BASE=https://api.example.com
NEXT_PUBLIC_WS_URL=wss://api.example.com

# Backend (config.yaml)
server:
  allowed_origins:
    - "app.example.com"
  cookie_domain: ".example.com"
  tls_enabled: true
```

| 配置项 | 用途 | 默认值 |
|--------|------|--------|
| `NEXT_PUBLIC_API_BASE` | 前端 API 基础 URL | `/api/go`（代理模式） |
| `NEXT_PUBLIC_WS_URL` | WebSocket 基础 URL | `ws://hostname:PORT` |
| `server.allowed_origins` | CORS/WebSocket 来源白名单 | 全部（空 = 允许所有） |
| `server.cookie_domain` | Session Cookie 域名属性 | 空（同主机） |

### 安全加固

| 功能 | 详情 |
|------|------|
| **CSRF 防护** | 双提交 Cookie 模式（`forgec2_csrf` + `X-CSRF-Token` 请求头） |
| **SameSite Cookie** | 所有 Session/CSRF Cookie 使用 `SameSite=Lax` |
| **可配置 CORS** | `allowed_origins` 配置 WebSocket 和 HTTP CORS |
| **Cookie 域名** | `cookie_domain` 配置跨域 Cookie 共享 |
| **请求体限制** | 最大 2MB JSON 请求体 |
| **异常恢复** | 所有 fire-and-forget 协程包裹 `recover()` |

### Bug 修复

| 问题 | 修复 |
|------|------|
| WebSocket Hub 竞态条件 | `sync.Once` 延迟初始化 |
| 速率限制器协程泄漏 | 上下文可取消的清理协程 |
| 断路器协程泄漏 | `Stop()` 方法 + `stopCh` channel |
| SOCKS 中继数据库抖动 | 内存字节累积，每 100 包刷写一次 |
| SOCKS 中继锁竞争 | 每连接快照-清空策略，独立锁 |
| Agent 广播竞态 | `broadcastAgentOnline` 同步调用 |
| 登录锁定内存泄漏 | 定期清理协程 + 上下文取消 |
| 查询缓存无上限 | `sync.Map` 替换为有界 `TTLCache`（1000 条，5 分钟 TTL） |

### 40+ 新模块

详见[功能列表](#功能列表)。

---

## 亮点（v2.2.0 → v2.3.0）

| 领域 | 新增内容 |
|------|----------|
| **部署** | 前后端完全可分离、CDN/源站部署支持 |
| **安全** | CSRF 防护、SameSite Cookie、请求体限制、可配置 CORS/来源 |
| **稳定性** | 异常恢复、协程泄漏修复、有界缓存、减少锁竞争 |
| **外部 C2** | Discord 和 Slack 外部 C2 通道 |
| **运营** | 战役管理、钓鱼攻击、BloodHound、NTLM 中继、容器逃逸、域前置 |
| **自动化** | 任务调度器、工作流引擎、自动标签、定时报告 |
| **RBAC** | 自定义角色、资源级权限门控、协作任务认领 |
| **监控** | Prometheus 指标、日志轮转、流量画像、Agent 健康插件 |
| **插件** | 40+ 插件（侦察、钩子、报告、凭据分析） |
| **前端** | 60+ 新页面、shadcn/ui 组件库、i18n（en/zh/ja/ko/ar）、vitest |

---

## 功能列表

### AI 助手
- **模型**：DeepSeek、OpenAI、Claude、通义千问、自定义 OpenAI 兼容端点
- **函数调用**：列出 Agent、执行命令、查询任务、凭据、监听器、操作员
- **智能等待**：`execute_command` 使用植入 `current_interval` 轮询结果（最大 60s）
- **流式输出**：SSE + Markdown 渲染 + 推理展示 + 工具调用可见
- **持久化**：聊天历史和进行中的草稿切换页面后不丢失

### C2 核心
- **传输层**：HTTP(S)、TCP、DNS、ICMP、gRPC、SSH
- **P2P 链式**：SMB 命名管道 / TCP 中继
- **Malleable 配置文件**：15+ 预设（bing、google、office365、teams、github…）
- **多监听器**：每个监听器独立的主机/端口/配置文件
- **Sleep + Jitter**：每个植入可独立设置，支持 0s 实时模式
- **断路器**：自动监听器健康监控和配置文件轮换
- **外部 C2**：Discord、Slack 通道中继
- **DNS**：DoH、DoT、IPv6 AAAA 记录隧道

### OPSEC 守卫
- 预执行规则引擎——任务分发前验证安全性
- 内置规则：已知危险参数、危险命令模式
- Web UI 快速测试面板验证命令安全性

### 植入能力

| 类别 | 任务 |
|------|------|
| Shell 与系统 | `shell`、`ps`、`killproc`、`suspend`、`resume`、`reboot` |
| 凭据 | `creds`、`mimikatz`、`kerberoast`、`dcsync`、自动入库 |
| 横向移动 | WMI、WinRM、PsExec、Pass-the-Hash、Pass-the-Ticket |
| Token 操作 | 偷取、创建、恢复、whoami |
| 执行 | execute-assembly、BOF、PowerPick、PE Loader、CLR 托管 |
| 持久化 | 注册表、计划任务、启动文件夹、WMI、服务、COM 劫持、IFEO |
| 监控 | 截图、键盘记录（窗口标题）、实时屏幕、录制 |
| 侦察 (P1) | `cookie_export`（Chrome/Edge SQLite）、`vpn_creds`、`wifi_creds` |
| 网络 | SOCKS5 中继、端口扫描、反向端口转发、NTLM 中继 |
| 规避 | AMSI 绕过、ETW 绕过、VEH 反钩子、硬件断点、睡眠掩码、沙箱检测 |
| 远程 | `remote_input`、远程桌面、剪贴板读写 |
| 容器 | Docker 检测、Kubernetes、容器逃逸 |
| 云 | 云凭据收集、Chrome 扩展 C2 |
| Token | Token 偷取/创建/恢复、身份模拟 |

### Web 控制台（Next.js）
- **60+ 页面** — 仪表盘、Agent、Shell、文件、AI、OPSEC、断路器、脚本、插件、战役、钓鱼、BloodHound、工作流、调度器等
- 仪表盘图表（热力图、OS 分布、任务状态、流量、地理、攻击路径、甘特图）
- 批量 Agent 操作、删除、Agent 详情（锁定/备注/Sleep/Spawn/信任/到期时间）
- 主题（亮色/暗色/跟随系统）、i18n（en/zh/ja/ko/ar）、Ctrl+K 搜索
- WebSocket 实时通知、在线操作员面板
- 生成页面：跨平台构建（EXE/DLL/PS1/Linux/macOS）、Malleable 配置锁定
- Agent 组件：健康环、统计网格、任务列表、截图、流量画像
- 90+ 加载/错误边界组件

### 插件
- 即插即用插件，位于 `plugins/` 目录，包含 `manifest.yaml`
- 40+ 插件：侦察（AD、DNS、进程、网络、注册表、服务、共享、Token、WiFi）、钩子（健康监控、异常检测、Burn 检测、凭据轮换）、报告（资产清单、MITRE 映射、网络拓扑、安全态势）
- Web UI：安装、启用/禁用、执行、导入/导出、评审、评分

### 脚本引擎
- 基于 JavaScript 的服务端自动化（goja 运行时）
- `forgec2.*` API：执行任务、查询 Agent、管理监听器
- 超时控制执行（默认 30s）
- Web UI 编辑器和输出面板

### 安全
- JWT + bcrypt、HttpOnly 安全 Session Cookie（SameSite=Lax）
- CSRF 双提交 Cookie 防护
- TOTP 双因素认证 + 备份码
- 首次启动自动生成随机管理员密码（打印到控制台）
- 首次运行自动替换默认 JWT Secret
- CSP、X-Content-Type-Options、X-Frame-Options、Referrer-Policy、Permissions-Policy 安全头
- 分路由速率限制（登录、API、信标）
- IP 登录锁定 + 渐进式延迟
- 审计日志、路径遍历防护
- AES-GCM 加密自动数据库备份
- 请求体大小限制（2MB）

---

## 快速开始

```bash
git clone https://github.com/Ruka-afk/forgec2.git
cd forgec2
go mod tidy
go build -o forgec2-server ./cmd/server
./forgec2-server -config config.yaml
```

打开 **http://localhost:3000**（Next.js UI）。首次启动会自动生成随机管理员密码——**请查看服务器输出获取凭据**。

> 纯 API 模式：`http://localhost:8000`

### Windows 构建

```powershell
go build -o forgec2-server.exe ./cmd/server
.\forgec2-server.exe -config config.yaml
```

### 跨域部署

前后端部署在不同域名：

1. **后端** — 配置允许的来源和 Cookie 域名：

```yaml
server:
  tls_enabled: true
  cert_file: data/server.crt
  key_file: data/server.key
  allowed_origins:
    - "app.example.com"
  cookie_domain: ".example.com"
```

2. **前端** — 指向后端：

```bash
# .env.local
NEXT_PUBLIC_API_BASE=https://api.example.com
NEXT_PUBLIC_WS_URL=wss://api.example.com
```

3. **反向代理**（Nginx/Caddy）— 路由流量：

```
app.example.com  → 前端静态文件（CDN 或服务器）
api.example.com  → Go 后端 :8000
```

---

## 配置

`config.yaml` 关键配置段：

```yaml
server:
  port: 8000
  tls_enabled: false
  offline_threshold: 60      # 秒，超时判定为"掉线"
  allowed_origins: []         # CORS/WebSocket 来源（空 = 允许所有）
  cookie_domain: ""           # Session Cookie 域名
implant:
  default_interval: 0        # 0 = 实时 Shell 模式
  default_jitter: 20
ai:
  enabled: true
  provider: deepseek
  api_key: "sk-..."
  model: deepseek-chat
rate_limit:
  login:
    max_attempts: 5
    lockout_time: 900
```

完整配置模板见项目根目录 `config.yaml`。

---

## AI 助手配置

1. 侧边栏打开 **AI 助手**
2. 点击 **设置**，启用 AI，选择提供商，填入 API Key
3. 保存——页面自动刷新，AI 就绪

助手会立即将植入命令加入队列，**不会**阻塞等待信标间隔。

---

## API 文档

交互式文档：**http://localhost:8000/api/docs**

OpenAPI 规范：`api/openapi.yaml`（同时在 `/api/docs/openapi.yaml` 提供）

认证方式：通过 `POST /login` 获取的 Session Cookie（`forgec2_session`）。

---

## 项目结构

```
forgec2/
├── cmd/server/          # 服务器入口
├── internal/
│   ├── server/          # HTTP 处理器、WebSocket、AI、OPSEC、脚本（80+ 文件）
│   ├── payload/agent/   # 植入源码（Windows / Linux / macOS）
│   ├── plugin/          # 插件运行时
│   ├── scripting/       # JavaScript 脚本引擎
│   ├── db/              # GORM 模型 + SQLite + TTL 缓存
│   ├── config/          # 配置加载器
│   ├── malleable/       # C2 配置引擎（编译器、预设、变换）
│   ├── crypto/          # 加密、签名、Loot 加密
│   └── infrastructure/  # 自动生成重定向器配置（Nginx、Apache、HAProxy、Caddy）
├── frontend/            # Next.js Web UI（60+ 页面）
├── api/openapi.yaml     # REST API 规范
├── plugins/             # 40+ 插件（侦察、钩子、报告）
├── extensions/          # Chrome 扩展
├── locales/             # 国际化文件（en、zh、ja、ko、ar）
├── docs/                # 设计文档、Python API
├── pkg/                 # 共享协议类型、gRPC 服务
└── config.yaml          # 配置模板
```

---

## 开发

```bash
go build ./...           # 构建所有包
go test ./...            # 运行所有 Go 测试
go vet ./...             # 静态分析
go mod tidy              # 清理依赖
```

### 前端开发

```bash
cd frontend
npm install
npm run dev              # 开发服务器 :3000
npm run build            # 生产构建
npm run check:css        # CSS 架构验证
npm run check:i18n       # i18n 键一致性检查
```

### Agent 技能（Grok / Cursor / OpenCode）

所有技能位于 `.grok/skills/` 和 `.opencode/skills/`——通过斜杠命令调用或自动触发：

| 类别 | 技能 |
|------|------|
| **日常开发** | `rebuild-deploy`、`fix-ui-page`、`fix-ui-style`、`debug-forgec2`、`ci-fix`、`e2e-smoke-test`、`release-github`、`agent-build` |
| **功能开发** | `add-task-type`、`add-model`、`add-test`、`add-plugin-task`、`add-api-endpoint`、`add-sidebar-item` |
| **C2 与监听器** | `listener-type`、`add-malleable-profile`、`dns-tunneling`、`socks-proxy-setup`、`port-forward-setup` |
| **植入** | `bypass-module`、`persistence-module`、`lateral-movement-module`、`reverse-shell-handler` |
| **运营模块** | `campaign-wizard`、`phishing-template`、`credential-guardian`、`mass-ops`、`session-replay` |
| **安全** | `go-vet-lint`、`opsec-check`、`rbac-architect`、`credential-parser` |
| **部署** | `docker-deploy`、`https-cert-setup`、`domain-fronter`、`reverse-proxy-config` |
| **实时与报告** | `internal-event`、`report-section`、`auto-notify`、`timeline-analysis` |
| **集成** | `slack-bot-integration`、`telegram-bot-integration`、`jira-integration`、`thehive-integration` |

---

## 部署

### Docker

```yaml
docker compose up -d
```

Compose 文件创建 `forgec2-server`，拉取前端镜像，首次运行自动生成 TLS + JWT 密钥。配置文件为挂载卷中的 `config.yaml`。

### 加固清单
- 生产环境使用反向代理（Nginx/Caddy）终止 TLS
- 设置 `allowed_origins` 限制 WebSocket/CORS 访问
- 所有用户启用 TOTP 双因素认证
- 通过 Settings UI 轮换 JWT Secret
- 定期查看审计日志（`AuditLog` 表）
- 通过 Settings UI 使用 `VACUOM` 和数据库备份

---

## 路线图

- [x] HTTP/HTTPS/TCP/DNS/ICMP/gRPC/SSH 传输 · P2P 链式
- [x] Artifact Kit · Malleable 配置 · SOCKS5
- [x] 多用户 RBAC · 协作 · AI 助手
- [x] i18n · 插件 · OpenAPI · TOTP · 备份
- [x] OPSEC 守卫 · 断路器 · 脚本引擎
- [x] 实时 Shell · AI 聊天持久化 · 智能任务等待
- [x] macOS 植入 · EDR 规避（分块睡眠、VEH 反钩子、硬件断点）
- [x] P1 侦察：Cookie 导出、VPN 凭据、WiFi 密码、增强键盘记录
- [x] 安全升级：自动生成密钥、CSP 安全头、Token 认证 WebSocket
- [x] 死代码清理：移除遗留 Go 模板（纯 Next.js UI）
- [x] **v2.3.0**：前后端分离、跨域部署、CSRF、SameSite Cookie
- [x] **v2.3.0**：战役、钓鱼、BloodHound、NTLM 中继、容器逃逸、工作流
- [x] **v2.3.0**：40+ 插件、Prometheus 指标、日志轮转、任务调度器
- [ ] 交互式远程桌面 (v2)
- [ ] Form Grabber · IM 截获

---

## 法律声明

**仅限授权安全测试使用。** 部署 ForgeC2 前必须获得目标系统的书面授权。参见 [LICENSE](./LICENSE)。

---

*ForgeC2 — 铸造你的访问，掌控你的叙事。*
