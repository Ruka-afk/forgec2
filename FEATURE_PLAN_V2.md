# ForgeC2 第二阶段功能补齐计划

> 对比 Cobalt Strike 5.x、Sliver 1.6、Mythic 3.x 等主流 C2 框架的差距分析
> 基于当前代码库（已实现 BOF、Sleep Mask、StreamCipher、反沙箱、SMB 传输、静态混淆基础）

---

## Phase 8: 高级进程注入（2 周）

### 8.1 多种注入原语

| 注入方法 | 文件 | 说明 |
|----------|------|------|
| CreateRemoteThread | `agent/injections/createremotethread.go` | ✅ 已有 `inject` 命令，需重构为标准单文件 |
| NtCreateThreadEx | `agent/injections/ntcreatethreadex.go` | 新建：通过 ntdll 间接调用 |
| QueueUserAPC | `agent/injections/apc.go` | 新建：APC 注入，适合低延时信标 |
| ThreadlessInject | `agent/injections/threadless.go` | 新建：Patch 线程入口点，不创建新线程 |
| EarlyBird APC | `agent/injections/earlybird.go` | 新建：CreateProcess 挂起 → APC 队列 → ResumeThread |

**命令注册**：`inject_createremotethread <pid> <shellcode_path>`、`inject_apc <pid> <shellcode_path>`、`inject_ntcreatethread <pid> <shellcode_path>`

### 8.2 直接系统调用（Syscall Stubs）

| 组件 | 文件 | 说明 |
|------|------|------|
| Hell's Gate | `agent/evasion/hellsgate.go` | 动态解析 SSN（系统服务号） |
| Halo's Gate | `agent/evasion/halosgate.go` | Hell's Gate 变体，跳过可疑 SSN |
| Syscall Stub | `agent/evasion/syscall_stub_amd64.go` | `mov eax, SSN; syscall; ret` 汇编 stub |
| 间接 Syscall | `agent/evasion/indirect_syscall.go` | 通过 ret 指令跳转到 ntdll 内 syscall 指令 |

**依赖**：`golang.org/x/sys/windows` + 内联汇编（`//go:noescape`）

### 8.3 注入到命令框架

- `agent.go` `executeTask` 增加 `inject` 派发改为调用 Phase 8.1 各方法
- 新增命令 `inject_list_methods` 列出可用注入方法
- **配置**：AgentConfig 增加 `InjectionMethod` 字段（编译期选择默认注入方式）

---

## Phase 9: 高级加载器（2 周）

### 9.1 Shellcode 加载执行

| 组件 | 文件 | 说明 |
|------|------|------|
| Shellcode Loader | `agent/payload/shellcode.go` | 新建：`VirtualAlloc` + `RtlCopyMemory` + `CreateThread` |
| 加载贝式 | `agent/payload/shellcode_binary.go` | 新建：支持 bin/C/Python/Ruby/VBA 等多格式输入 |
| XOR 解码器 | `agent/payload/xor_decode.go` | 新建：运行时 XOR 解码后执行 |

**命令**：`shinject <pid> <base64_shellcode>`、`shspawn <shellcode>`（直接 spawn 新进程）

### 9.2 Donut Shellcode 生成（服务端）

| 组件 | 文件 | 说明 |
|------|------|------|
| Donut 整合 | `internal/payload/donut.go` | 新建：调用 Donut CLI 或内嵌 Donut 库（Go 重新实现） |
| .NET → Shellcode | `internal/payload/convert.go` | 新建：托管 Donut 输出 |

### 9.3 sRDI（Shellcode Reflective DLL Injection）

| 组件 | 文件 | 说明 |
|------|------|------|
| 反射加载器 | `agent/payload/rdi.go` | 新建：解析 DLL → 内存加载 → 重定位 |
| Shellcode 转换 | `internal/payload/srdi.go` | 新建：将 DLL 转换为 position-independent shellcode |

**命令**：`srdi_load <dll_path> <export_name>`、`srdi_spawn <base64_srdi>`

### 9.4 生成页面增强

- `templates/generate.html` 增加 Shellcode / Donut / sRDI 选项卡
- 新增 `/generate/shellcode`、`/generate/donut`、`/generate/srdi` 路由
- 新增 `/api/payload/convert` 转换接口（任意 PE → shellcode）

