# ForgeC2 Maturity Blueprint (v2.6.2)

>对标 Cobalt Strike / Sliver / Mythic / Havoc 的高成熟度 C2 架构蓝图。
>本文是 **设计 + 现有实现引用** 的单一入口；实现细节以源码为准。
>配套生成物：`docs/COMMAND_REFERENCE.md`（`node scripts/gen-command-reference.mjs`）。

**范围声明**：本文面向**授权红队 / 攻防演练**场景。所有 offensive 能力默认受
approval gate、ROE 与审计约束（见 §4 checklist）。

---

## 1. 完整项目目录结构（树形，现状 + 目标增量）

```
forgec2/
├── cmd/
│   ├── server/                 # teamservre 入口
│   ├── sign-release/           # Ed25519 热更新签名（OTA trust root）
│   ├── reset-password/
│   └── …                       # dbq, healthcheck, i18n-tool, checkopenapi
├── internal/
│   ├── config/                 # yaml + env 校验、热重载 token
│   ├── crypto/                 # HKDF / X25519+AES-GCM session / Ed25519 / loot 静态加密
│   ├── db/                     # GORM + sqlite(postgres profile)
│   ├── malleable/              # CS 风格 profile 引擎（URI/transform/UA）
│   ├── obfuscation/            # 一行 payload / PS 混淆
│   ├── payload/                # ★ Agent 生成与实现
│   │   ├── agent/              #   Go implant（主，~310 files）
│   │   │   ├── agent_beacon.go / transport_*.go / task_registry.go
│   │   │   ├── evasion_*.go    #   AMSI/ETW/HWBP/VEH/spoof/callback
│   │   │   ├── sleepmask*.go / ekko_sleep_windows.go
│   │   │   ├── injection_*.go / edr_*.go / cleanup_*.go
│   │   │   └── agent_lifecycle.go  # persistence / uninstall / self_update
│   │   ├── agent_stager/       #   stager（分阶段加载）
│   │   ├── loader/             #   reflective loader 辅助
│   │   └── update_signing.go   #   server 侧 OTA 私钥托管
│   ├── plugin/                 # manifest 插件运行时（stdin JSON / 进程 guard）
│   ├── scripting/              # goja JS 引擎
│   ├── server/                 # Gin + listeners + handlers + opsec/
│   └── webdist/                # go:embed 前端产物
├── pkg/
│   ├── protocol/               # ★ TaskSpec 单一真源（tasks.go + taskspec_data.go）
│   ├── c2pb/                   # gRPC JSON codec envelope
│   └── encoding/
├── proto/
│   ├── c2.proto
│   └── c-implant/              # 轻量 C implant（子集任务）
├── plugins/                    # 52 manifest 插件（recon/report/hook/…）
├── frontend/                   # Vite + React TS（src/pages, src/lib）
├── scripts/
│   ├── gen-command-reference.mjs   # ★ 从 taskspec 生成命令手册
│   ├── gen-capability-matrix.mjs   # ★ 向 CAPABILITY_MATRIX 注入任务清单段（+ VERSION stamp）
│   ├── build-embedded.ps1 / check-webdist.mjs / migrate-taskspec.mjs
│   └── …
├── docs/
│   ├── MATURITY_BLUEPRINT.md   # ← 本文
│   ├── COMMAND_REFERENCE.md    # 生成物
│   ├── CAPABILITY_MATRIX.md    # 质量分级（Core/Hardened/…）
│   ├── DEPLOY.md / TRANSPORT_E2E.md
├── .github/workflows/          # ci.yml（gofmt/vet/test/OpenAPI）+ release.yml
├── docker-compose.yml / Dockerfile / config.yaml / go.mod / Makefile
└── README.md / README.en.md / ARCHITECTURE.md / SECURITY.md / AGENTS.md
```

**目录原则**

| 层 | 目录 | 变更策略 |
|----|------|----------|
| 协议真源 | `pkg/protocol` | 只加 TaskSpec；禁止在 server/agent 各写一份元数据 |
| Implant | `internal/payload/agent` | 文件名 `evasion_*` / `transport_*` / `task_*` 前缀分类 |
| Listener | `internal/server/*_listener.go` | 一协议一文件；统一走 `makeBeaconHandler` |
| 文档 | `docs/` | 生成物标 generated；手写文档带版本头 |

