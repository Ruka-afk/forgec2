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
│   │   │   ├── sleepmask*.go    #   含 windows 全图 AES 睡眠掩码
│   │   ├── injection_*.go / edr_*.go / cleanup_*.go
│   │   │   └── agent_lifecycle.go  # persistence / uninstall / self_update
│   │   ├── agent_stager/       #   stager（分阶段加载）
│   │   ├── loader/             #   reflective loader 辅助
│   │   └── update_signing.go   #   server 侧 OTA 私钥托管
│   ├── plugin/                 # manifest 插件运行时（stdin JSON / 进程 guard / WASM 层 / 包签名）
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
│   ├── DEPLOY.md / TRANSPORT_E2E.md / LISTENER_TLS.md / REMOTE_DESKTOP.md / CAPACITY.md
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
QUIC/mTLS 路径（https/tls 监听器另可 per-listener 要求并校验客户端证书、指定客户端 CA 或把下限抬到 TLS1.3，只可收紧不可放宽，见 `docs/LISTENER_TLS.md`）。Agent 镜像用 stdlib `crypto/ecdh` + AES-GCM；C implant 走
`curve25519.c` + Windows CNG。

### 2.5 Evasion / OPSEC（`evasion_*.go`, `sleepmask*.go`, `edr_*.go`）

分层：**配置策略**（`evasion_registry` + EDR strategy 开关）→ **主动绕过**
（AMSI/ETW 内存 patch、HW BP + VEH、VEH unhook ntdll、thread spoof、
callback 擦除、blockDLLs、PPID spoof）→ **睡眠混淆**（sleepmask 系列：windows
构建把睡眠改道为间接 `NtDelayExecution` syscall，`enableSleepMask` 策略兼容
全图 AES 掩码 advanced/foliage/zilean 三套 SleepMasker）→ **API 混淆**（apihash、
direct syscall stub）→ **流量**（malleable +
cover traffic + jitter + work window）。所有 patch 使用**运行时解码字节**
（`decodeBypassPatch`）避免静态签名。实现要点引用见 §3.5–3.7。

### 2.6 Plugin & Scripting（`internal/plugin`, `internal/scripting`）

三类型：`command` / `hook` / `report`。Manifest YAML 声明 interpreter、
timeout、events、依赖拓扑。执行：stdin JSON、**清空敏感 env**、PATH 去劫持
（`sanitizePluginPATH`）、stdout/stderr 2MiB cap、Windows Job Object
KILL_ON_JOB_CLOSE + Unix Setpgid、guard 挂载失败即拒绝执行、timeout 300s
服务端上限、并发配额（`plugins.max_concurrent`）。52 个打包插件；goja 提供
嵌入 JS（D2-4 后 require 宿主机逃逸已封、事件回调有界）。**D4-1**：manifest
可带 `digest`（全包文件 sha256）+ Ed25519 `signature`，服务端按
`plugins.trusted_keys` 验签，`require_signed` 可拒绝未签名包，`cmd/sign-plugin`
负责签发。**D4-2**：`interpreter: wasm` 走 wazero 不可信层（无 WASI、16MiB
内存上限、超时、日志/结果封顶），ABI v1 见 `docs/PLUGIN_TIERS.md`。

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

MinGW 构建的子集 implant：shell + 系统管理/持久化、recon 扩展（drives、find、
portscan、clipboard_get/set、get_sleep_mode、usb_enum、usb_drop、session_recon、
token_whoami，输出形状对齐 Go agent）、ECDH/CNG
crypto、基本 evade。定位是「体积/依赖极限」与 E2E 对照，**不承诺** Go Agent 任务
对等；malleable output chain 对 C 不解码（matrix 已标注）。双 implant 策略下以
Go 为功能真源。

