# ⚒️ ForgeC2

> 为现代红队铸造的命令与控制平台。

[![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Ruka-afk/forgec2)](https://github.com/Ruka-afk/forgec2/releases)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**中文** · [English](README.en.md)

ForgeC2 是一个自托管的单二进制 C2 平台，纯 Go 编写。一个可执行文件里装下加固的植入体构建管线、多协议信标、预算可控的 AI 作战台和完整的 Vite + React Web 控制台——没有前端服务要守，没有数据库引擎要哄，没有依赖要哄着跑。

> ⚠️ **仅限授权安全测试**：必须持有目标所有者的明确书面授权。详见文末[法律声明](#法律声明)。

---

## 开箱即得

| | |
|---|---|
| 🚀 **单文件即全部** | React 控制台、REST API、信标端点、SQLite——全部只占一个端口，一个文件即部署。 |
| 🧬 **按需载荷工厂** | EXE / DLL / PowerShell / ELF / macOS 植入体、XOR stager、shellcode、Donut、一句话——浏览器里点，服务端交叉编译。 |
| 📡 **九传输＋P2P/外部 C2** | HTTP(S)、WSS、gRPC、mTLS、H2C、TCP、DNS、ICMP、SSH，外加 SMB/TCP P2P 级联与 Discord/Slack 外部 C2。 |
| 🤖 **内置 AI 副驾** | DeepSeek、OpenAI、Claude 或任何 OpenAI 兼容模型——在聊天里直接指挥整场行动，工具调用、预算可控。 |
| 🛡️ **OPSEC 是功能** | 行动前规则引擎、可塑流量画像（malleable）、AMSI/ETW 对抗、睡眠掩码，以及处处防呆的载荷管线。 |
| 🧩 **为扩展而生** | 40+ 即插插件、JavaScript 脚本引擎、工作流自动化、完整 OpenAPI 面。 |

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

打开 `http://localhost:8000`——首次启动时服务端会在控制台打印新生成的管理员密码。

### 自己构建

```bash
git clone https://github.com/Ruka-afk/forgec2.git && cd forgec2

# 需要 Go 1.25+ 和 Node.js 20+
powershell -File scripts/build-embedded.ps1   # 前端 → 内嵌 → 二进制

# 一行全验证 + 构建 + 部署（自动修、跳重型）
powershell -File scripts/verify-all.ps1       # tsc + lint + vet + test + build + health

# ……或容器化
docker compose up -d
```

---

## 载荷工厂

ForgeC2 的核心是一个工作台式的生成器，把载荷制作当正规构建管线对待：

- **粘性连接面板**——监听器、C2 地址、传输、流量画像、信标节拍、密钥，构建时永远在视线内
- **构建状态机**——每个产物报告 Ready / Compiling / Done / Failed，内联结果
- **产物家族**——植入体（EXE、DLL、PS1、ELF、macOS）、stager、shellcode/Donut、一句话、一键快捷预设
- **自定义图标与 JPG 伪装**——上传 `.ico/.png`（≤256KB）或选预设，`photo.jpg.exe` 双扩展名，`rsrc.syso` 注入 `RT_ICON`/`VersionInfo`
- **传输感知表单**——选 WSS、gRPC、SSH、DNS、ICMP、mTLS、H2C 只露出该传输真正需要的字段
- **全部双语**——中英 i18n，CI 强制键覆盖

## 植入体能力

50+ 任务类型，覆盖标准作战手册：

**立足**——shell、PowerShell、execute-assembly、BOF、PowerPick、PE/CLR 加载、令牌窃取/伪造/还原、凭据、mimikatz、kerberoast、DCSync
**横向**——WMI、WinRM、PsExec、PTH、PTT、SMB/TCP 中继、SOCKS5、端口转发、NTLM relay
**持久化**——注册表、计划任务、启动项、WMI、服务、COM 劫持、IFEO
**对抗**——AMSI/ETW 绕过、VEH 脱钩、硬件断点、睡眠掩码、沙箱检测
**监视**——截图、实时屏幕、窗口标题键盘记录、录制、剪贴板、远程输入
**侦察**——cookie 导出、VPN/WiFi 凭据、端口扫描、进程树、OS/域发现

完整分任务、分系统能力矩阵（含 C 植入体差异）：[docs/CAPABILITY_MATRIX.md](docs/CAPABILITY_MATRIX.md)

---

## 作战控制台

- **60+ 页面**——实时图表仪表盘（热力图、OS 分布、任务甘特、地理、攻击路径）、舰队管理、文件浏览器、终端、令牌实验室、流量画像
- **主机详情**——一键侦察（`hostinfo/ps/netstat/users/av`）、运行中 AV 芯片（自动采集）、kill-date 倒计时、P2P 链、快捷睡眠、懒加载截图
- **屏幕与 AI**——Blob URL 流（省 60% 内存）、WS 主通道 + 哈希跳帧（静态桌面省 80% 带宽）、AI 上下文预算条＋工具批处理（展开/折叠/全复制）+ 置顶会话
- **多人协同**——RBAC 角色、主机锁定、任务认领、审计链、租户隔离查询
- **自动化**——工作流引擎、任务调度、自动打标、PDF 报告生成
- **队友工具**——战役、钓鱼（SMTP + 追踪）、BloodHound 摄入、域名前置、基础设施重定向
- **韧性**——监听器健康熔断、AES-GCM 加密数据库备份、优雅故障转移

## 安全姿态

- 首次启动自生成管理员密码、JWT 密钥、TLS 证书——全链路无默认口令
- JWT + bcrypt 会话、TOTP 双因素、CSRF 双提交、SameSite Cookie、严格安全头
- 登录限流与 IP 锁定、包体上限、路径穿越守卫、全量审计日志
- 载荷管线：crypto/rand 熵、随机 PE 节名、原地良性导入注入、AMSI 感知的宏生成

---

## 架构一览

```
                    ┌────────────────────────────────────────────┐
   Operators ─────▶ │  ForgeC2 (single binary, :8000)            │
                    │  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
                     │  │  Web UI  │  │   API    │  │  Beacon  │  │
                     │  │React+Vite│  │ Gin REST │  │ endpoints│  │
                     │  │ (embedded)│ │ + WS + AI│  │          │  │
                    │  └──────────┘  └──────────┘  └──────────┘  │
                    │  SQLite · Plugins · Scripting · OPSEC      │
                    │  Build queue → cross-compiled implants     │
                    └───────────┬────────────────────────────────┘
                                │ HTTP(S)/WSS/gRPC/mTLS/H2C/TCP/DNS/ICMP/SSH
                    ┌───────────▼────────────────────────────────┐
                    │  Windows / Linux / macOS implants (P2P)     │
                    └────────────────────────────────────────────┘
```

深挖：[ARCHITECTURE.md](ARCHITECTURE.md)

---

## 配置

全部收敛在一个 YAML（[config.example.yaml](config.example.yaml) 为准）：

| 键 | 用途 |
|---|---|
| `server.port` / `server.tls_enabled` | 监听地址与 TLS 终结 |
| `server.allowed_origins` / `cookie_domain` | 跨域部署 |
| `implant.default_interval` / `default_jitter` | 信标节拍默认值 |
| `ai.provider` / `api_key` / `model` | AI 助手后端 |
| `rate_limit.login.*` | 登录爆破防护 |

## 开发

```bash
go build ./cmd/server     # 后端（或 go build ./... 全量）
go test ./internal/...    # 测试（带 -count=1 跑）
cd frontend && npm run dev  # UI 热重载 :3000

# 全量一行流（见 scripts/verify-all.ps1）
powershell -File scripts/verify-all.ps1  # vet + tsc + lint + test + build + webdist + health
```

仓库卫生由检查门禁：`go vet`（过滤 payload/agent）、 changed 文件 `gofmt -w`、OpenAPI 校验、前端 CSS/i18n/路径/包体积门（`npm run check` 已并发化）。

## 文档与版本

- [CHANGELOG.md](CHANGELOG.md)——完整发布历史（当前 **v2.6.2**）
- [docs/](docs/)——传输 E2E  lab、能力矩阵、设计文档
- [CONTRIBUTING.md](CONTRIBUTING.md)——构建、测试、发版规范
- [SECURITY.md](SECURITY.md)——漏洞披露

---

## 法律声明

ForgeC2 仅用于**授权安全测试**。对任何系统使用前必须持有所有者的明确书面许可。见 [LICENSE](LICENSE)。

---

*Forge your access. Control your narrative.（铸访问，控叙事。）*
