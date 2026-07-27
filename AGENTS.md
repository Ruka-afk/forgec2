# ForgeC2 — Agent 指令手册

> 本文档是 ForgeC2 项目的唯一权威参考。所有 AI agent 在本仓库工作前必须通读此文档。
> 作者视角：20 年红队 C2 框架开发经验。

---

## 1. 项目本质

ForgeC2 是一个**生产级红队 Command & Control 框架**。它不是 Web 应用加了些 C2 功能，而是一个以**操作安全（OPSEC）为核心**的对抗性基础设施。每一个设计决策都必须回答：**这会不会让操作员或 implant 暴露？**

核心能力矩阵：

| 能力域 | 实现 |
|--------|------|
| Agent 通信 | WebSocket、TCP、DNS（DoH/DoT/IPv6）、ICMP、gRPC、SSH、SMB 命名管道、WireGuard |
| 外部 C2 通道 | Discord、Slack、ExtC2 WebSocket 中继 |
| 载荷生成 | EXE/DLL/PS1/Linux/macOS/Shellcode/Donut/Stager/OneLiner，支持 garble 混淆 + UPX 压缩 |
| 进程注入 | Classic DLL Injection、Early Bird、Process Hollowing、Threadless、AtomBombing、Syscall |
| 逃逸技术 | AMSI bypass、ETW patch、VEH hook unhook、Hardware BP、Block DLLs、PPID Spoofing、Syscall stubs |
| 横向移动 | PsExec、WMI、DCOM、WinRM、SCF、SMB、NTLM Relay |
| 权限提升 | Potato 系列（Hot/Print/Juicy/EFSPotato）、UAC Bypass |
| 持久化 | 注册表、计划任务、服务、WMI 事件订阅、DLL 搜索顺序劫持 |
| OPSEC | Sleep Mask（Zilean/Foliage/Advanced）、Egress 检测、沙箱检测、反调试、JARM/JA3 指纹随机化 |
| 任务类型 | 50+ 种（screenshot、keylogger、mimikatz、kerberoast、execute-assembly、BOF、portscan 等） |

**关键认知：这不是一个"有安全功能的应用"，而是一个"以安全为第一性原理的操作平台"。**

---

## 2. 架构全景

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (Next.js 16)                    │
│              Static export → embedded FS (spaFS)          │
├─────────────────────────────────────────────────────────┤
│                   HTTP Server (Gin)                       │
│  ┌──────────┬──────────┬──────────┬──────────┬─────────┐ │
│  │ Auth     │ Rate     │ RBAC     │ CSRF     │ SIEM    │ │
│  │ JWT+2FA  │ Limiter  │ Perm     │ Double   │ Webhook │ │
│  │ Session  │ Per-IP   │ 57 perms │ Submit   │ Export  │ │
│  └──────────┴──────────┴──────────┴──────────┴─────────┘ │
├─────────────────────────────────────────────────────────┤
│                    Transport Layer                        │
│  WebSocket │ TCP │ DNS │ ICMP │ gRPC │ SSH │ SMB │ WG   │
├─────────────────────────────────────────────────────────┤
│                    Payload Engine                         │
│  Generator → Obfuscator → Packer → Artifact Kit          │
│  (garble, UPX, PE timestamp, reflective DLL)             │
├─────────────────────────────────────────────────────────┤
│                    Agent Implant                          │
│  160 files │ Windows/Linux/macOS │ 50+ task handlers     │
│  Sleep Mask │ Syscall Stubs │ Evasion │ Injections       │
├─────────────────────────────────────────────────────────┤
│                    Database (SQLite/PostgreSQL)           │
│  GORM │ 65+ models │ gormigrate │ WAL mode │ TTL cache  │
└─────────────────────────────────────────────────────────┘
```

### 请求生命周期

1. Agent beacon 到达（WS/TCP/DNS/ICMP/gRPC）
2. 载荷加密层解密（ECDH session key 或 XOR）
3. `processBeacon()` 匹配 agent → 取出 pending tasks
4. 任务序列化 → 加密 → 通过相同 transport 返回
5. Agent 执行任务 → 下一次 beacon 回传结果
6. Server 存储结果 → broadcast WS → 触发 plugin hooks

### Task 生命周期

```
pending → running → completed / failed / cancelled
                   ↗ aborted (强制中止时)
