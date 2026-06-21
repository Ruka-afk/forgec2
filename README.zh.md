# ForgeC2

[English](./README.md) | [中文](./README.zh.md)

**专业红队作战 C2 框架**

纯 Go 构建的单二进制 C2 框架。P2P 信标链、DNS 信标、Artifact Kit、AI 智能助手、50+ 任务类型、实时屏幕监控。

**v1.3.0** — AI 助手 · Implant 术语 · 三态状态 · 33 JS 外置 · 安全加固

## 功能

### 🤖 AI 智能助手
- 多模型：DeepSeek、OpenAI、Claude、通义千问、自定义
- 函数调用：自然语言管理 Implant
- SSE 流式输出，支持 Markdown（表格、代码、列表）
- 推理过程展示、对话持久化、Markdown 导出
- 安全：长度上限、工具去重、连续调用限制

### 🏗️ C2 核心
- HTTP(S)、TCP、DNS、ICMP 传输
- P2P 链式通信（SMB 命名管道 / TCP 中继）
- 15+ 可塑 C2 配置（bing、google、office365、teams 等）
- 多监听器独立配置
- 可配置心跳间隔 + 抖动

### 🧠 Implant 能力
| 类别 | 功能 |
|------|------|
| Shell & 系统 | `shell`、`ps`、`killproc`、`suspend`、`resume`、`reboot` |
| 凭据 | `creds`、`mimikatz`、`kerberoast`、`dcsync`、自动入库 |
| 横向移动 | WMI、WinRM、PsExec、Pass-the-Hash、Pass-the-Ticket |
| 令牌 | 窃取、创建、恢复、查询 |
| 执行 | execute-assembly、BOF、PowerPick、PE Loader |
| 持久化 | 注册表、计划任务、启动文件夹、WMI、服务 |
| 监控 | 截图、键盘记录、实时屏幕流 |
| 网络 | SOCKS5 代理、端口扫描、反向端口转发 |

### 🖥️ Web 界面
- 仪表盘三态显示（在线/超时/离线）
- Implant 详情含公网 IP + GeoIP 地图
- Shell 紧凑工具栏 + 命令历史
- 后渗透工具包：40+ 分类命令
- 生成页：共享监听器、跨平台构建
- 审计日志 CSV 导出、凭据密码脱敏显示
- 可折叠侧边栏 + 在线用户面板

### 🔒 安全
- JWT + bcrypt，自动生成密钥，HttpOnly CSRF
- 速率限制、审计日志、路径穿越防护
- 密码永不暴露在 HTML DOM 中
- XSS 防护：DOM textContent 转义
- 33 JS 外置、NoRoute 信标兜底

## 快速开始

```bash
git clone https://github.com/Ruka-afk/forgec2.git
cd forgec2 && go mod tidy && go run ./cmd/server
```

默认 `http://0.0.0.0:8080` · 账号 `admin` / `admin`

## 路线图

- [x] HTTP/HTTPS/TCP/DNS 传输 · P2P 链式通信
- [x] Artifact Kit · 可塑配置 · SOCKS5
- [x] 多用户 RBAC · 协作 · AI 助手
- [x] 安全审计 · JS 外置 · 三态状态
- [x] 公网 IP + GeoIP · 后渗透工具包
- [ ] macOS · EDR 规避

## 法律声明

**仅限授权的安全测试使用。** 须获得明确书面授权。详见 `LICENSE`。

---

*ForgeC2 — 铸就访问，掌控叙事。*