门禁与验证：CI `c-implant` job 交叉构建（`-Wall -Wextra -Werror`；`sqlite3.c`
单独编译不作 `-Werror`，其告警不能掩盖 implant 告警）+ 任务字符串断言 +
`C2_TRANSPORT_DNS` 变体语法检查 + 主机 `x25519_selftest` + WASM 示例 clang
构建并经 `cmd/plugin-check -run` 执行；本地 `scripts/e2e-c-implant-local.ps1`
（一次性服务器 + `cmd/mint-reg-secret`）对 12 个任务用例做真实结果断言
（含两条 usb_drop 拒绝语义：拒绝拷贝自身、拒绝非可移动目标卷），无需操作台
UI。DNS TXT 传输模块 `dns_transport.c` 已具备 Go 线级对等，但尚未接入 beacon
主循环（默认构建不含 dnsapi）。

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

> 2026-10-10 逐项对照源码核实并勾选。勾选项均有测试/门禁佐证；
> 括号内为关键实现位置。未勾选项列入 §7 风险表。

### 加密与传输
- [x] TLS1.3 only 于 QUIC / 推荐 HTTP；禁用明文 `grpc://` 于非 lab（`quic_listener.go` MinVersion；`grpc://` 非 TLS 时拒绝降级）
- [x] 会话 X25519 + AES-256-GCM；reg_secret 每 implant 隔离（`beacon_register.go` + `cipher.go`）
- [x] HMAC 覆盖 uuid/nonce/seq；重放窗拒绝旧 seq（`beacon_core` seq 单调校验）
- [x] loot 静态加密 + 密钥轮换；私钥路径权限收紧（`crypto.ReencryptLoot`；`data/update_signing.key` 0600）
- [x] 发布/OTA 一律 Ed25519；agent **必须** pin 公钥（拒绝裸 URL）（`agent_lifecycle.go:117` 空 pin 即拒）
- [x] uTLS 指纹池与 malleable UA 一致，避免 JA3 孤岛

### 认证与权限
- [x] JWT + 可选 TOTP；CSRF 双提交（`forgec2_csrf` header + cookie 双校验）
- [x] RBAC：`RequireRole` 于 settings/self_update/sign
- [x] mTLS `RequireAndVerifyClientCert` 可配置且失败则拒绝降级（`grpc_mtls_resync_hardening_test.go`）
- [x] 插件与 agent 最小权限；SeDebug 按需、默认不开

### 任务与 OPSEC
- [x] ~70 危险任务 approval gate 不可绕过（API + UI）（`tasktypes.go` 注册表派发；两处门禁均有测试）
- [x] `opsec.PreFlight` 在 dispatch 前执行
- [x] ROE CIDR deny 优先于 allow
- [x] 日志脱敏（token/secret/凭据不出明文）（`logfields` 统一脱敏）
- [x] kill-switch / uninstall 清凭据残留

### 实现卫生
- [x] 所有 listener：`LimitReader` + panic recover + 状态位诚实
- [x] 插件：无敏感 env、PATH 白名单、超时、输出 cap、进程树 kill（`executor.go`；cap 有测试）
- [x] `gofmt` / `go vet`（忽略 agent unsafe.Pointer 既知项）/ 测试 skip 仅已知 flaky（CI 门禁）
- [x] OpenAPI / i18n / permissions 前端门禁绿（`check:pre` 8 项并发）
- [x] 无密钥入库（gitleaks pre-commit + CI secrets job）

### 数据诚实性（2026-10-10 新增）
- [x] 读路径失败不伪装成功空数据：report/credentials/notifications/dashboard/collab/
      chat/mesh/timeline/chain/token/cloud/bloodhound/phishing/webhooks/backup-codes 等
      21 处查询失败改 500 fail-loud（`handleQueryError`；`query_failure*_test.go` 门禁）
- [x] 部分失败可见：报告 `partial` 标志 + HTML 横幅 + 前端分段 allSettled（`handlers_report.go`）
- [x] 凭据导出失败不产生空 CSV（`handlers_credentials.go`）
- [x] 插件包 digest 跨平台一致（CRLF 归一化 + volatile 目录排除 + NUL 字节二进制保护）
- [x] 重签密钥轮换后 52 个 bundled 插件全量验签通过（`TestBundledPluginsVerifyUnderStrictPolicy`）

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