```

任务优先级：`0=low, 1=normal, 2=high, 3=urgent`。`fetchPendingTasks` 按 `priority DESC, created_at ASC` 排序。取消 running 任务时自动注入 priority=3 的 `abort` 任务。

---

## 3. 项目结构（精确到文件级）

```
forgec2/
├── cmd/
│   ├── server/main.go           — 入口：config → DB → server.New → SetupRoutes → Run
│   ├── checkopenapi/main.go     — OpenAPI 规范校验
│   ├── dbquery/main.go          — 数据库直接查询工具
│   └── i18n-tool/main.go        — 国际化翻译检查
├── internal/
│   ├── server/                   — 145 个 .go 文件（核心）
│   │   ├── server.go             — Server struct（1881行）+ New() + Run() + Shutdown()
│   │   ├── routes.go             — 16 个路由注册函数，400+ API 端点（946行）
│   │   ├── handlers_auth.go      — 登录/登出/token刷新
│   │   ├── handlers_sessions.go  — 用户会话管理（创建/吊销/列表）
│   │   ├── handlers_beacon.go    — Agent beacon 处理 + fetchPendingTasks
│   │   ├── handlers_websocket.go — WebSocket 连接管理（wsReadPump/wsWritePump）
│   │   ├── handlers_commands.go  — 任务命令分发（取消/重跑/中止）
│   │   ├── handlers_emergency.go — 紧急停止（killswitch）
│   │   ├── handlers_agent_batch.go — 批量操作
│   │   ├── handlers_generate_*.go — 载荷生成（8个文件）
│   │   ├── handlers_files.go     — 文件操作（ls/read/upload/download/delete）
│   │   ├── handlers_monitor.go   — Agent 状态监控 + 离线检测
│   │   ├── handlers_domain_front_stager.go — 域前置 + Stager Token
│   │   ├── handlers_settings.go  — 系统设置 + 密码策略 + TOTP
│   │   ├── handlers_2fa.go       — TOTP 启用/禁用/Backup Codes
│   │   ├── handlers_users.go     — 用户 CRUD + RBAC
│   │   ├── handlers_credentials.go — 凭据管理 + CSV 导出
│   │   ├── handlers_plugins.go   — 插件管理
│   │   ├── handlers_ai.go        — AI 助手（DeepSeek/OpenAI/Claude）
│   │   ├── audit.go              — 审计日志记录
│   │   ├── audit_alerts.go       — 登录锁定 + 告警
│   │   ├── siem.go               — SIEM Webhook 导出
│   │   ├── backup.go             — 数据库备份管理器（加密+定时+清理）
│   │   ├── config_reloader.go    — 配置热重载
│   │   ├── events.go             — 事件系统
│   │   ├── metrics.go            — Prometheus 指标
│   │   ├── extc2.go              — 外部 C2 通道（Discord/Slack）
│   │   ├── tasktypes.go          — 已知任务类型注册
│   │   ├── helpers.go            — 工具函数（csvSanitize 等）
│   │   ├── constants.go          — 全局常量
│   │   ├── middleware/            — auth.go, csrf.go, rate_limit.go, middleware.go
│   │   ├── totp/                 — TOTP + Backup Code（bcrypt hash）
│   │   └── opsec/                — OPSEC Guard
│   ├── db/
│   │   ├── models.go             — 65+ GORM 模型（1297行）
│   │   ├── migrations.go         — gormigrate 迁移（298行）
│   │   ├── database.go           — InitDB / InitDBWithDriver（SQLite + PostgreSQL）
│   │   └── ttlcache.go           — 带 TTL 的查询缓存
│   ├── crypto/
│   │   ├── session.go            — ECDH 会话密钥管理
│   │   ├── loot.go               — Loot 加密（AES-GCM，凭据加密）
│   │   ├── cert.go               — TLS 证书生成
│   │   └── signing.go            — 代码签名
│   ├── payload/
│   │   ├── agent/                — 160 个文件的 Agent implant 源码
│   │   ├── agent_stager/         — Stager 代码
│   │   ├── generator.go          — 载荷生成入口
│   │   ├── obfuscator.go         — 混淆引擎
│   │   ├── shellcode.go          — Shellcode 生成
│   │   ├── donut.go              — Donut loader
│   │   └── profiles/             — Malleable C2 配置
│   ├── config/config.go          — 配置结构 + 加载 + 验证 + 保存
│   ├── plugin/                   — 插件管理器 + Marketplace
│   ├── malleable/                — Malleable C2 profile 解析
│   ├── report/                   — PDF 报告生成
│   ├── logger/                   — 日志系统
│   ├── webdist/                  — 嵌入式前端 dist（//go:embed）
│   └── testutil/                 — 测试辅助
├── pkg/
│   ├── protocol/                 — 共享协议类型（tasks.go, types.go, attack.go）
│   ├── encoding/                 — Beacon 编解码
│   └── c2pb/                     — gRPC 服务定义
├── frontend/                     — Next.js 16 前端（57 个页面）
│   ├── src/app/(main)/           — 主路由组（agents, tasks, settings 等）
│   ├── src/components/           — React 组件 + 37 个 shadcn/ui 原语
│   ├── src/lib/                  — api.ts, i18n/, hooks/, utils
│   └── src/types/                — TypeScript 类型
├── plugins/                      — 50+ 插件（recon/hooks/reports）
├── locales/                      — 服务端 i18n（en/zh）
├── scripts/                      — PowerShell 构建脚本
├── proto/c2.proto                — gRPC proto 定义
└── docs/                         — 设计文档 + Python API + SDK
```

---

## 4. 回答语言规则

与用户交互时**必须使用中文回复**，与用户提问语言保持一致。

**例外**：技术术语、代码、变量名、函数名、文件名、包名、命令行始终使用英文。

---

## 5. Go 后端工作流

### 每次修改 Go 代码后必须执行

```powershell
# 一键执行：build → vet → tidy → 编译 → 重启 → 健康检查
powershell -File scripts/dev-backend.ps1
```

或手动分步：

1. `go build ./...` — 编译检查，有错误必须先修复
2. `go vet ./...` — 静态分析（忽略 `internal/payload/agent/` 的 `unsafe.Pointer` 警告）
3. `go mod tidy` — 清理依赖
4. `go build -o forgec2-server.exe ./cmd/server` — 编译二进制
5. 重启服务（见下方）

### 重启服务

```powershell
taskkill /f /im forgec2-server.exe 2>$null
Start-Sleep -Seconds 1
$proc = Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru
Start-Sleep -Seconds 2
$health = Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -ErrorAction SilentlyContinue
if ($health.status -ne "ok") { Write-Warning "Server health check failed" }
```

### 前端 → 嵌入式工作流

```powershell
# 完整构建（前端 + 嵌入 + Go 二进制 + 重启）
powershell -File scripts/build-embedded.ps1
```

或手动：

1. `cd frontend && npm run build` — 构建前端
2. 将 `frontend/out/` 复制到 `internal/webdist/dist/`
3. `go build -o forgec2-server.exe ./cmd/server` — 编译（自动嵌入）
4. 重启服务

**只改了 Go 代码（未改前端）时，使用 `dev-backend.ps1` 即可。**

---

## 6. 数据库架构

### 驱动选择

- **SQLite**（默认）：纯 Go 驱动 `glebarez/sqlite`，无需 CGO，适合开发和小规模部署
- **PostgreSQL**：通过 `gorm.io/driver/postgres`，适合生产部署

在 `config.yaml` 中配置：
```yaml
database:
  path: data/db/forgec2.db    # SQLite 路径
  driver: sqlite               # "sqlite" 或 "postgres"
  dsn: ""                      # PostgreSQL DSN（driver=postgres 时使用）
