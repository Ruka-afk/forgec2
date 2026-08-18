# ForgeC2 Frontend

Vite 7 + React Router v7 前端，使用 Tailwind CSS 和 shadcn/ui 构建。

## 开发

```bash
npm install
npm run dev
```

访问 `http://localhost:5173` 查看结果（dev server 将 `/api`、`/login`、`/extc2`、`/ws` 代理到 Go 后端 `127.0.0.1:8000`）。生产模式下直接访问 Go 服务器（嵌入式 `internal/webdist/dist`）。

## 构建

```bash
npm run build
```

输出目录: `out/`

## 技术栈

- Vite 7 + React Router v7 (BrowserRouter)
- React 19
- TypeScript 5
- Tailwind CSS 4 (PostCSS)
- shadcn/ui (base-nova, `@base-ui/react`)

## 项目结构

```
src/app/          — 页面路由
src/components/   — React 组件
src/components/ui/ — shadcn/ui 原语
src/lib/          — 工具函数、API 客户端、i18n、hooks
src/types/        — TypeScript 类型定义
```

## 关键特性

- 静态导出（`src/lib/vite/staticExport.ts` 插件按 `STATIC_ROUTES` 产出 `<route>.html` + `<route>/index.html`），由 Go 服务器通过 `spaFS` 提供服务
- 暗色模式 (`.dark` class on `<html>`)
- 国际化 (`useI18n()` hook)
- WebSocket 实时更新

## 禁止模式

- 使用 CDN 版 Tailwind (`public/js/tailwind.min.js`)
- `@tailwind base;` / `@tailwind components;` / `@tailwind utilities;`
- 添加真实 Next 运行时依赖（`next/link`、`next/navigation`、`next/dynamic` 由 `src/lib/next/*` 兼容层 shim，勿还原 Next 本体）