---

## Phase 10: 横向移动增强（2 周）

### 10.1 横向移动套件

| 方法 | 文件 | 说明 |
|------|------|------|
| WMI Exec | `agent/lateral/wmi.go` | 新建：通过 `win32_process` 创建远程进程 |
| WinRM | `agent/lateral/winrm.go` | 新建：通过 WinRM Web 服务执行命令 |
| PsExec | `agent/lateral/psexec.go` | 新建：SVC 管理 + 文件复制 + 服务启动 |
| DCOM | `agent/lateral/dcom.go` | 新建：通过 DCOM（MMC20.Application）远程执行 |
| SCF 凭证 | `agent/lateral/scf.go` | 新建：SCF 文件触发 Net-NTLM hash |

### 10.2 横向命令重构

- 重构 `lateral` 命令为 `lateral_wmi / lateral_winrm / lateral_psexec / lateral_dcom`
- 统一参数格式：`lateral_wmi <target> <user> <password|hash> <command>`
- 支持 `lateral_list` 列出可用横向方法

### 10.3 SMB 主机发现

| 组件 | 文件 | 说明 |
|------|------|------|
| SMB 扫描 | `agent/scanner/smb_scan.go` | 新建：探测内网 SMB 主机 |
| 主机枚举 | `agent/scanner/host_enum.go` | 新建：LDAP + SMB 联合发现 |

**命令**：`net_scan_smb`、`net_enum_hosts`

---

## Phase 11: 凭证攻击深度扩展（2 周）

### 11.1 DPAPI

| 组件 | 文件 | 说明 |
|------|------|------|
| DPAPI 解包 | `agent/credentials/dpapi.go` | 新建：解密 DPAPI blob |
| MasterKey 获取 | `agent/credentials/dpapi_mk.go` | 新建：获取用户 MasterKey |
| Chrome/Edge 解密 | `agent/credentials/dpapi_browser.go` | 新建：解密浏览器保存的密码 |

### 11.2 LSA 保护绕过

| 组件 | 文件 | 说明 |
|------|------|------|
| LSA 绕过 | `agent/credentials/lsa_bypass.go` | 新建：RunAsPPL 绕过（注册表+驱动） |

### 11.3 AD CS 攻击

| 组件 | 文件 | 说明 |
|------|------|------|
| ESC1 检测 | `agent/credentials/adcs_esc1.go` | 新建：枚举 AD CS 模板漏洞 |
| 证书请求 | `agent/credentials/adcs_request.go` | 新建：通过 ICertRequest 请求证书 |
| Shadow Credentials | `agent/credentials/shadow_creds.go` | 新建：WriteOwner + Shadow Credentials |

**命令**：`dpapi_masterkey`、`dpapi_blob <file>`、`adcs_find`、`adcs_request <template>`、`shadow_creds <target>`

---

## Phase 12: LDAP 域信息采集（1 周）

### 12.1 LDAP 枚举引擎

| 组件 | 文件 | 说明 |
|------|------|------|
| LDAP API 封装 | `agent/recon/ldap.go` | 新建：通过 `ADSI` COM 接口进行 LDAP 查询 |
| 用户枚举 | `agent/recon/ldap_users.go` | 新建：列出所有域用户 |
| 组枚举 | `agent/recon/ldap_groups.go` | 新建：列出所有域组及成员 |
| 计算机枚举 | `agent/recon/ldap_computers.go` | 新建：列出所有域计算机 |
| SPN 枚举 | `agent/recon/ldap_spn.go` | 新建：列出所有服务主体名称（Kerberoast 预备） |
| ACL 枚举 | `agent/recon/ldap_acl.go` | 新建：列举高危 ACE（GenericAll, WriteOwner 等） |

**命令**：`ldap_users`、`ldap_groups`、`ldap_computers`、`ldap_spn`、`ldap_acl`、`ldap_query <ldap_filter>`

**注意**：Windows 通过 `activeds.dll` + `ADOpenDSObject` COM；Linux 无法枚举 AD（stub 返回错误）

---

## Phase 13: C2 信道扩展（2 周）

### 13.1 ICMP C2