```

### 模型约定

- Agent（`Implant`）使用 **UUID 主键**（`BeforeCreate` hook 生成）
- 其他模型使用 **自增 uint 主键**
- 加密字段通过 GORM hook 自动处理（`BeforeCreate`/`AfterFind`/`BeforeUpdate`）
  - 涉及模型：`CredentialEntry.Password`、`CloudCred.Key/Value/Extra`、`ExtC2Channel.BotToken`、`Redirector.SSHKey/SSHPassword`
- 所有模型必须添加到 `db/database.go:InitDB()` 的 `AutoMigrate` 调用中
- 所有模型必须添加到 `internal/testutil/testutil.go` 的测试 DB migration 中
- 所有模型必须添加到 `internal/server/handlers_contract_test.go` 的 `newContractDB` 中

### 迁移约定

使用 `go-gormigrate/gormigrate/v2`。每个迁移有日期字符串 ID：
```go
{
    ID: "2025-07-25-migration-name",
    Migrate: func(tx *gorm.DB) error {
        execMigration(tx, "ALTER TABLE ...", "label")
        return nil
    },
}
```

- 迁移必须**纯增量**（ADD COLUMN、CREATE INDEX）
- 每个迁移**必须有 rollback 函数**（对于索引迁移，DROP INDEX）
- `execMigration()` 会静默忽略 "duplicate column"、"already exists"、"no such table" 错误
- 迁移定义在 `internal/db/migrations.go`

### RBAC 权限模型

- 57 个权限常量（`PermAgentsRead`、`PermPluginsWrite` 等）
- 内置角色：`admin`（全部权限）和 `user`（只读 + 部分写入）
- 自定义角色通过 `CustomRole` 模型 + `RolePermission` 关联
- `requireOperator(c)` = 需要 `admin` 或 `user` 角色
- `requireAdmin(c)` = 需要 `admin` 角色
- 权限检查：`s.requirePermission(c, perm.XXXRead)`

---

## 7. 安全模型

### 认证链

1. **JWT + Session Cookie** — `forgec2_session` cookie，服务端验证
2. **CSRF 防护** — Double-submit cookie 模式（`forgec2_csrf` cookie + `X-CSRF-Token` header）
3. **TOTP 2FA** — 基于 `pquerna/otp`，可选启用
4. **Backup Codes** — bcrypt hash 存储，一次性使用
5. **Session 管理** — 每个登录创建 `UserSession` 记录（SHA-256 token hash），支持单会话吊销
6. **Account Lockout** — IP 级别 + 用户名级别双重锁定
7. **Rate Limiting** — 登录（5次/900秒锁）、API（令牌桶）、Beacon（窗口限流）

### 密码策略

在 `config.yaml` 中配置：
```yaml
password_policy:
  min_length: 8
  require_upper: true
  require_lower: true
  require_digit: true
  require_symbol: false
  max_age_days: 0      # 0 = 不强制轮换
