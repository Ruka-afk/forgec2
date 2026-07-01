# ForgeC2

[![CI](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml/badge.svg)](https://github.com/Ruka-afk/forgec2/actions/workflows/ci.yml)

[English](./README.md) | [中文](./README.zh.md)

**专业红队作战 C2 框架**

ForgeC2 是一款纯 Go 编写的现代化单二进制 C2 框架，集成完整 Web 控制台、多协议信标、AI 智能助手、插件系统与 50+ Implant 任务类型。

**v2.1.0** — CI/CD · macOS Agent · AI 智能等待 · WebSocket 心跳 · EDR 基础 · P1 侦察任务

---

## v2.1.0 更新亮点

| 模块 | 更新内容 |
|------|----------|
| **AI 智能助手** | 智能任务等待（按 Implant 心跳间隔轮询，最长 60s），对话持久化，可关闭等待 |
| **Shell** | 0 秒实时模式、UTF-8 修复、间隔不一致时提示重新生成 |
| **Web 界面** | WebSocket ping/pong（25s）、20 次重连、WS 正常时停止 HTTP 轮询 |
| **Implant** | macOS `agent_darwin.go`、Linux 自启动持久化、`cookie_export`、`vpn_creds`、增强按键记录 |
| **EDR** | 分块睡眠混淆（`evasion: true` / `FORGEC2_EVASION=1`） |
| **运维** | GitHub Actions CI、Makefile、审计告警（登录锁定、批量删除） |
| **插件** | `plugins/samples/` 3 个 JSON 示例 |
| **开发** | `.grok/skills/` 4 个技能（rebuild-deploy、fix-ui-page、add-i18n、add-ai-tool） |

---

## 功能

### AI 智能助手
- **多模型**：DeepSeek、OpenAI、Claude、通义千问、自定义端点
- **智能等待**：`execute_command` 默认等待任务结果（按 `current_interval` 轮询）；设 `wait_for_result: false` 仅排队
- **流式输出**：SSE + Markdown、推理过程、工具调用可视化
- **对话持久化**：切换页面不丢失历史与生成中草稿

### C2 核心
- HTTP(S)、TCP、DNS、ICMP · P2P 链式通信 · 15+ 可塑配置 · 多监听器

### Implant 能力

| 类别 | 功能 |
|------|------|
| Shell & 系统 | shell、ps、killproc、suspend、resume、reboot |
| 凭据 | creds、mimikatz、kerberoast、dcsync |
| 侦察 (P1) | `cookie_export`（Chrome/Edge）、`vpn_creds`（OpenVPN/cmdkey/WinSCP） |
| 监控 | 截图、按键记录（含窗口标题）、实时屏幕流 |
| 远程 (桩) | `remote_input` + `POST /api/agents/:id/input` |

### 安全
- JWT + TOTP · 速率限制 · 审计日志 · 加密备份
- 登录暴力破解锁定告警 · 批量删除 Implant 告警

---

## 快速开始

```bash
git clone https://github.com/Ruka-afk/forgec2.git
cd forgec2
make build-all    # 或 make build
./forgec2-server -config config/config.yaml
```

默认 **http://localhost:8080**，账号 `admin` / `admin`

---

## 开发

```bash
make test           # 运行测试
make bundle         # 重新打包 JS
make dev            # 开发模式（不打包 JS）
make i18n-check     # 翻译检查
```

### Agent Skills（Grok / Cursor）

全部位于 `.grok/skills/`，可用斜杠命令或自动触发：

| 类别 | Skills |
|------|--------|
| **日常开发** | `rebuild-deploy`、`fix-ui-page`、`debug-forgec2`、`ci-fix`、`e2e-smoke-test` |
| **功能扩展** | `add-task-type`、`add-agent-feature`、`add-ui-page`、`add-api-endpoint`、`add-i18n` |
| **AI 与插件** | `add-ai-tool`、`plugin-task`、`add-manifest-plugin` |
| **C2 与 Implant** | `add-malleable-profile`、`implant-regenerate`、`edr-evasion`、`add-recon-p1` |
| **实时与报告** | `websocket-event`、`report-section`、`remote-desktop` |

---

## 路线图

- [x] 国际化 · 插件 · OpenAPI · TOTP · 备份
- [x] Shell 实时模式 · AI 对话持久化 · 智能任务等待
- [x] macOS Implant（基础：持久化、截屏、osascript）
- [x] EDR 基础规避 · P1 侦察（Cookie/VPN 凭据/增强按键记录）
- [ ] 交互式远程桌面 · IM 窃取 · 表单劫持

---

## 法律声明

**仅限授权的安全测试使用。** 详见 [LICENSE](./LICENSE)。

---

*ForgeC2 — 铸就访问，掌控叙事。*