---

## 2. 核心模块详细设计（300–500 字/模块）

### 2.1 Agent Core（`internal/payload/agent`）

Agent 是编译期裁剪的单一 Go 二进制（`package main`），按 `//go:build` 分
Windows/Linux/Darwin/ARM64。核心循环：`agent_beacon.go` 计算 sleep+jitter（
`agent_timing.go` / `sleep_variator`）→ 选传输（`agent_transport.go` 多路复用
HTTP/uTLS/WS/gRPC/QUIC/DNS/ICMP/SSH/SMB）→ 提交 v2 ECDH envelope → 解析响应
任务队列（`agent_queue.go`）→ `task_registry.go` 按 Type 分发 handler → 结果
经同一 envelope 回传。状态机支持 `agent_workinghours`（工作窗）、kill-switch
（`agent_killswitch`）、ghost mode、断线指数退避（`beacon_backoff`）。热更新经
`selfUpdate`（`agent_lifecycle.go`）：仅信任编译期 pin 的 Ed25519 公钥
`updatePinnedPubKeyHex`，拒绝裸 URL。配置热推走 `config_push` + `UpdatePubKey`
持久化。设计目标：**零运行时依赖、可静态编译、崩溃不拖垮 beacon**
（stream handler 全 recover）。

### 2.2 Transport / Listener Fabric（`internal/server/*_listener.go`）

所有 listener 只做三件事：收字节、`LimitReader` 限长、调用共享
`makeBeaconHandler(protocol)`——**传输不引入信任**，鉴权/重放窗/KDF 全在
beacon_core。协议清单：HTTP(S)+uTLS、WSS、gRPC（JSON codec envelope）、H2C、
QUIC（ALPN `h3`/`fc2`，TLS1.3）、DNS、ICMP 分片、UDP、SSH、SMB P2P、TCP/TLS。
失败转移由 circuit-breaker 探测 + extraListeners 映射（`quic://` key 模式）。
QUIC/gRPC 已实现 accept 退避、panic recover、GracefulStop 超时；硬化项见 §3.8–3.9。

### 2.3 Tasking / Command Registry（`pkg/protocol`）

**单一真源**：`tasks.go` 定义 219 个 `TaskType*` 常量；`taskspec_data.go` 注册
217 个 `TaskSpec{Name, Description, Category, Parameters, Aliases, RequiresApproval}`。
Server 元数据 API、approval gate（`tasktypes.go` ~70 个危险类型）、Agent 别名/
help 全部派生自 `AllSpecs()`。重复注册/别名冲突 **panic**（坏构建优于静默影子
命令）。新增命令 = 改这一处 + agent handler + registry 绑定；手册用
`scripts/gen-command-reference.mjs` 再生。

### 2.4 Crypto & Session（`internal/crypto` + `internal/payload/agent/cipher.go`）

握手：注册 v3 = 每 implant `reg_secret` + X25519 ECDH + HKDF-SHA256 多标签派生
（session / file-chain / kill-switch）+ 重放窗；会话 AES-256-GCM，支持 rekey +
LRU 上限。静态 loot AES 加密 + 指纹轮换。发布物 Ed25519。TLS1.3 强制于
QUIC/mTLS 路径。Agent 镜像用 stdlib `crypto/ecdh` + AES-GCM；C implant 走
`curve25519.c` + Windows CNG。

### 2.5 Evasion / OPSEC（`evasion_*.go`, `sleepmask*.go`, `edr_*.go`）

分层：**配置策略**（`evasion_registry` + EDR strategy 开关）→ **主动绕过**
（AMSI/ETW 内存 patch、HW BP + VEH、VEH unhook ntdll、thread spoof、
callback 擦除、blockDLLs、PPID spoof）→ **睡眠混淆**（sleepmask 系列 + Ekko
timer）→ **API 混淆**（apihash、direct syscall stub）→ **流量**（malleable +
cover traffic + jitter + work window）。所有 patch 使用**运行时解码字节**
（`decodeBypassPatch`）避免静态签名。实现要点引用见 §3.5–3.7。

### 2.6 Plugin & Scripting（`internal/plugin`, `internal/scripting`）