```

验证函数：`server.validatePasswordComplexity(password)` — 读取 `s.cfg.PasswordPolicy`。

### 加密体系

- **传输加密**：ECDH 会话密钥（`config.crypto.key = "ecdh:"`）或 XOR
- **Loot 加密**：AES-GCM（`crypto.InitLootEncryption(jwtSecret)`）
- **密码哈希**：bcrypt（`golang.org/x/crypto/bcrypt`）
- **TLS 指纹**：JARM/JA3 随机化（`utls` 库）
- **Backup Code 哈希**：bcrypt（`totp.HashBackupCode()`）

### 紧急停止（Killswitch）

- `POST /admin/emergency-stop` — 需要密码确认，设置所有在线 agent 的 `kill_date`
- `GET /admin/emergency-status` — 返回在线 agent 数量 + 待处理 kill 数量
- 实现在 `handlers_emergency.go`

---

## 8. 前端架构

详见 `frontend/AGENTS.md`。关键要点：

- **Next.js 16.2 + React 19 + TypeScript 5** — 静态导出（`output: "export"`）
- **Tailwind CSS 4 via PostCSS** — `postcss.config.mjs` + `@tailwindcss/postcss`
- **shadcn/ui（base-nova 风格）** — 37 个组件在 `src/components/ui/`，使用 `cn()` 合并类名
- **CSS 变量主题** — `globals.css` 中 `:root` / `.dark` 定义 oklch 色彩变量
- **暗色模式** — `.dark` class on `<html>`，内联脚本在 body 渲染前从 localStorage 恢复
- **状态管理** — Zustand 全局 store + React hooks 局部状态
- **国际化** — `useI18n()` hook，`t("key")` 翻译，locale 文件在 `frontend/src/lib/i18n/`
- **API 调用** — 封装在 `src/lib/api.ts`，自动附加 session cookie
- **Toast** — `sonner` 库，`toast.success()` / `toast.error()`
- **SPA 路由** — Go 服务器 `spaFS` 将所有非 API 路径回退到 `index.html`
- **动态路由** — `generateStaticParams` + `_` 占位符预渲染

### 前端修改后验证

```bash
cd frontend && npm run build  # 必须无错误
```

---

## 9. 编码约束（硬性规则）





---


## 11. 测试约定

### 测试模式

- **Table-driven tests + 子测试** — 项目标准模式
- **`testing.T` + `t.Helper()`** — 测试辅助函数标记
- **In-memory SQLite** — 测试使用 `:memory:` 数据库
- **`httptest.NewRecorder()`** — HTTP handler 测试使用 gin test 模式

### 测试 DB 初始化

- `internal/testutil/testutil.go` — `SetupTestDB(t)` 函数
- `internal/server/handlers_contract_test.go` — `newContractDB(t)` 函数
- 两者都必须 AutoMigrate **所有** 模型（包括新增的）

### 已知测试失败

以下测试失败是**已知问题**，不要尝试修复（除非用户明确要求）：

| 测试 | 原因 |
|------|------|
| `TestInitJWTSecret` | JWT secret 自动生成逻辑变化 |
| `TestCSRFProtect_PreservesExistingCookie` | CSRF cookie 行为变化 |
| `TestHandleListCredentials*` | `loot encryption not initialized`（需要先调用 `crypto.InitLootEncryption`） |
| `unsafe.Pointer` 警告 | `internal/payload/agent/` 中 Windows 系统调用的正常用法 |

### 运行测试

```bash
go test ./internal/server/... -count=1 -timeout 60s      # 服务器测试
go test ./internal/crypto/... -count=1                    # 加密测试
go test ./internal/db/... -count=1                        # 数据库测试
go test ./internal/plugin/... -count=1                    # 插件测试
```

---

## 12. API 路由概览

`routes.go` 包含 16 个路由注册函数，400+ 端点。关键路由：

| 前缀 | 认证 | 说明 |
|------|------|------|
| `/health`, `/ready` | 无 | 健康检查 |
| `/metrics` | 无 | Prometheus 指标 |
| `/login`, `/logout` | 无/有 | 认证 |
| `/ws`, `/ws/beacon` | Cookie/Header | WebSocket |
| `/api/agents` | RBAC | Agent CRUD + 任务 + 文件操作 |
| `/api/listeners` | RBAC | 监听器管理 |
| `/api/generate` | RBAC | 载荷生成 |
| `/api/credentials` | RBAC | 凭据管理 |
| `/api/settings` | Admin | 系统设置 |
| `/api/users` | Admin | 用户管理 |
| `/api/plugins` | RBAC | 插件系统 |
| `/api/infrastructure` | RBAC | 基础设施（域名前置、证书等） |
| `/api/dashboard` | RBAC | 仪表盘图表 |
| `/admin/emergency-stop` | Admin | 紧急停止 |
| `/debug/pprof/*` | Rate-limited | 性能分析 |

### 认证中间件

```
公开 → CSRFProtect → AuthRequired → RequirePermission(perm)
```

`AuthRequired` 验证 JWT → 检查 `UserSession` 是否被吊销 → 设置 `user`、`user_id`、`user_role` 到 gin.Context。

---

## 13. 关键设计决策（为什么这样做）

### 为什么用 spaFS 而不是独立的前端服务器？
- 单一二进制部署，无需 nginx/reverse proxy
- 减少攻击面（一个端口、一个进程）
- 便于红队在受限环境中快速部署

### 为什么 Agent 使用 WebSocket 而不是 HTTP 长轮询？
- 双向通信，服务器可以主动推送任务
- 更低的延迟（不需要等待 HTTP 响应）
- 更好的隐蔽性（看起来像正常的 WebSocket 流量）

### 为什么用 SQLite 作为默认数据库？
- 零依赖，单文件部署
- 适合红队快速搭建和销毁
- PostgreSQL 作为可选项用于团队协作场景

### Agent 协议与发布兼容性
- 可以修改 `internal/payload/agent/`，但涉及通信协议或任务语义时必须同时评估服务端兼容性。
- 已部署 Agent 可能仍使用旧协议；新增字段应保持可选，破坏性变更必须提供版本协商或兼容路径。
- `unsafe.Pointer` 是 Windows 系统调用的正常实现方式；修改时须以目标平台编译与运行验证为准。

### 为什么要有 Malleable C2？
- 模仿合法应用的 HTTP 流量特征
- 让网络流量看起来像正常的 CDN/云服务请求
- 对抗基于流量特征的检测（JA3、JARM、Content-Type 分析）

---

## 14. 常用命令速查

| 操作 | 命令 |
|------|------|
| 一键后端构建+重启 | `powershell -File scripts/dev-backend.ps1` |
| 一键全量构建+重启 | `powershell -File scripts/build-embedded.ps1` |
| 前端构建 | `cd frontend && npm run build` |
| 前端检查 | `cd frontend && npm run check` |
| Go 编译 | `go build ./...` |
| Go 静态分析 | `go vet ./...` |
| Go 依赖整理 | `go mod tidy` |
| 编译二进制 | `go build -o forgec2-server.exe ./cmd/server` |
| 运行服务器测试 | `go test ./internal/server/... -count=1 -timeout 60s` |
| 运行所有测试 | `go test ./... -count=1 -timeout 60s` |
| 载荷混淆构建 | `powershell -File scripts/build-obfuscated.ps1` |
| 交叉编译 | `make build-cross` |
| i18n 检查 | `cd frontend && npm run check:i18n` |
| CSS 检查 | `cd frontend && npm run check:css` |
| 数据库重置 | `make db-reset` |

---

## 15. 环境要求

| 依赖 | 版本 | 用途 |
|------|------|------|
| Go | 1.25+ | 后端编译 |
| Node.js | 20+ | 前端构建 |
| PowerShell | 5.1+ | 构建脚本（win32 平台） |
| Docker | 24+ | 容器化部署（可选） |
| garble | latest | Agent 混淆构建（可选） |
| UPX | latest | 二进制压缩（可选） |

---

## 16. 已完成的 17 项功能增强

| # | 功能 | 状态 | 关键文件 |
|---|------|------|----------|
| 1 | Backup Codes 持久化 | ✅ | `db/models.go`, `totp/totp.go`, `handlers_2fa.go`, `handlers_auth.go` |
| 2 | 会话吊销 & 单会话 Kill | ✅ | `db/models.go`, `handlers_sessions.go`, `middleware/auth.go` |
| 3 | 按账户锁定（超越 IP） | ✅ | `audit_alerts.go`, `handlers_auth.go` |
| 4 | Killswitch / 紧急停止 | ✅ | `handlers_emergency.go` |
| 5 | 任务优先级队列 | ✅ | `db/models.go` (Priority), `handlers_beacon.go` (ORDER BY) |
| 6 | Agent 重连宽限期 | ✅ | `handlers_beacon.go`, `handlers_monitor.go` |
| 7 | Agent WS 断开处理 | ✅ | `handlers_websocket.go`, `server.go` (handleWSBeaconDisconnect) |
| 8 | 批量操作 | ✅ | `handlers_agent_batch.go` |
| 9 | 强制中止任务 | ✅ | `handlers_commands.go` (abort task injection) |
| 10 | 备份 & 恢复自动化 | ✅ | `backup.go`, `handlers_backup.go` |
| 11 | 结构化日志 & SIEM 导出 | ✅ | `siem.go`, `audit.go`, `config.go` |
| 12 | Toast 通知系统 | ✅ | sonner (已有) |
| 13 | CSV/JSON 导出 | ✅ | 已有多资源导出 |
| 14 | 搜索/筛选/排序 | ✅ | 已有前端实现 |
| 15 | 可配置密码策略 | ✅ | `config.go` (PasswordPolicy), `handlers_settings.go` |
| 16 | Docker + PostgreSQL | ✅ | `db/database.go`, `docker-compose.yml`, `config.go` |
| 17 | 域前置 / 流量变形 | ✅ | `handlers_domain_front_stager.go` (已有) |

---

## 17. 红队操作注意事项

> 以下内容来自实际红队操作经验，是代码之外但同样重要的知识。

### OPSEC 分级

| 级别 | 说明 | 对应实现 |
|------|------|----------|
| L0 | 绝对不能被发现 | Sleep Mask、Syscall Stubs、AMSI/ETW bypass |
| L1 | 可以被怀疑但不能被确认 | JARM/JA3 随机化、Malleable C2、Traffic Profile |
| L2 | 可以被发现但要有延迟 | 低频 beacon、Working Hours、Jitter |
| L3 | 可以被快速发现 | 高权限操作、横向移动、Kerberoast |

### Agent 变更发布要求

- 修改 Agent 通信协议、任务格式或默认行为时，必须维护新旧版本兼容性，或明确提供升级与回退方案。
- 每次 Agent 改动必须完成对应平台的构建验证，以及与当前服务端的 beacon、任务投递、结果回传回归测试。

### Transport 选择原则

| 场景 | 推荐 Transport | 原因 |
|------|----------------|------|
| 高度受限网络 | DNS (DoH/DoT) | 混淆在正常 DNS 流量中 |
| 企业环境 | WebSocket (HTTPS) | 看起来像正常的 Web 流量 |
| 内网横向 | SMB 命名管道 | 利用已有 SMB 基础设施 |
| 高带宽需求 | TCP 直连 | 最低延迟和最大吞吐 |
| 完全隔离 | ICMP | 无防火墙策略限制时使用 |
| P2P mesh | WireGuard + Gossip | 多跳通信，无单点故障 |