## 6.5 插件签名与信任链（现状）

**2026-10-10 密钥轮换**。历史问题：9 个 bundled 插件的 manifest digest 记录的是
签名时刻包内 volatile 文件（未展开环境变量留下的 `%SystemDrive%` 树，内含会变动的
Windows 缓存 DB）的哈希，原签名种子已销毁，digest 不可复现，严格策略下这 9 个
插件被 fail-closed 拒绝（`TestBundledPluginsVerifyUnderStrictPolicy` 失败）。

当前状态（已恢复并对冲了整类风险）：

| 机制 | 位置 | 作用 |
|------|------|------|
| digest CRLF→LF 归一化 | `internal/plugin/trust.go:fileSHA256` | git `core.autocrlf` 检出差异不再改变 digest（Windows/Linux 一致，已用 git index LF 内容模拟验证） |
| NUL 字节二进制判定 | 同上 | wasm/exe 等二进制载荷按原字节哈希，不被行尾归一化破坏 |
| volatile 目录排除 | `digestExcludedDir` | `__pycache__` / `%SystemDrive%` 等运行时产物不进 digest |
| 启动清理 | `removeStrayPackageDirs` | `LoadFromDisk` 前移除 stray `%SystemDrive%` 树 |
| `(type, channel_id)` 唯一约束 | `2026-10-10-extc2-channel-unique` | upsert 原子化，历史重复行折叠（ExtC2 相关） |

重签流程（密钥轮换后）：`PLUGIN_SIGNING_KEY=<seed> go run ./cmd/sign-plugin -root plugins <dir>`。
新公钥在 `config.example.yaml` 与 `scripts/setup-dev.sh` 的 `plugins.trusted_keys`。
种子**不在仓库内**，由操作员离线保管（轮换时只展示一次）。

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
| **中** | 静态签名（AMSI/ETW patch 字节） | `decodeBypassPatch` 运行时解码；HWBP 备选路径；D2-4 插件/脚本沙箱边界收紧 |
| **中** | TLS 降级（阻断加密链路诱发明文回退） | D2-2b：全 TLS 配置的 implant 拒绝 WSS/DNS→HTTP 与明文 failover/rotation；https→http 跳转被拒 |
| **中** | 凭据 dump 落盘残留 | uninstall 清理；approval；loot 加密 |
| **中** | 备份静默失效（无人察觉直到恢复时） | `backup.retain`/`schedule` 可配置；`/api/v1/health` 暴露 age/last_error；`forgec2_backup_age_seconds` + 成功/失败计数（D-4） |
| **中** | C implant 与文档漂移 | matrix 标明子集；CI 构建门禁（`-Werror` + 任务字符串断言 + x25519 主机自检 + WASM 示例门禁）+ 一次性服务器全任务 E2E（`scripts/e2e-c-implant-local.ps1`，无需 UI） |
| **中** | 主 implant 只有构建门禁、无运行级验证 | 一次性服务器经真实 API 生成并**运行** ELF/EXE：CI `agent-e2e` job + `scripts/e2e-agent-local.ps1`（登录/CSRF → listener → generate → 下载魔数 → v3 注册 → 身份回填 → 4 任务矩阵 + `read` 缺参 400 契约） |
| **中** | 远程桌面屏幕内容越权 / 输入洪泛 | 三闸门（operator + 租户 + `agents.write`）；`/ws` 帧按角色过滤且未知用户 fail-closed；输入白名单 + 坐标/键名边界 + 40 次每秒限速；旧 RD WS origin 校验 + 8/64 连接上限 + 保活 + 审计；屏幕页/token 页租户作用域修复；契约见 `docs/REMOTE_DESKTOP.md` |
| **低** | 文档过期（matrix 曾停 v2.4.1） | P4-2 刷新 + 生成命令手册 |
| **低** | 根目录构建杂物 | gitignore + 清理脚本 |
| **低** | 调试端口误暴露（profiler） | `server.pprof_addr` 默认关闭且强制回环（配置加载 + 绑定双重拒绝），专属 mux 只挂 `/debug/pprof/*`，操作面 API/SPA 在该端口 404，关停随 `Shutdown` 释放；`enable_metrics` 保持需鉴权 |