| 组件 | 文件 | 说明 |
|------|------|------|
| ICMP Agent | `agent/transport/icmp.go` | 新建：通过 ICMP echo request/response 携带数据 |
| ICMP Server | `internal/server/icmp_listener.go` | 新建：原始套接字监听 ICMP |

**注意**：Windows 需要管理员权限创建原始套接字，Linux 需要 `CAP_NET_RAW`

### 13.2 外部 C2（External C2）

| 组件 | 文件 | 说明 |
|------|------|------|
| External C2 协议 | `internal/server/extc2.go` | 新建：实现 CS 兼容的外部 C2 协议 |
| SMB 中继 | `internal/server/extc2_smb.go` | 新建：SMB 命名管道中继 |

### 13.3 HTTP/2 + HTTP/3

| 组件 | 文件 | 说明 |
|------|------|------|
| HTTP/2 支持 | `agent/transport/h2.go` | 新建：通过 `golang.org/x/net/http2` 实现 |
| HTTP/3 支持 | `agent/transport/h3.go` | 新建：通过 `github.com/quic-go/quic-go` 实现 |

### 13.4 Domain Fronting

| 组件 | 文件 | 说明 |
|------|------|------|
| Domain Front Agent | `agent/transport/domain_front.go` | 新建：修改 Host 头实现前端绕过 |

### 13.5 Dead Drop（云存储）

| 组件 | 文件 | 说明 |
|------|------|------|
| Dropbox C2 | `agent/transport/dropbox.go` | 新建：通过 Dropbox API 传递信标 |
| GDrive C2 | `agent/transport/gdrive.go` | 新建：通过 Google Drive API |

### 13.6 P2P Mesh

| 组件 | 文件 | 说明 |
|------|------|------|
| Mesh 路由 | `agent/p2p/mesh.go` | 新建：全互联模式，任意节点间路由 |

**命令/配置**：`AgentConfig.Protocols` 增加 `icmp`、`h2`、`h3`、`domainfront`、`dropbox` 等选项

---

## Phase 14: 完整 Malleable Profile（2 周）

### 14.1 Profile 系统重写

| 组件 | 文件 | 说明 |
|------|------|------|
| Profile 解析器 | `internal/malleable/profile.go` | 新建：解析 CS 风格 Malleable C2 Profile |
| HTTP GET 段 | `internal/malleable/http_get.go` | 新建：GET 请求的 URI、参数、Header、Data Transform |
| HTTP POST 段 | `internal/malleable/http_post.go` | 新建：POST 请求配置 |
| Data Transform | `internal/malleable/transform.go` | 新建：Base64 / NetBIOS / Mask / XOR 变换链 |

### 14.2 数据变换链

| 变换 | 说明 |
|------|------|
| `base64` | Base64 编码/解码 |
| `netbios` | NetBIOS 名称编码（A-encoding） |
| `mask` | XOR mask + 偏移 |
| `print` | 可打印字符编码 |
| `append "..."` | 追加固定字符串 |
| `prepend "..."` | 前置固定字符串 |

### 14.3 Jitter 增强

| 改进 | 说明 |
|------|------|
| Sleep Jitter | ✅ 已有 |
| Content Length Jitter | 新增：随机填充 HTTP body |
| URI Jitter | 新增：随机选择 URI |
| Parameter Jitter | 新增：随机参数名+值 |

### 14.4 Profile 预置集

| 名称 | 说明 |
|------|------|
| `default` | ✅ 已有 |
| `microsoft` | Microsoft 365 遥测模拟 |
| `google_analytics` | Google Analytics 模拟 |
| `cloudflare_cdn` | Cloudflare CDN 模拟 |
| `akamai` | Akamai 请求模拟 |

---

## Phase 15: 增强反沙箱/规避（2 周）

### 15.1 多方法 ETW Bypass

| 方法 | 文件 | 说明 |
|------|------|------|
| Patch EtwEventWrite | `agent/evasion/etw_patch.go` | ✅ 已有 |
| Patch NtTraceEvent | `agent/evasion/etw_ntrace.go` | 新建：更底层 hook |
| TLD patching | `agent/evasion/etw_tld.go` | 新建：EtwWrite 替代方案 |