三类型：`command` / `hook` / `report`。Manifest YAML 声明 interpreter、
timeout、events、依赖拓扑。执行：stdin JSON、**清空敏感 env**、PATH 去劫持
（`sanitizePluginPATH`）、stdout/stderr 2MiB cap、Windows Job Object
KILL_ON_JOB_CLOSE。52 个打包插件；goja 提供嵌入 JS。缺口：Unix 侧
process-tree kill（见 P4-3）、可选 WASM 沙箱。

### 2.7 Frontend / Operator UX（`frontend/`）

Vite+React19、~54 pages、i18n en/zh、OpenAPI 类型生成、权限键与
`internal/db/models.go` 同步检查。关键页：Agents、Listeners、Generate、
Tasks/Timeline、Lateral、Evasion、Plugins、OPSEC、Workflow、Audit。
嵌入路径 `//go:embed all:dist` ← `internal/webdist/dist`（`scripts/build-embedded.ps1`）。

### 2.8 OTA / Lifecycle（`update_signing.go`, `commands_evasion.go`, `agent_lifecycle.go`）

服务端持有 `data/update_signing.key`（路径跟随 `server.data_dir`，私钥不出目录）；
`POST /api/update-signing/sign` 与 `POST /agents/:id/self_update` 组成推送链：
sha256 → Ed25519 签名 → JSON envelope `{url,signature}` → agent **仅用 pin 公钥**
验证 → 下载替换。`buildLdflags` 在 generate 时注入 `-X main.updatePinnedPubKeyHex`
（构建即钉钥）；`PushUpdateKey` 可再经 config_push 保证跨重启仍 pin。
验签失败按 Error 上报且不 `os.Exit` 成功路径。Server 自身 hot update 走
`update_check.go` + `crypto.update_signing_key`（可选 `require_release_signature`
强制无公钥拒绝热更）；tag 发布 CI 强制 `RELEASE_SIGNING_KEY` 签出 `.sig`。

### 2.9 Audit / Safety（`opsec/guard.go`, approval gates, ROE）

危险任务（~70）强制 approval；`opsec.PreFlight` 规则引擎按风险分级；ROE
CIDR allow/deny；`LogAuditRecord` 覆盖 self_update、lateral 等；日志字段在
`logfields` 统一脱敏。目标：**功能可用 + 责任链可追**。

### 2.10 C Implant（`proto/c-implant`）

MinGW 构建的子集 implant：shell、DNS transport、ECDH/CNG crypto、基本 evade。
定位是「体积/依赖极限」与 E2E 对照，**不承诺** Go Agent 任务对等；malleable
output chain 对 C 不解码（matrix 已标注）。双 implant 策略下以 Go 为功能真源。

---

## 3. 关键技术实现伪代码（≥10，含 `evasion_*.go` 引用）

### 3.1 Beacon 注册 / 会话建立（v3）

```
# 对应: beacon_register / crypto/keys.go / cipher.go
Agent:  eph = X25519.Generate()
        msg = {uuid, eph_pub, reg_secret_id, nonce, ts}
        mac = HMAC(session_key(reg_secret), msg)
Server: 验 reg_secret + 时间窗 → 共享 = X25519(eph, server_eph)
        k_session = HKDF(shared, "session")
        返回 server_eph + encrypted(config) + seq
Agent:  校验 seq 单调；进入 beacon 循环（可 rekey）
```

### 3.2 任务分发（单一真源）

```
# 对应: pkg/protocol/taskspec.go + task_registry.go
onBeaconResponse(tasks):
  for t in tasks:
    canon = ResolveAlias(t.type)          # 别名 → 规范名
    spec  = SpecOf(canon)                 # 未知 → 拒绝并回错误
    if spec.RequiresApproval and not t.approved_token: drop
    handler = registry[canon]             # 缺 handler → "not implemented"
    result  = handler(t.params)           # 全程 recover
    queueResult(t.id, result)             # 超时/重试由 queue 层负责
```

### 3.3 热更新信任根（OTA）

```
# 对应: agent_lifecycle.go verifyUpdateSignature / updatePinnedPubKeyHex
selfUpdate(envelope):
  require updatePinnedPubKeyHex != ""     # 未 pin → 拒绝
  require envelope.signature, url, (optional sha256)
  body = http.Get(url)                    # 超时 + 大小上限
  if envelope.sha256: assert sha256(body)
  assert ed25519.Verify(pin_pub, sha256(body), sig)   # 忽略 envelope.public_key
  atomicReplace(self_path, body); restart
```

