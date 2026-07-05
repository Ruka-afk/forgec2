# ForgeC2 Project Rules

## Go Backend Workflow

每次修改或添加 Go 代码后，**必须**自动执行以下步骤：

1. `go build ./...` — 检查编译是否通过
2. `go vet ./...` — 检查常见问题（忽略 `internal/payload/agent/` 中 unsafe.Pointer 的警告，这是 Windows 系统调用的正常用法）
3. `go mod tidy` — 确保依赖完整

如果有编译错误，必须先修复才能继续。

## 启动服务

编译通过后，可以用以下命令启动开发服务器：

```powershell
cd <repo_root>
go run ./cmd/server -config config.yaml
```

或编译后运行：

```powershell
go build -o forgec2-server.exe ./cmd/server
./forgec2-server.exe -config config.yaml
```

## Frontend

前端代码在 `frontend/` 目录下，使用 Next.js。修改前端代码后需要构建 JS：

```powershell
powershell -ExecutionPolicy Bypass -File ./build_js.ps1 -SkipCSS
```
