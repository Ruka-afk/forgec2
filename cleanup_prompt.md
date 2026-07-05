# ForgeC2 无用代码清理任务

## 目标
清理项目中所有不再使用的代码、死参数、无效路由和僵尸功能。

## 清理清单

### 1. 移除 `renderPageOrJSON` 中遗留的模板名参数
`internal/server/render_api.go:10` 中 `renderPageOrJSON` 第二个参数 `_ string` 是旧模板引擎遗留物，已完全不用。所有 39 个调用处传入了无意义的字符串（如 `"dashboard_content"`、`"agents_content"` 等）。

**操作：**
- 将 `renderPageOrJSON` 签名改为 `func (s *Server) renderPageOrJSON(c *gin.Context, data gin.H)`
- 删除全部 39 个调用处的第一个字符串参数

**涉及文件：**
- `internal/server/render_api.go`（函数定义）
- `internal/server/handlers_agents.go`（8 处调用）
- `internal/server/handlers_auth.go`（3 处调用）
- `internal/server/handlers_automation.go`（2 处调用）
- `internal/server/handlers_ai.go`（1 处）
- `internal/server/handlers_bof.go`（1 处）
- `internal/server/handlers_commands.go`（1 处）
- `internal/server/handlers_credentials.go`（1 处）
- `internal/server/handlers_files.go`（1 处）
- `internal/server/handlers_generate.go`（3 处）
- `internal/server/handlers_infra.go`（1 处）
- `internal/server/handlers_lateral.go`（1 处）
- `internal/server/handlers_monitor.go`（1 处）
- `internal/server/handlers_ops.go`（1 处）
- `internal/server/handlers_privesc.go`（1 处）
- `internal/server/handlers_report.go`（1 处）
- `internal/server/handlers_scan.go`（1 处）
- `internal/server/handlers_scripting.go`（1 处）
- `internal/server/handlers_search.go`（1 处）
- `internal/server/handlers_templates.go`（1 处）
- `internal/server/handlers_timeline.go`（1 处）
- `internal/server/handlers_token.go`（2 处）
- `internal/server/handlers_toolkit.go`（1 处）
- `internal/server/handlers_traffic.go`（1 处）
- `internal/server/handlers_users.go`（1 处）
- `internal/server/server.go`（1 处）

### 2. 删除前端失效的 `/chat` 侧边栏链接和 i18n 键
`frontend/src/components/Sidebar.tsx:67` 有指向 `/chat` 的导航链接，但聊天页面已被删除。

- `frontend/src/components/Sidebar.tsx` — 删除 `{ href: "/chat", labelKey: "nav.chat", icon: "fa-solid fa-comments" }` 条目
- `internal/server/locales.go` — 删除 `"nav.chat"` i18n 键（en/zh/ja/ko/ar 共 5 处）

### 3. 删除 BackupManager 中未使用的死方法
`internal/server/backup.go` 中以下方法从未被调用：

- `Restore()` — line 241
- `ListBackups()` — line 269
- `GenerateKey()` — line 312
- `ValidateKey()` — line 320
- `VerifyBackup()` — line 343
- `SHA256File()` — line 328

**操作：** 删除这些方法。`handleBackupDatabase`（`handlers_auth.go:672`）自己做文件复制，不依赖于 BackupManager。

### 4. 删除无效的 SRDI 生成路由
`internal/payload/shellcode.go:306` 的 `GenerateSRDIShellcode()` 始终返回错误。其对应路由 `POST /generate/srdi`（`server.go:487`）和前端表单项也一并删除。

- 删除 `GenerateSRDIShellcode` 函数
- 删除路由 `agentCmd.POST("/generate/srdi", s.handleGenerateSRDI)` 和 `handleGenerateSRDI` handler
- 检查前端 generate 页面是否引用了 srdi

### 5. 删除未使用的 `MalleableInfo()` 方法
`internal/server/profile.go:78` — 从未在任何地方被调用。

### 6. 删除未使用的 x86 shellcode 生成路径
`internal/payload/shellcode.go:13` 的 `buildPowershellWinExecShellcode()` 仅被 `donut.go:126` 调用，且永远传 `x64=true`；x86 分支 `buildPowershellWinExecShellcodeX86()`（line 143）是死代码。

### 7. 删除仅在测试中使用的 malleable 辅助方法
以下方法只被 `profile_test.go` 引用，生产代码中无用：
- `HTTPGet.RandomURI()` — `profile.go:382`
- `HTTPPost.RandomURI()` — `profile.go:389`
- `JitterCfg.RandomPadding()` — `profile.go:397`
- `JitterCfg.RandomParamName()` — `profile.go:413`

**操作：** 删除这四个方法（如果不想破坏测试，可以把实现移到测试文件中）。

## 验证
每完成一项清理后：
1. `go build ./...` 确保编译通过
2. `go vet ./...` 确保无警告
3. `go test ./...` 确保测试通过