### 3.4 断线退避 + 心跳

```
# 对应: beacon_backoff / agent_beacon / workinghours
loop:
  if !inWorkingHours(): sleep(nextWorkSlot); continue
  resp = tryAllTransports(order)          # 主传输失败 → failover
  if resp == nil:
    sleep = min(maxSleep, base * 2^n + jitter)
  else:
    n = 0; sleep = base + jitter
  executeDueTasks(); flushResults()
```

### 3.5 AMSI 绕过（引用 `evasion_amsi_windows.go`）

```
# 实现要点: amsiSessionBypass()
if !patchAMSI (EDR strategy): return disabled
hMod = GetModuleHandle("amsi.dll")
proc = GetProcAddress(hMod, "AmsiOpenSession")
patch = decodeBypassPatch()               # 运行时解码，避免静态 {31 C0 C3}
VirtualProtect(proc, len, PAGE_EXECUTE_READWRITE, &old)
write patch bytes → proc                 # 语义: xor eax,eax; ret → S_OK
# 备选: amsiRegBypass() 通过注册表关 Defender 实时监控 / 清 AMSI provider
#        （需权限，走任务 approval）
```

### 3.6 ETW 绕过（引用 `evasion_etw_windows.go`）

```
# 实现要点: etwNtTraceEvent() — 打 ntdll!NtTraceEvent 比 EtwEventWrite 更深
if !patchETW: return disabled
proc = GetProcAddress(ntdll, "NtTraceEvent")
patch = decodeBypassPatch()               # 同 AMSI，避免静态签名
VirtualProtect + write                   # → STATUS_SUCCESS
# etwRegBypass(): AutoLogger Start=0 + .NET EventSourceEnabled=0（PS 任务）
# 关联: evasion_etwti_windows.go — ETW-TI 回调层
```

### 3.7 Hardware BP + VEH（引用 `evasion_hardwarebp_windows.go` / `evasion_veh_unhook_windows.go`）

```
# HWBP: 不改 .text，适合页守卫/完整性校验环境（仅当前线程）
CONTEXT.Dr0 = &AmsiScanBuffer; Dr7 |= enable DR0
CONTEXT.Dr1 = &EtwEventWrite;  Dr7 |= enable DR1
AddVectoredExceptionHandler(veh):
  if ExceptionCode == SINGLE_STEP:
    if Rip in amsi: Rax = AMSI_RESULT_NOT_DETECTED; skip to ret
    if Rip in etw:  Rax = 0; skip to ret
    return CONTINUE_EXECUTION

# VEH unhook: 从磁盘恢复 ntdll .text，写入时若 AV (0xC0000005) 则
# VirtualProtect(page, RWX) 后 CONTINUE_EXECUTION 重试
```

### 3.8 QUIC listener 硬化（引用 `quic_listener.go` + `transport_quic.go`）

```
# Server
cfg = {MaxIdle:30s, KeepAlive:10s, MaxIncomingStreams: cap}
tls.MinVersion = TLS1.3; ALPN = [h3, fc2]
Accept loop: temp err → backoff×N; permanent → running=false（状态诚实）
stream: recover(); LimitReader(16MiB); handler shared beacon path

# Agent
Dial with MinVersion=TLS1.3; same ALPN
OpenStream → Write → Close() half-close → ReadAll(LimitReader)
fail → return nil → transport failover
```

**P4-1 落地项**：显式 `MaxIncomingStreams`、gRPC keepalive + `MaxConnectionAge`
（经 `keepalive.ServerParameters`）、统一 send/recv 上限；matrix 将稳定路径从
Experimental 升 Hardened/Core。

### 3.9 gRPC listener（引用 `grpc_listener.go`）

```
opts = MaxRecvMsgSize(10MB), JSONCodec, optional Creds(tls/mTLS)
stream loop:
  env = Recv(); empty → InvalidArgument
  resp = beaconHandler(env.Payload)     # 与 HTTP 同一信任边界
  resp == nil → 结束 stream（failover 友好）
  Send(env)
Stop: GracefulStop 超时 5s → Stop()
# P4-1: + keepalive.ServerParameters{Time,Timeout,MaxConnectionAge,Grace}
#        + KeepaliveEnforcementPolicy + MaxSendMsgSize
```