### 15.2 多方法 AMSI Bypass

| 方法 | 文件 | 说明 |
|------|------|------|
| AmsiScanBuffer Patch | `agent/evasion/amsi_patch.go` | ✅ 已有 |
| AmsiOpenSession Patch | `agent/evasion/amsi_session.go` | 新建：Patch AmsiOpenSession |
| HWBP Bypass | `agent/evasion/amsi_hwbp.go` | 新建：硬件断点绕过 |
| Registry Bypass | `agent/evasion/amsi_reg.go` | 新建：注册表禁用 AMSI |

### 15.3 调用栈欺骗

| 组件 | 文件 | 说明 |
|------|------|------|
| Stack Spoofer | `agent/evasion/stack_spoof.go` | 新建：伪造调用栈，隐藏 syscall 来源 |

### 15.4 VEH/SEH Unhooking

| 组件 | 文件 | 说明 |
|------|------|------|
| VEH Unhook | `agent/evasion/veh_unhook.go` | 新建：通过 VEH 异常处理恢复 ntdll 原始代码 |

### 15.5 Block DLLs

| 组件 | 文件 | 说明 |
|------|------|------|
| Block DLL Policy | `agent/evasion/blockdlls.go` | 新建：子进程启用 `ProcessSignaturePolicy` |
| Block DLL (PEB) | `agent/evasion/blockdlls_peb.go` | 新建：PEB 中设置 `BlockDlls` 标志 |

---

## Phase 16: 基础设施自动化（1 周）

### 16.1 Redirector 部署

| 组件 | 文件 | 说明 |
|------|------|------|
| Nginx 配置生成 | `internal/infra/nginx.go` | 新建：生成 Nginx redirector 配置 |
| Apache 配置生成 | `internal/infra/apache.go` | 新建：生成 Apache redirector 配置 |
| Caddy 配置生成 | `internal/infra/caddy.go` | 新建：生成 Caddy 自动 HTTPS redirector 配置 |

### 16.2 Let's Encrypt

| 组件 | 文件 | 说明 |
|------|------|------|
| ACME 客户端 | `internal/infra/acme.go` | 新建：通过 ACME 自动获取 TLS 证书 |
| 证书管理 | `internal/infra/cert_mgr.go` | 新建：到期自动续期 |

### 16.3 CDN 集成

| 组件 | 文件 | 说明 |
|------|------|------|
| Cloudflare API | `internal/infra/cloudflare.go` | 新建：Cloudflare DNS/Proxy 配置 |
| AWS CloudFront | `internal/infra/cloudfront.go` | 新建：CloudFront 分发配置 |

---

## Phase 17: 生成格式扩展（1 周）

### 17.1 多格式 Payload

| 格式 | 文件 | 说明 |
|------|------|------|
| VBA (宏) | `internal/payload/templates/vba.tmpl` | 新建：Office 宏格式 |
| HTA | `internal/payload/templates/hta.tmpl` | 新建：HTML Application |
| JScript | `internal/payload/templates/jscript.tmpl` | 新建：JScript .js |
| VBScript | `internal/payload/templates/vbscript.tmpl` | 新建：VBScript .vbs |
| C# | `internal/payload/templates/csharp.tmpl` | 新建：C# 源码 |
| Python | `internal/payload/templates/python.tmpl` | 新建：Python 脚本 |
| Ruby | `internal/payload/templates/ruby.tmpl` | 新建：Ruby 脚本 |
| PowerShell（增强） | `internal/payload/powershell_template.ps1` | 优化：更多混淆 + AMSI 绕过 |

### 17.2 One-Liner 增强

- `templates/generate.html` One-Liner 选项卡增加所有新格式
- 新增 `/generate/oneliner/<format>` 路由

---

## Phase 18: 团队协作（3 周）

### 18.1 多操作员系统

| 组件 | 文件 | 说明 |
|------|------|------|
| 操作员管理 | `internal/db/models.go` 增加 `Operator` 模型 | 新建：用户名、角色、API Key |
| DB 迁移 | `internal/db/database.go` | Operator 表 AutoMigrate |
| 角色系统 | `internal/server/rbac.go` | 新建：admin / operator / viewer 三级角色 |
| 操作员 CRUD | `internal/server/handlers_operator.go` | 新建：添加/删除/禁用操作员 |
| 操作员 UI | `templates/operators.html` | 新建操作员管理页面 |

