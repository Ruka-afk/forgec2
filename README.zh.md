# ⚒️ ForgeC2

> 为现代红队铸造的命令与控制平台。

[![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Ruka-afk/forgec2)](https://github.com/Ruka-afk/forgec2/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

[English](README.md) · **中文**

ForgeC2 是一个用纯 Go 编写的自托管、单二进制 C2 平台。一个可执行文件即可承载完整的植入体构建流水线、多协议信标通信、AI 辅助作战控制台和 Next.js Web 界面——无需独立前端服务器、无需数据库引擎、无需额外依赖。

---

## 核心能力

| | |
|---|---|
| 🚀 **单二进制，一切内置** | Next.js 控制台、REST API、信标端点和 SQLite 全部跑在同一个端口上。一个文件即可部署。 |
| 🧬 **按需载荷工厂** | EXE / DLL / PowerShell / ELF / macOS 植入体、XOR 加载器、Shellcode、Donut、One-Liner——浏览器内生成，服务端交叉编译。 |
| 📡 **十种传输** | HTTP(S)、WSS、gRPC、mTLS、H2C、TCP、DNS、ICMP、SSH——外加 SMB/TCP P2P 链式与 Discord/Slack 外部 C2。 |
| 🤖 **内置 AI 副驾** | DeepSeek、OpenAI、Claude 或任意 OpenAI 兼容模型——在聊天中直接指挥行动。 |
| 🛡️ **OPSEC 即特性** | 任务分发前规则引擎、Malleable C2 配置、AMSI/ETW 绕过、睡眠掩码，以及拒绝"默认不安全"的载荷流水线。 |
| 🧩 **可扩展设计** | 40+ 即插即用插件、JavaScript 脚本引擎、工作流自动化、完整 OpenAPI 接口面。 |

---

## 快速开始

**Linux**

```bash
chmod +x forgec2-server-linux-amd64
./forgec2-server-linux-amd64 -config config.yaml
```

**Windows**

```powershell
.\forgec2-server.exe -config config.yaml
```

打开 `http://localhost:8000`——首次启动时服务器会把随机生成的管理员密码打印到控制台。

### 从源码构建

```bash
git clone https://github.com/Ruka-afk/forgec2.git && cd forgec2

# 需要 Go 1.25+ 与 Node.js 20+
powershell -File scripts/build-embedded.ps1   # 前端 → 嵌入 → 单二进制

# ...或容器化部署
docker compose up -d
```

---

## 载荷生成器

ForgeC2 的核心是一套工作台式生成器，把载荷创建变成一条规范的构建流水线：

- **Sticky 连接面板** — 监听器、C2 URL、传输、Malleable 配置、信标时序和密钥始终在视野内
- **构建状态** — 每个产物实时反馈 就绪 / 编译中 / 已完成 / 失败，附带内联结果
- **产物家族** — Agent 二进制（EXE、DLL、PS1、ELF、macOS）、加载器、Shellcode/Donut、One-Liner、一键快速预设
- **传输感知表单** — 选择 WSS、gRPC、SSH、DNS、ICMP、mTLS 或 H2C 时，只显示该传输真正需要的字段
- **全量国际化** — 中英双语，键覆盖由 CI 强制检查

## 植入能力

覆盖标准作战剧本的 50+ 任务类型：

**访问** — Shell、PowerShell、execute-assembly、BOF、PowerPick、PE/CLR 加载、Token 偷取/创建/恢复、凭据、mimikatz、kerberoast、DCSync
**横向** — WMI、WinRM、PsExec、Pass-the-Hash、Pass-the-Ticket、SMB/TCP 中继、SOCKS5、端口转发、NTLM 中继
**持久化** — 注册表、计划任务、启动文件夹、WMI、服务、COM 劫持、IFEO
**规避** — AMSI/ETW 绕过、VEH 反钩子、硬件断点、睡眠掩码、沙箱检测
**监控** — 截图、实时屏幕、窗口标题键盘记录、录制、剪贴板、远程输入
**侦察** — Cookie 导出、VPN/WiFi 凭据、端口扫描、进程树、OS/域发现

逐任务、逐平台完整能力矩阵：[docs/CAPABILITY_MATRIX.md](docs/CAPABILITY_MATRIX.md)

---

## 作战控制台

- **60+ 页面** — 实时图表仪表盘（热力图、OS 分布、任务甘特图、地理、攻击路径）、Agent 舰队管理、文件浏览器、终端、Token 实验室、流量画像
- **多操作员** — RBAC 角色、Agent 锁定、任务认领、审计轨迹
- **自动化** — 工作流引擎、Cron 调度器、自动标签、定时 PDF 报告
- **队友工具** — 战役管理、钓鱼（SMTP + 追踪）、BloodHound 导入、域前置、基础设施重定向器
- **韧性** — 监听器健康断路器、AES-GCM 加密数据库备份、优雅故障转移

## 安全姿态

- 首次启动自动生成管理员密码、JWT Secret 与 TLS 材料——全库无默认凭据
- JWT + bcrypt 会话、TOTP 双因素、CSRF 双提交、SameSite Cookie、严格安全头
- 认证速率限制与 IP 锁定、请求体上限、路径遍历防护、审计日志
- 载荷流水线：crypto/rand 熵源、随机化 PE 节名、就地良性导入注入、AMSI 感知宏生成

---

## 架构一览

```
                    ┌────────────────────────────────────────────┐
   Operators ─────▶ │  ForgeC2 (单二进制, :8000)                  │
                    │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
                    │  │  Web UI  │  │   API    │  │  Beacon  │  │
                    │  │ Next.js  │  │ Gin REST │  │ endpoints│  │
                    │  │ (嵌入)    │  │ + WS + AI│  │          │  │
                    │  └──────────┘  └──────────┘  └──────────┘  │
                    │  SQLite · 插件 · 脚本 · OPSEC               │
                    │  构建队列 → 交叉编译植入体                   │
                    └───────────┬────────────────────────────────┘
                                │ HTTP(S)/WSS/gRPC/mTLS/H2C/TCP/DNS/ICMP/SSH
                    ┌───────────▼────────────────────────────────┐
                    │  Windows / Linux / macOS 植入体（P2P）       │
                    └────────────────────────────────────────────┘
```

深入阅读：[ARCHITECTURE.md](ARCHITECTURE.md)

---

## 配置

所有配置集中在一个 YAML 文件中（以 [config.example.yaml](config.example.yaml) 为参考模板）。要点：

| 键 | 用途 |
|---|---|
| `server.port` / `server.tls_enabled` | 监听地址与 TLS 终止 |
| `server.allowed_origins` / `cookie_domain` | 跨域部署 |
| `implant.default_interval` / `default_jitter` | 信标节律默认值 |
| `ai.provider` / `api_key` / `model` | AI 助手后端 |
| `rate_limit.login.*` | 认证暴力破解防护 |

## 开发

```bash
go build ./...            # 后端
go test ./internal/...    # 测试（务必加 -count=1）
cd frontend && npm run dev  # UI 热重载 :3000
```

仓库卫生由检查门禁保障：`go vet`、OpenAPI 校验（`cmd/checkopenapi`）、前端 CSS/i18n/路径门禁（`npm run check`）。

## 文档与版本

- [CHANGELOG.md](CHANGELOG.md) — 完整版本历史（当前 **v2.5.0**）
- [docs/](docs/) — 传输 E2E 实验、能力矩阵、设计文档
- [CONTRIBUTING.md](CONTRIBUTING.md) — 如何构建、测试与提交代码
- [SECURITY.md](SECURITY.md) — 漏洞披露流程

---

## 法律声明

ForgeC2 **仅限授权安全测试使用**。对任何非自有系统使用前，必须获得所有者的明确书面授权。参见 [LICENSE](LICENSE)。

---

*铸造你的访问，掌控你的叙事。*