### 3.10 插件进程隔离（引用 `plugin/executor.go`, `job_*.go`）

```
run(plugin):
  env = {PATH: sanitizeAbsoluteOnly, HOME: TMP, GOCACHE…}  # 无 server secrets
  cmd = CommandContext(timeout)
  Start(); guard = attachProcessGuard:  # Win: Job KILL_ON_JOB_CLOSE
                                        # Unix P4-3: Setpgid + Kill(-pgid)
  Wait(); guard.release()
  capture stdout/stderr with 2MiB cap → JSON Result/Report
```

### 3.11 Malleable transform（引用 `internal/malleable`）

```
# URI 轮询 + transform chain + header 序 + UA pool
out = body
for step in chain: out = apply(step, out)   # b64/url/xor/mask/strrep…
place into cookie/query/header via placements[]
request.headers = fixedBrowserOrder + sortedCustom + rotateUA(profile)
server: scan all query/cookie values for decode chain（参数名轮换兼容）
```

### 3.12 Kill-switch / 自毁（引用 `cleanup_*`, `uninstallSelf`）

```
uninstallSelf():
  Windows: 删 Run 键（双命名方案）/ schtasks / Startup exe
           cleanupCredDumpFiles()          # 不留 SAM/lsass.dmp
  best-effort os.Executable() 异步删除
killswitch: 持久化后丢弃 key → 无法解密后续任务 → 冷却退出
```

---

## 4. 安全加固 Checklist（代码审查可勾选）

### 加密与传输
- [ ] TLS1.3 only 于 QUIC / 推荐 HTTP；禁用明文 `grpc://` 于非 lab
- [ ] 会话 X25519 + AES-256-GCM；reg_secret 每 implant 隔离
- [ ] HMAC 覆盖 uuid/nonce/seq；重放窗拒绝旧 seq
- [ ] loot 静态加密 + 密钥轮换；私钥路径权限收紧
- [ ] 发布/OTA 一律 Ed25519；agent **必须** pin 公钥（拒绝裸 URL）
- [ ] uTLS 指纹池与 malleable UA 一致，避免 JA3 孤岛

### 认证与权限
- [ ] JWT + 可选 TOTP；CSRF 双提交（`forgec2_csrf`）
- [ ] RBAC：`RequireRole` 于 settings/self_update/sign
- [ ] mTLS `RequireAndVerifyClientCert` 可配置且失败则拒绝降级
- [ ] 插件与 agent 最小权限；SeDebug 按需、默认不开

### 任务与 OPSEC
- [ ] ~70 危险任务 approval gate 不可绕过（API + UI）
- [ ] `opsec.PreFlight` 在 dispatch 前执行
- [ ] ROE CIDR deny 优先于 allow
- [ ] 日志脱敏（token/secret/凭据不出明文）
- [ ] kill-switch / uninstall 清凭据残留

### 实现卫生
- [ ] 所有 listener：`LimitReader` + panic recover + 状态位诚实
- [ ] 插件：无敏感 env、PATH 白名单、超时、输出 cap、进程树 kill
- [ ] `gofmt` / `go vet`（忽略 agent unsafe.Pointer 既知项）/ 测试 skip 仅已知 flaky
- [ ] OpenAPI / i18n / permissions 前端门禁绿
- [ ] 无密钥入库（gitleaks pre-commit）

---

## 5. 部署与运行指南

### 5.1 本地（开发）

```powershell
# 1) 前端
cd frontend; npm ci; npm run build; cd ..
.\scripts\build-embedded.ps1
.\scripts\check-webdist.mjs

# 2) 服务
go build -o forgec2-server.exe ./cmd/server
# config.yaml: port 8000, tls 按需
.\forgec2-server.exe -config config.yaml
# 日志 logs/forgec2.log；健康检查 GET /health → ok
```

### 5.2 Docker Compose（生产拓扑）

现有 `docker-compose.yml` 已含：`cap_drop: ALL`、`no-new-privileges`、healthcheck、
可选 Postgres profile。**生产加固已落地**为 overlay 文件
`docker-compose.prod.yml`（read_only + tmpfs、mem/pids 上限、强制
`FORGEC2_JWT_SECRET`、存储密钥透传、默认 `0.0.0.0`）：