**权限矩阵**：
| 操作 | admin | operator | viewer |
|------|-------|----------|--------|
| 生成 Agent | ✅ | ✅ | ❌ |
| 执行命令 | ✅ | ✅ | ❌ |
| 查看信标 | ✅ | ✅ | ✅ |
| 管理操作员 | ✅ | ❌ | ❌ |
| 管理 BOF | ✅ | ✅ | ❌ |
| 系统设置 | ✅ | ❌ | ❌ |

### 18.2 协作控制台

| 组件 | 文件 | 说明 |
|------|------|------|
| WebSocket 实时同步 | `internal/server/ws.go` | 新建：操作员间实时消息同步 |
| 聊天 | `templates/chat.html` | 新建：内嵌操作员聊天 |

### 18.3 审计增强

- 所有操作记录到 `audit_logs` 表，包含操作员身份
- 审计 UI 增加操作员过滤

---

## Phase 19: 事件系统与自动化（2 周）

### 19.1 事件引擎

| 组件 | 文件 | 说明 |
|------|------|------|
| 事件管理器 | `internal/server/events.go` | 新建：事件注册/触发/监听 |
| 内置事件 | — | `agent.checkin`、`agent.disconnect`、`task.complete`、`task.fail`、`credential.found` |

### 19.2 任务链

| 组件 | 文件 | 说明 |
|------|------|------|
| 任务链引擎 | `internal/server/automation.go` | 新建：条件-动作任务链 |
| Webhook | `internal/server/webhook.go` | 新建：事件触发 HTTP webhook |
| 自动化 UI | `templates/automation.html` | 新建：可视化任务链编辑 |

### 19.3 示例自动化

```
事件: agent.checkin（新机器上线）
  条件: 主机名含 "DC"
  动作: 执行 ldap_enum + 创建管理员通知

事件: credential.found
  动作: 自动尝试 pass_the_hash + 执行 secretsdump
```

---

## Phase 20: 报告生成（1 周）

### 20.1 报告引擎

| 组件 | 文件 | 说明 |
|------|------|------|
| 报告生成器 | `internal/server/report.go` | 新建：聚合 Agent 数据生成报告 |
| IOC 提取 | `internal/server/report_ioc.go` | 新建：从 beacon 日志提取网络 IOC |
| 时间线 | `internal/server/report_timeline.go` | 新建：操作时间线 |

### 20.2 报告格式

| 格式 | 说明 |
|------|------|
| HTML | 浏览器直接查看（已有基础） |
| Markdown | 可编辑，适合团队协作文档 |
| JSON | 机器可读，可作为红队 API 输出 |

### 20.3 报告内容

- 所有上线主机清单（IP、主机名、用户、OS、域）
- 凭证收集汇总（明文、hash、ticket）
- 横向路径图
- 操作时间线
- 检测指标（哪些操作可能触发告警）

---

## Phase 21: 清除/反取证（1 周）

### 21.1 Kill Date

| 组件 | 文件 | 说明 |
|------|------|------|
| Kill Date 编译期 | `agent/config.go` 增加 `KillDateStr` | 编译指定到期日期 |
| Kill Date 运行时 | `agent.go` init 检查 | 到期后自动自毁 |

### 21.2 自删除

| 组件 | 文件 | 说明 |
|------|------|------|
| 安全删除 | `agent/cleanup/self_delete.go` | 新建：覆写 + 删除自身二进制 |
| 日志清除 | `agent/cleanup/log_wipe.go` | 新建：清除 EventLog 记录 |
| 痕迹清除 | `agent/cleanup/track_wipe.go` | 新建：清除注册表/文件系统痕迹 |

**命令**：`cleanup`（一键清除所有痕迹 + 自删除）

### 21.3 自保护

| 组件 | 文件 | 说明 |
|------|------|------|
| 进程保护 | `agent/evasion/protect.go` | 新建：通过 NtSetInformationProcess 设置 PsProtectedProcess |

---

## Phase 22: 社区/BOT 生态（1 周）