---

## 8. 成熟度评分（1–10）

| 维度 | 分 | 理由 |
|------|----|------|
| 功能成熟度 | **8.5** | 217 任务、11 传输、52 插件、malleable v2、双语言 UI；扣分：C-implant 子集（recon/投递面已对齐 Go agent：drives/find/portscan/clipboard/usb_enum/usb_drop/session_recon/token_whoami）、部分 lateral/tun 非 Win stub |
| 稳定性/健壮性 | **9.4** | 队列/退避/kill-switch/限长/recover 齐全；QUIC stream cap + gRPC keepalive/MaxConnectionAge；Unix 进程组 kill；D2-4 插件 guard fail-closed + WaitDelay + timeout 硬上限；**数据诚实性专项**：21 处查询失败改 500 fail-loud、报告 partial 标志 + HTML 横幅、凭据导出失败不产空 CSV、ExtC2 Start 失败不留配置行、domain-front 探测移出锁并持久化 + 128 上限、AutoTag 单事务批量 + 上限、audit(user,created_at) 复合索引（`query_failure*_test.go` 门禁）；扣分：仍非 WASM 沙箱（CPU 与写入预算均已落地：Windows Job Object CPU/IO 记账、Linux /proc 进程组采样，默认 120s CPU + 1024 MiB 写入，超限杀树并报预算错误；WASM 输入改为写在模块初始内存之后，模块数据不再被宿主覆写）；**2026-10 监听器记账修复**（额外 `tcp://`/`tls://` 监听器的连接路径重复 `wg.Done()`：一次健康探测或一次上线即可把计数器打成负数并 panic 整个进程，现与主监听器统一由 `handleTCPConnection` 记账） |；**2026-10 远程桌面链路硬化**（RD hub 同会话重连替换旧连接、被替换连接的 Leave 不误删后继、每 agent 8 / 全局 64 连接上限、30s ping 保活修复 5 分钟静默断流、单飞预约消除 start/stop 双排队）
| 生态成熟度 | **9.4** | 命令>50、别名、manifest+goja、en/zh；D2-4 脚本 require 宿主机逃逸已封、事件回调有界；D4-1 插件包 digest+Ed25519 签名信任链 + `cmd/sign-plugin` + 并发配额；D4-2 **WASM 不可信插件层**（wazero，无 WASI、16 MiB 内存上限、超时，ABI v1 见 `docs/PLUGIN_TIERS.md`）；D4-3 52 个内置插件全量签名（2026-10 轮换密钥重签）+ digest CRLF 归一化跨平台一致 + volatile 目录排除 + `(type,channel_id)` 唯一约束 + 示例配置默认 `require_signed: true`；WASM ABI v1 修正：输入写在模块初始线性内存之后（wasm-ld 默认把数据段放在地址 1024，旧实现会在 `run()` 前覆写模块全局/堆）、结果改为单个 i64（`ptr | len<<32`——真实工具链实测 clang 即使开 `+multivalue` 也把双字段结构体返回降级为 sret 参数，多值返回对 C/Rust 不可达）、未导出 `memory` 的模块 fail-closed 而非宿主崩溃；WASM 示例模块：`examples/wasm/hello-report`（freestanding C、无 libc、含 build.sh 与提交产物）由 `internal/plugin/wasm_example_test.go` 在每次测试中走真实 executor 执行；**插件创作管线**（人可读 + agent 可读）：`cmd/plugin-init` 按 tier 生成「即合法」包（清单 + 各语言入口模板，注释内嵌运行时契约；WASM tier 附带 C 骨架与 build.sh），`cmd/plugin-check` 复用服务器同一条加载路径校验（`LoadManifest`/`Validate` → 包 digest+Ed25519 信任链 → WASM ABI 面用宿主同一 wazero 运行时检查 → `-run` 走真实 executor 并按插件类型校验输出契约），每条诊断带稳定 code 且 `-json` 输出机器可读报告，供「生成→校验→修复」闭环；规范与 JSON Schema 落 `docs/PLUGIN_AUTHORING.md` 与 `docs/schemas/plugin-manifest.schema.json`；解释器探测能识别 WindowsApps `python.exe` 假别名（退出码 9009）并报 `tier.interpreter_broken`；扣分：无（原「WASM 层暂无官方 SDK/示例模块」已修复） |
| 安全与合规加固 | **9.4** | TLS1.3/ECDH/GCM/Ed25519 pin/ROE/approval/审计；可选 ChaCha20-Poly1305；生产 compose；D2-1 `/api/v1` operator guard；D2-2 审计租户隔离；D2-3 gRPC mTLS fail-closed + resync oracle 关闭；**§4 checklist 25 项全部勾选**（含数据诚实性 5 项）+ 宏 run/stop/list 租户隔离（跨租户 404/空列表/停不掉测试）；**2026-10 per-listener TLS 姿态**（`client_ca_file`/`require_client_cert`/`min_tls_version` 挂在监听器记录上：可加客户端证书要求、自带客户端 CA、把下限抬到 TLS1.3，但永远无法放宽全局姿态或跌破 TLS1.2；不可兑现的组合（无 CA 却要证书、CA 不可读/无证书、下限写 1.1、非 TLS scheme 带 TLS 旋钮）在 API 边界 400 且启动时再拒一次，额外 HTTPS 监听器 TLS 配置失败不再进入 accept 循环；真实握手测试覆盖「带证书接受 / 无证书拒绝 / 异 CA 拒绝 / 1.3 下限拒 1.2 客户端 / 裸 `tls://` 同策略」；健康探针对强制客户端证书的监听器降级为 TCP 可达并显式标 `skipped`（端口不通仍真失败）；GUI 表单 + 详情页双语 + `docs/LISTENER_TLS.md`）；扣分：ChaCha20 非默认 |；**2026-10 远程桌面硬化**（屏幕内容统一三闸门：operator 身份 + 租户作用域 + `agents.write`；`/ws` 屏幕帧扇出按角色过滤，自定义角色走 `custom_roles`、查询失败 fail-closed，只读角色不再能靠一条 socket 看桌面；`remote_input` 白名单 move/click/key + 坐标 ±32767 + 键名 1–32 + 40 次/秒/(操作员,agent) 限速 429；屏幕流 `width` 校验规范化（原为拼接进 agent CSV 命令的参数注入）与非法 `mime` 400；旧 `/ws/remote-desktop` 补 origin 校验、身份取会话用户、8/64 连接上限、替换旧 socket、ping 保活与开/关审计；屏幕页与 token 页跨租户 IDOR 修复；契约与上限表见 `docs/REMOTE_DESKTOP.md`）；**2026-10 mTLS 客户端证书生成闭环**（生成表单新增 `mtls_cert`/`mtls_key`/`mtls_ca` 三个 PEM 字段：必须成对、`tls.X509KeyPair` 可解析、CA 捆绑含证书、单项 ≤64KiB，不满足在 API 边界 400；材料以 base64 进 AES-GCM config blob（config-over-wire 引导构建同样携带，否则初始端点连不上），私钥不进 `go build` argv、不落库；agent `initMTLS` 补 base64 解码——此前把 base64 当 PEM 直接喂 `X509KeyPair`，生成的 mTLS agent 静默不带客户端证书，现以真实 uTLS 握手测试固定「带证书通过 / 无证书拒绝 / 垃圾材料 fail-closed」；GUI 双语字段 + 内联成对提示 + `docs/LISTENER_TLS.md` 更新；已知限制：C implant 仍不带客户端凭据）；**2026-10 stager 传输对等**（`POST /generate/stager|stager_linux` 以与直建相同的边界规则接受 `mtls_cert`/`mtls_key`/`mtls_ca`（成对、`X509KeyPair` 可解析、CA 含证书、单项 ≤64KiB，不满足 400）；stage-2 信标经 config blob 走同一条材料通路，stage-1 stager 由生成的 `mtls_inject.go` 编译内嵌——PEM 换行无法经 `-X` 传递且私钥不得出现在构建 argv，材料不可解析时直接退出而非无凭据回退；表单接收却从未接线的 `skip_tls_verify`/`proxy`/`user_agent` 修复为纯函数 `stagerFetch` 并配映射测试（此前 stager 一律校验证书、忽略代理与 UA）；惰性 token 流程 `POST /stager/register` 对强制客户端证书的监听器在注册即 400 并说明原因（该流程不落任何材料，URL 注定不可用）；CI 烟测 `TestSmokeTokenStagerMTLSFetch` 真实构建 Linux stager 对「要求并校验客户端证书」的 TLS 服务器完成拉取/解密/执行，拒绝场景断言 stage-2 未执行）
| 可维护性/更新性 | **9.3** | 单一 TaskSpec 真源、CI 门禁、gofmt 清债、OTA 签名链闭环；matrix 任务清单生成段 + `--check`；版本行由 `VERSION` 单源 stamp；D3 CI 含 `-race` job、40% 覆盖率下限（实测 CI 范围 46%+）、定时 fuzz、Docker/Compose 校验、**真实服务端 live UI e2e**、**生成式 Go agent 运行级 E2E**（CI `agent-e2e`：一次性服务器真实生成并运行主 implant）；OTA 版本比较对无法解析的版本号 fail-closed（未 stamp 的构建不会被远端旧版本覆盖）；发布含 SBOM + provenance；扣分：无（原「可变 tag」已由 D/E 批次修复：39 处 Actions 钉 SHA、Dockerfile/compose 基础镜像钉多架构 digest、Dependabot 增加 docker/docker-compose 生态；升级 PR 评审仍是人工步骤；**2026-10 lint 债务治理**（机械类 linter（unconvert/ineffassign/govet/misspell）全树清零；windows/linux 双分析集零容忍 + 债务上限 `scripts/lint-backlog.txt`（968/634；gosec G702 taint 结果运行间不可复现，已排除并由 CI 日志留全量审计清单）由 `scripts/check-lint-debt.sh` 校验、只降不升；同批修复三处真实缺陷：证书上传/重生成覆盖显式 `key_file`、workflow 自跳预算每轮重置令 P2 守卫失效、Linux 凭据转储丢弃已读到的 SSH 私钥内容）；**2026-10 批次K 死代码裁决**（unused 整树清零：windows 116 / linux 64 全部删除或搬迁，债务上限降至 846/569 只降不升；裁决原则「写而不读的一律删」：只写不读的旗标与孤儿链全数退役——`enableVEHUnhook`、`chameleonEnabled`、anti-debug 计分、egress 报告缓存、profile_rotate 的 `encoding` 槽、config_push 的 sleep/jitter 槽；6 个 windows-only 逃生旗标迁入 `edr_strategies_windows.go`；`indirectNtDelayExecution` 收编进 sleepmask、`ekko_sleep_windows.go` 与全力死链的 `envdetect_windows.go` 整删；`edr_windows.go` 枚举改 Win32 ABI 步长手工遍历消除镜像结构体；新增唯一行为修复：删除 agent 时接线 `cleanupLPortFwdForAgent` 拆隧道防止悬空 socket） |
| 额外加分 | **8.0** | HTTP/3、别名史、多语言插件、跨平台构建已有；扣分：模块化代理/WASM、Win TUN 未打包 |
| **综合** | **9.5** | P3 工程债已清 + P4 全落地 + Phase C + **D-2 服务端 P0**（operator guard/审计租户/gRPC mTLS fail-closed/resync oracle/插件脚本 fail-closed/存储密钥与 compose 契约/PG 备份边界）+ **D-2b agent fail-closed**（禁明文降级、DNS 校验、证书 pin、响应上限）+ **D-3 供应链**（race/覆盖率/fuzz/docker/live-e2e 门禁、SBOM + provenance）+ **D-4 插件生态**（包签名信任链、共享 lib 覆盖、并发配额、WASM 不可信层、52 插件已签名且默认强制验签）+ **D-5 备份可运维性**（可配置保留/调度 + 新鲜度指标与 health 暴露）+ **2026-10 数据诚实性与可信度专项**（§4 checklist 清零；21 处 fail-loud + partial 标志 + 5 个租户隔离/孤儿行/持久化/批量/索引修复，每项配回归测试）+ **2026-10 传输与更新链专项**（WebSocket Hub 的连接 claim/隔离契约回归测试；批量重试不再静默丢弃超限结果帧；OTA 版本比较对不可解析版本 fail-closed）+ **2026-10 供应链钉扎**（39 处 Actions 钉 SHA + 4 个基础镜像钉多架构 digest + Dependabot 覆盖 docker/docker-compose；golangci-lint 改为增量棘轮收债）+ **2026-10 插件 CPU 预算**（Windows Job Object / Linux /proc 进程组记账，默认 120s，超限杀树并报 CPU 预算错误；采样间隔显式传参，消除 watchdog 与测试的竞态）+ **2026-10 插件写入预算**（同一 watchdog 记账写入字节，默认 1024 MiB，超限杀树；Windows Job Object I/O 计数 / Linux `/proc/<pid>/io` wchar）+ **2026-10 WASM ABI 加固**（输入写在模块初始内存之后，不再覆写模块数据；缺内存模块 fail-closed）+ **2026-10 插件创作管线**（`cmd/plugin-init` 脚手架 + `cmd/plugin-check` 校验器复用真实信任链与 executor，`-run` 真跑并校验输出契约，诊断带稳定 code 与 `-json` 报告；规范 + manifest JSON Schema 落文档） + **2026-10 C-implant 任务扩展与门禁**（clipboard_get/set、drives、find、portscan、get_sleep_mode 六个任务输出对齐 Go agent；CI 交叉构建 `-Werror` + 字符串断言 + `x25519_selftest` 主机自检 + WASM 示例 `plugin-check -run`；`cmd/mint-reg-secret` + 一次性服务器本地 E2E 全绿，无需操作台）+ **2026-10 C-implant 侦察/投递对等**（usb_enum/usb_drop/session_recon/token_whoami 输出对齐 Go agent；usb_drop 两条拒绝语义入 E2E 矩阵并扩到 12 用例；一次性服务器配置显式关两员审批只为让高危类型抵达 implant）+ **2026-10 Go agent 运行级 E2E 门禁**（一次性服务器经真实 API 生成并运行主 implant：v3 注册、身份回填、4 任务矩阵与 `read` 缺参 400 契约全部断言；CI 新增 `agent-e2e` job，本地 Windows EXE / CI Linux ELF 双变体，红跑保现场并输出服务器日志尾部）+ **2026-10 per-listener TLS 姿态**（监听器级 `client_ca_file`/`require_client_cert`/`min_tls_version` 只可收紧全局姿态，真实握手测试 + 健康探针 mTLS 降级显式标 skipped + GUI/双语文档；同批修复额外 `tcp://`/`tls://` 监听器 WaitGroup 双 Done 崩溃）+ **2026-10 性能与容量专项**（beacon 热路径三态 benchmark 入仓（空闲 / 取任务 / 带结果）并在 `docs/CAPACITY.md` 记录单核实测 ≈1840/1380/1010 次每秒、由 check-in 速率推导机队上限与明确排除项；监听器详情读路径 `implants(listener_id,last_seen)` 复合索引消除临时 B 树排序与回表，SQL 计划回归探针改为「一探针一迁移」以免已应用迁移被追溯修改；`server.pprof_addr` 只读回环调试口（默认关闭，配置加载与绑定双重拒绝非回环地址，专属 mux 仅挂 `/debug/pprof/*`）；`PrepareStmt` 语句缓存 A/B 量化后否决并留档） | + **2026-10 远程桌面硬化专项**（把远程桌面/屏幕链路从「能用」收到「可审计的受控面」：三闸门统一（operator + 租户 + `agents.write`），`/ws` 屏幕帧扇出按角色过滤（自定义角色解析 custom_roles，未知用户 fail-closed），原先只读角色靠同租户 socket 即可看桌面；`remote_input` 在 API 边界白名单化（move/click/key、坐标 ±32767、键名 1–32）并限速 40 次/秒/(操作员,agent)（每个事件本就要写任务行 + 审计行，未限速即写入放大原语）；屏幕流 `width` 校验并规范化为 16–16384（此前字符串直接拼进 agent 的 CSV 流命令，属参数注入）、非法 `mime` 由静默忽略改为 400；旧 `/ws/remote-desktop` 中继补 origin 校验、操作员身份改取自会话（原先查询参数可伪造）、每 agent 8 / 全局 64 连接上限（升级前拒绝返回真实 HTTP 状态）、同操作员重连替换并关闭旧 socket、30s ping 保活（此前服务端从不 ping，5 分钟读截止把活跃会话掐死）与开/关/截图审计；屏幕监控 start/stop 改单飞预约（并发第二个请求 409）且不再跨任务创建/插件钩子/审计写持锁（每帧 ingest 同一把锁）；屏幕页与 token 页补租户作用域（跨租户 IDOR）；前端修复间隔单位（毫秒→秒，此前每次新开流必 400）并在 WS 帧活跃时跳过 HTTP 轮询；新增 `docs/REMOTE_DESKTOP.md` 固化两端传输、闸门、上限表、审计事件与已知限制；前端覆盖率下限 14.5%→14.8%（实测 14.91%，8473/56846）） + **2026-10 mTLS 客户端证书生成闭环**（表单字段 → 成对/可解析/大小校验（API 边界 400）→ AES-GCM blob（base64；bootstrap 构建同样携带）→ agent 解码修复（原 base64 当 PEM，静默无客户端证书）→ 真实 uTLS 握手测试；私钥不进 `go build` argv、不入库；GUI 双语 + `docs/LISTENER_TLS.md` 更新；前端覆盖率随批实测 14.98%（8522/56905，下限保持 14.8%）；已知限制：C implant 不带客户端凭据）+ **2026-10 stager 传输对等**（`/generate/stager|stager_linux` 接受 mTLS 三字段并在 API 边界 400；stage-2 信标经 config blob、stage-1 stager 经生成的 `mtls_inject.go`（PEM 换行与私钥都不进构建 argv）；`skip_tls_verify`/`proxy`/`user_agent` 从「表单接收但丢弃」修复为纯映射 `stagerFetch` 并配测试；惰性 token 注册对强制客户端证书监听器 400；CI 烟测真实构建并运行 Linux stager 对 mTLS 服务器完成拉取/解密/执行，拒绝场景断言 stage-2 未执行）；**2026-10 lint 债务治理**（机械类 linter 清零 + windows/linux 双分析集零容忍 + 债务上限只降不升；同批修复证书 `key_file` 覆盖、workflow 自跳预算重置、Linux 凭据转储丢弃 SSH 私钥内容）+ **2026-10 批次K 死代码裁决**（unused 在 windows/linux 双分析集全量清零（原 116/64，「写而不读」与孤儿链一并退役：全图 sleepmask 机理 `chameleonEnabled`/`enableVEHUnhook` 开关、Ekko Go 路径（C 端实现保留）、strategy 旗标迁 `edr_strategies_windows.go`；唯一行为修复：服务器删除 agent 时接线 `cleanupLPortFwdForAgent`；lint 债务上限降至 846/569）

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