```bash
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d --build
```

```bash
# 首次
cp config.example.yaml config.yaml   # 或 scripts/setup-dev.sh
export FORGEC2_JWT_SECRET=$(openssl rand -hex 32)
docker compose up -d --build
# 验证
curl -fsS http://127.0.0.1:8000/health
# 可选 DB
DB_PASSWORD=… docker compose --profile postgres up -d
```

### 5.3 Listener 启用矩阵（示例）

| 场景 | config 键 | 端口 |
|------|-----------|------|
| 默认 UI/API | `server.port` | 8000 |
| HTTPS beacon | `tls_enabled` + cert | 443 |
| QUIC/H3 实验 | `quic_enabled`, `quic_addr` | 4433/udp |
| DNS | `dns_enabled`, `dns_domain` | 53/udp |
| gRPC | `grpc_enabled` + TLS | 自定 |

### 5.4 OTA 密钥（两条独立信任根）

**A. Agent self_update**（`data/update_signing.key`，server 自动生成）

- 构建时 `buildLdflags` 注入 `-X main.updatePinnedPubKeyHex=<pub>`（闭合环）。
- 运行期可再经 `config_push` + `push_update_key` 推送/覆写 pin（持久化 `update.key`）。
- 信封 `{url, signature}` 由 `SignUpdateHash(sha256)` 签出；agent fail-closed 验签 + TOCTOU 复检。
- key 路径跟随 `server.data_dir`（与 `stager.key` 一致）。

**B. Server 热更新**（`RELEASE_SIGNING_KEY` ↔ `crypto.update_signing_key`）

```powershell
go run ./cmd/sign-release -gen     # 打印 RELEASE_SIGNING_KEY / update_signing_key
# 私钥进 GitHub Secret；公钥写入 config.yaml crypto.update_signing_key
# CI 对每个 *.sha256 产出 detached .sig；tag 发布强制要求 secret（release.yml）
# performHotUpdate 无公钥时拒绝热更（fail-closed）或显式 allow 例外
```

两套密钥**不可互换**（A 签 SHA-256 摘要；B 签 checksum 文件原文）。

---

## 6. 命令参考手册

**不要手写维护。** 权威来源 `pkg/protocol/taskspec_data.go` +
`internal/server/tasktypes.go`（`dangerousTaskTypes` 并入 approval 标记）：

```bash
node scripts/gen-command-reference.mjs          # → docs/COMMAND_REFERENCE.md
node scripts/gen-command-reference.mjs --check  # CI freshness gate
node scripts/gen-capability-matrix.mjs          # inject matrix task inventory
node scripts/gen-capability-matrix.mjs --check  # CI freshness gate
```

分类（与 registry Category 一致）：`execution, discovery, collection,
credential-access, defense-evasion, lateral-movement, privesc, persistence,
c2, impact, other`。每个条目含 type、aliases、parameters、approval 标记。

当前生成物摘要（v2.6.2）：**217** 任务类型、**57** approval-gated、**3** 别名、
**70** 参数。手册页脚列出的唯一无 spec 常量是 `shell_output` —— 这是**故意的**：
它是 implant 交互式 shell 回传的 RESULT 类型，不可派发（见
`pkg/protocol/tasks.go:410` 与 `docs/COMMAND_REFERENCE.md` 页脚），不是覆盖缺口。

---

## 7. 潜在风险与缓解（按严重度）

| 级别 | 风险 | 缓解 |
|------|------|------|
| **严重** | OTA 私钥泄露 → 供应链投毒 | 私钥仅 `data/update_signing.key`；agent pin 编译期公钥；轮换 + 指纹审计 |
| **严重** | 明文 lab 协议误上生产 | `grpc://` TLS 失败拒绝降级；文档/启动 banner 警告 |
| **高** | 危险任务无 approval 直达 | registry 级 `RequiresApproval` + API/UI 双闸；测试覆盖 |
| **高** | 插件逃逸读取 server env | 已清 env + PATH 消毒；P4-3 进程树 + 可选网络隔离；D2-4 guard fail-closed + WaitDelay + 300s timeout 上限 |
| **高** | Beacon 重放/跨 implant 冒用 | reg_secret 隔离 + seq 窗 + HMAC 覆盖 uuid；D2-3 resync 仅对 AEAD 已验证或会话已丢失的帧签发 |
| **高** | 操作平面策略被 API 绕过 | D2-1 `/api/v1` 接入 operator guard（仅 health 豁免）；审计按租户隔离（D2-2） |
| **中** | QUIC/gRPC 资源耗尽 | MaxIncomingStreams、msg size cap、accept backoff |
| **中** | 静态签名（AMSI/ETW patch 字节） | `decodeBypassPatch` 运行时解码；HWBP 备选路径 |
| **中** | 凭据 dump 落盘残留 | uninstall 清理；approval；loot 加密 |
| **中** | C implant 与文档漂移 | matrix 标明子集；E2E 分轨 |
| **低** | 文档过期（matrix 曾停 v2.4.1） | P4-2 刷新 + 生成命令手册 |
| **低** | 根目录构建杂物 | gitignore + 清理脚本 |

