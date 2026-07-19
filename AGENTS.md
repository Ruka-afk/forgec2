# ForgeC2 Project Rules

## Go Backend Workflow

每次修改或添加 Go 代码后，**必须**自动执行以下步骤：

1. `go build ./...` — 检查编译是否通过
   - 如果有编译错误，必须先修复才能继续
2. `go vet ./...` — 检查常见问题（忽略 `internal/payload/agent/` 中 unsafe.Pointer 的警告，这是 Windows 系统调用的正常用法）
3. `go mod tidy` — 确保依赖完整
4. 编译二进制：`go build -o forgec2-server.exe ./cmd/server`
5. **重启服务** — 停止旧进程并启动新服务：
    ```powershell
    # 停止旧进程
    taskkill /f /im forgec2-server.exe 2>$null
    Start-Sleep -Seconds 1

    # 启动新服务
    $proc = Start-Process -WindowStyle Hidden -FilePath ".\forgec2-server.exe" -ArgumentList "-config config.yaml" -PassThru
    Start-Sleep -Seconds 2

    # 验证服务启动成功
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:8000/health" -ErrorAction SilentlyContinue
    if ($health.status -ne "ok") { Write-Warning "Server health check failed" }
    ```

   > 一键执行以上 1–5 步：`powershell -File scripts/dev-backend.ps1`（自动 build → vet → tidy → 编译 → 重启 → 健康检查）。



```
