# ForgeC2 功能补齐计划

> 基于 [C2 Beacon Attack](https://mp.weixin.qq.com/s/3wVu3nOv35VjRSqI5b400w) 文章对比分析


## Phase 1: BOF 执行（Beacon Object File）

**目标**: 支持在 Agent 内存中执行 COFF 格式的 BOF 文件

### 服务端

| 模块 | 文件 | 改动 |
|------|------|------|
| BOF 管理页面 | `templates/bof.html` | 新建：上传、列表、执行 BOF |
| BOF 路由 | `server.go` | `auth.GET("/bof", handleBOFPage)` + `auth.POST("/api/bof/upload", handleBOFUpload)` + `auth.POST("/api/bof/run", handleBOFRun)` |
| BOF 处理器 | `handlers_bof.go` | 新建：上传解析 COFF、下发执行任务 |

### Agent 端

| 模块 | 文件 | 改动 |
|------|------|------|
| COFF 加载器 | `agent/coff/loader.go` | 新建：解析 COFF 头、处理重定位、符号解析 |
| 函数解析 | `agent/coff/resolver.go` | 新建：动态解析 Win32 API 地址（通过 GetProcAddress） |
| 命令注册 | `agent/tasks.go` | 注册 `bof_run` 命令类型 |

### 数据结构

```go
type BOFFile struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    // 文件名
    Data      []byte    // COFF 二进制
    Args      string    // 参数定义描述
    CreatedAt time.Time
}
```

### 流程图

```
Web UI → POST /api/bof/upload → 存入 DB → Agent 拉取任务
→ Agent 下载 COFF → 解析头 → 加载到内存 → 重定位 → 执行
→ 回传结果 → Web UI 显示
```

### 依赖

- Go 端：无外部依赖，纯内存操作
- COFF 加载参考：TrustedSec/COFFLoader, CobaltStrike BOF 规范


## Phase 2: AMSI/ETW Patch + Sleep Mask

### 2.1 AMSI Patch

**目标**: 绕过 AMSI 对 PowerShell/.NET 脚本的扫描

**原理**: 在 Agent 运行时修改 `amsi.dll!AmsiScanBuffer` 函数的前几个字节，使其始终返回 `AMSI_RESULT_CLEAN`

**实现**:

| 步骤 | 说明 |
|------|------|
| 1 | `LoadLibrary("amsi.dll")` |
| 2 | `GetProcAddress("AmsiScanBuffer")` |
| 3 | `VirtualProtect(PAGE_EXECUTE_READWRITE)` |
| 4 | 写入 `mov eax, 0x80070057; ret`（返回错误码，不扫描） |
| 5 | `VirtualProtect` 恢复原属性 |

**Agent 端文件**: `agent/evasion/amsi.go`

### 2.2 ETW Patch

**目标**: 阻断 Windows 事件跟踪上报，隐藏运行时行为

**原理**: Patch `ntdll!EtwEventWrite` / `ntdll!NtTraceEvent` 直接返回

**实现**: 同 AMSI 模式，patch 函数入口为 `ret` 指令

**Agent 端文件**: `agent/evasion/etw.go`

### 2.3 Sleep Mask

**目标**: 心跳休眠期间加密 Agent 内存，防 EDR 内存扫描

**原理**:

```
Beacon 执行任务 → 进入休眠
  → 加密堆+栈+代码段 → Sleep(interval)
  → 解密恢复 → 下次心跳
```

| 组件 | 说明 |
|------|------|
| 加密算法 | RC4 或 XorShift（轻量且快） |
| 加密范围 | 关键数据段 + goroutine 栈 |
| 解密时机 | 休眠前 → 加密 → Sleep → 解密 → 醒来 |

**Agent 端文件**: `agent/evasion/sleepmask.go`

**注意事项**:
- Go runtime 的 goroutine 栈由 runtime 管理，直接加密可能导致崩溃
- 建议方案：加密自定义 heap 分配的数据区域 + 关键字符串表
- 更激进方案：在 `syscall.Sleep` 调用前 hook，加密整个 data 段

**依赖**: `golang.org/x/sys/windows` 或直接 `syscall`


## Phase 3: LSASS 凭据提取

**目标**: 远程转储 LSASS 进程内存，提取凭据

### Agent 端

| 步骤 | 说明 |
|------|------|
| 1 | 通过 PID/进程名定位 LSASS |
| 2 | `MiniDumpWriteDump` 直接调用（通过 syscall） |
| 3 | 加密内存块（AES/自研算法） |
| 4 | 流式分块上传到 C2（HTTPS POST chunk） |

**文件**: `agent/credentials/lsass.go`

### 服务端

| 步骤 | 说明 |
|------|------|
| 1 | 接收分块上传的加密 dump |
| 2 | 解密后存储为 `.dmp` 文件 |
| 3 | Web UI 提供下载或在线解析 |

### 注意事项

- `MiniDumpWriteDump` 是敏感 API，建议动态解析
- 需处理 SeDebugPrivilege
- Windows  Defender 会拦截，需配合 AMSI/ETW Patch


## Phase 4: .NET 程序集加载 + 自研加密

### 4.1 .NET 程序集加载

**目标**: 在 Agent 进程内纯内存加载执行 .NET 程序集（如 Seatbelt、Rubeus）

**架构**:

```
Agent (Go) → 创建 .NET Runtime → 反射加载 Assembly
  → 调用入口点 → 捕获 Console.Out → 回传结果
```

| 组件 | 文件 | 说明 |
|------|------|------|
| CLR Host | `agent/dotnet/host.go` | 通过 `CLRCreateInstance` 加载 .NET Runtime |
| Assembly Loader | `agent/dotnet/loader.go` | 反射调用 `Assembly.Load(byte[])` |
| Output Capture | `agent/dotnet/capture.go` | 重定向 `Console.Out` 到内存流 |

**注意事项**:
- 需在非 .NET 进程中宿主 CLR，特征明显但不可避免
- 在调用前先执行 AMSI Patch

### 4.2 自研加密协议

**目标**: 替换当前通信 XOR 加密为更强算法

**设计**:

```
密钥: 32 字节，Profile 配置，编译期确定
算法: ChaCha20 或 XorShift128+（性能优先）
流程:
  Agent 发包: nonce(8B) + ciphertext
  Server 收包: 提取 nonce → 解密 → 处理
  双向独立密钥流
```

**文件**: `internal/crypto/cipher.go` + `agent/crypto/cipher.go`

**与现有系统兼容**: 可配置切换，新旧 agent 并存


## Phase 5: 可配置 Profile + 反沙箱

### 5.1 C2 Profile

**目标**: 所有通信特征可配置，编译期确定

**配置项**:

| 字段 | 说明 |
|------|------|
| `headers` | HTTP 请求头自定义 |
| `uris` | API 路由自定义 |
| `user_agent` | UA 字符串 |
| `jitter` | 心跳抖动百分比 |
| `sleep_time` | 默认休眠时间 |
| `encrypt_key` | 加密密钥 |

**文件**: `config.yaml` 扩展 `agent` 段 + `agent/profile.go`

### 5.2 反沙箱

**目标**: 检测运行环境是否为沙箱/虚拟机，决定是否执行

**检测项**:

| 检测 | 方法 |
|------|------|
| 硬件 | CPU 核心数 < 2、内存 < 2GB、磁盘 < 60GB |
| 进程 | vmtoolsd.exe、procmon.exe、wireshark.exe 等 |
| 时间加速 | 对比 `rdtsc` 与 `GetTickCount` 时间差 |
| 用户名 | `WDAGUtilityAccount`、`MAD`、`Sandbox` 等 |
| 域名 | 是否加入域 |

**文件**: `agent/evasion/sandbox.go`

**行为**: 检测到沙箱后进入"良性"模式——正常心跳但不执行敏感命令


## Phase 6: Beacon 侧端口扫描 + 多协议通信

### 6.1 端口扫描

**目标**: Agent 端执行 TCP 端口扫描，结果回传

```
扫描任务:
  args: { hosts: ["192.168.1.1-254"], ports: "80,443,8080-8090", timeout: 2000 }
  
Agent 执行:
  for each host:
    for each port:
      goroutine: TCP dial(timeout) → 成功标记开放
  → 汇总结果 → 加密回传
```

**文件**: `agent/scanner/portscan.go`

### 6.2 DNS 通信（已有基础）

**目标**: 完善 DNS Beacon 作为备用信道

| 改动 | 说明 |
|------|------|
| Agent DNS resolver | `agent/transport/dns.go` | 通过 DNS TXT 查询拉取任务 |
| Server DNS handler | 已有 DNS 监听器配置 | 完善 TXT 记录响应 |

### 6.3 SMB 通信（新增）

**目标**: 内网横向时通过 SMB named pipe 通信

```
Agent A (边界) → HTTP(S) → C2
Agent B (内网) → SMB Pipe → Agent A → C2
```

**文件**: `agent/transport/smb.go`


## Phase 7: 间接系统调用 + 静态混淆

### 7.1 间接系统调用

**目标**: 绕过 EDR 用户态 hook，直接调用 ntdll 系统服务

**实现**:

```
1. 定位 ntdll.dll 的 .text 段原始代码
2. 提取 syscall 指令序列 (mov eax, SSN; syscall; ret)
3. 在 Agent 内存中构建 syscall stub
4. 所有敏感操作通过 syscall stub 调用
```

**文件**: `agent/evasion/syscall.go`

### 7.2 静态混淆

**目标**: 增加逆向难度，规避签名检测

| 技术 | 说明 |
|------|------|
| 字符串加密 | 所有敏感字符串 XOR 加密，运行时解密 |
| GOPATH 混淆 | 编译时使用随机路径 |
| 加壳 | 使用 UPX/MPRESS 压缩 |
| 数字签名 | 使用自签名证书伪装 |

**实现方式**: 通过 `go generate` + 构建脚本自动化


## 实施优先级

| Phase | 功能 | 预估工时 | 依赖 |
|-------|------|----------|------|
| 1 | BOF 执行 | 3-5 天 | 无 |
| 2 | AMSI/ETW Patch + Sleep Mask | 2-3 天 | 无 |
| 3 | LSASS 凭据提取 | 2-3 天 | Phase 2 |
| 4 | .NET 程序集 + 加密 | 3-5 天 | Phase 2 |
| 5 | Profile + 反沙箱 | 2-3 天 | 无 |
| 6 | 端口扫描 + 多协议 | 3-5 天 | 无 |
| 7 | 间接系统调用 + 混淆 | 2-3 天 | 无 |

**总计**: 约 17-27 天