---

## 8. 成熟度评分（1–10）

| 维度 | 分 | 理由 |
|------|----|------|
| 功能成熟度 | **8.5** | 217 任务、11 传输、52 插件、malleable v2、双语言 UI；扣分：C-implant 子集、部分 lateral/tun 非 Win stub |
| 稳定性/健壮性 | **9.0** | 队列/退避/kill-switch/限长/recover 齐全；QUIC stream cap + gRPC keepalive/MaxConnectionAge；Unix 进程组 kill；D2-4 插件 guard fail-closed + WaitDelay + timeout 硬上限；扣分：仍非 WASM 沙箱、无插件 CPU/IO 配额 |
| 生态成熟度 | **7.5** | 命令>50、别名、manifest+goja、en/zh；D2-4 脚本 require 宿主机逃逸已封、事件回调有界；扣分：插件非 WASM 沙箱、依赖宿主解释器、无包签名 |
| 安全与合规加固 | **9.0** | TLS1.3/ECDH/GCM/Ed25519 pin/ROE/approval/审计；可选 ChaCha20-Poly1305；生产 compose；D2-1 `/api/v1` operator guard；D2-2 审计租户隔离；D2-3 gRPC mTLS fail-closed + resync oracle 关闭；扣分：ChaCha20 非默认、mTLS 仅全局而非 per-listener |
| 可维护性/更新性 | **8.5** | 单一 TaskSpec 真源、CI 门禁、gofmt 清债、OTA 签名链闭环；matrix 任务清单生成段 + `--check`；版本行由 `VERSION` 单源 stamp；D2-4 契约测试（storage keys/compose/PG 备份边界）；扣分：matrix 其余手写段仍人工维护、CI 仍无 race/覆盖率阈值 |
| 额外加分 | **8.0** | HTTP/3、别名史、多语言插件、跨平台构建已有；扣分：模块化代理/WASM、Win TUN 未打包 |
| **综合** | **8.8** | P3 工程债已清 + P4 全落地 + Phase C（生产 compose / 可选会话 ChaCha20 / matrix 生成段）+ **Phase D-2 服务端 P0 安全收口**（operator guard 覆盖 `/api/v1`、审计租户隔离、gRPC mTLS fail-closed、resync oracle 关闭、插件/脚本 fail-closed、存储密钥与 compose 契约、PG 备份显式边界） |

---

## 附：Phase B 映射（本蓝图 ↔ 代码）

| 项 | 主题 | 关键文件 |
|----|------|----------|
| P4-1 | QUIC/gRPC 硬化 | `quic_listener.go`, `grpc_listener.go`, `transport_quic.go` |
| P4-2 | matrix 刷新 + 命令手册 | `CAPABILITY_MATRIX.md`, `gen-command-reference.mjs` |
| P4-3 | 插件进程/环境隔离 | `plugin/executor.go`, `job_other.go` |
| P4-4 | OTA 闭环（钉钥 ldflags + CI/release 门禁 + key 路径 + 验签分类） | `generator_ldflags.go`, `cmd/server/main.go`, `task_recon.go`, `ci.yml`, `release.yml` |
| P4-5 | 非 Win stub 可行子集（0 新 stub；darwin 双架构交叉编译门禁 + 构建矩阵） | `ci.yml` smoke, `scripts/build-agent.ps1`, `tun_*` |

*生成说明：设计评审通过后按 P4-* 拆 PR；每个逻辑单元 commit+push。*