### 22.1 插件系统

| 组件 | 文件 | 说明 |
|------|------|------|
| 插件管理器 | `internal/server/plugin.go` | 新建：动态加载/注册插件 |
| 插件 SDK | `internal/server/plugin_sdk.go` | 新建：插件接口定义 |

### 22.2 BOF 社区仓库

| 组件 | 文件 | 说明 |
|------|------|------|
| BOF 仓库 UI | `templates/bof_repo.html` | 新建：社区 BOF 浏览/一键导入 |
| BOF 仓库 API | `internal/server/handlers_bof_repo.go` | 新建：从 GitHub 拉取 BOF |

---

## 实施路线图

### 第一阶段（核心补齐，8 周）
| Phase | 功能 | 周期 | 优先级 |
|-------|------|------|--------|
| 8 | 高级进程注入 + 直接系统调用 | 2 周 | 🔴 P0 |
| 9 | Shellcode / Donut / sRDI 加载 | 2 周 | 🔴 P0 |
| 10 | 横向移动增强 | 2 周 | 🔴 P0 |
| 11 | 凭证攻击扩展 | 2 周 | 🔴 P0 |

### 第二阶段（信息+信道，6 周）
| Phase | 功能 | 周期 | 优先级 |
|-------|------|------|--------|
| 12 | LDAP 域信息采集 | 1 周 | 🟡 P1 |
| 13 | C2 协议扩展 | 2 周 | 🟡 P1 |
| 14 | 完整 Malleable Profile | 2 周 | 🟡 P1 |
| 17 | 生成格式扩展 | 1 周 | 🟡 P1 |

### 第三阶段（增强体验，7 周）
| Phase | 功能 | 周期 | 优先级 |
|-------|------|------|--------|
| 15 | 增强反沙箱/规避 | 2 周 | 🟢 P2 |
| 16 | 基础设施自动化 | 1 周 | 🟢 P2 |
| 18 | 团队协作 | 3 周 | 🟢 P2 |
| 19 | 事件系统与自动化 | 2 周 | 🟢 P2 |

### 第四阶段（收尾，3 周）
| Phase | 功能 | 周期 | 优先级 |
|-------|------|------|--------|
| 20 | 报告生成 | 1 周 | 🔵 P3 |
| 21 | 清除/反取证 | 1 周 | 🔵 P3 |
| 22 | 社区/BOT 生态 | 1 周 | 🔵 P3 |

**总计预估**：约 21-24 周

---

## 依赖关系图

```
Phase 8 (注入) ──────────────┐
                              ├──→ Phase 15 (规避增强)
Phase 11 (凭证) ─────────────┤
                              │
Phase 10 (横向) ─────────────┘
                              │
Phase 12 (LDAP) ◄────── Phase 11 (凭证需要 LDAP 数据)
                              │
Phase 13 (C2 协议) ────────── 独立
                              │
Phase 14 (Profile) ────────── 独立
                              │
Phase 18 (团队) ◄────── Phase 19 (自动化依赖事件系统)
                              │
Phase 20 (报告) ◄────── 所有 Phase（消费数据）
                              │
Phase 21 (清除) ───────────── 独立
                              │
Phase 22 (社区) ───────────── 独立
```

---

## 现有薄弱功能增强计划

### 并行优化（不阻塞主线）

| 功能 | 现状 | 增强内容 | 文件 |
|------|------|----------|------|
| BOF 社区 | 仅有上传/执行 | 内建 BOF 仓库浏览器 + GitHub 一键导入 | `templates/bof_repo.html` |
| Sleep Mask | VirtualAlloc+XOR | sleep skew（±25% 随机）+ 混淆周期变体 | `agent/sleepmask.go` |
| StreamCipher | SHA256 流加密 | 可选 ChaCha20 / AES-GCM | `internal/crypto/cipher.go` |
| P2P | SMB/TCP parent-child | Mesh 全互联模式 + 自动路由 | `agent/p2p/mesh.go` |
| Token | steal/make/revert | token 持久化（跨进程）、审计日志 | `agent/token_persistence.go` |
| Crypto 配置 | 页面已有输入 | 自动生成随机密钥按钮 | `templates/generate.html` |
