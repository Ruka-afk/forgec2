# ForgeC2 Frontend

Next.js 16 前端，使用 Tailwind CSS 和 shadcn/ui 构建。

## 开发

```bash
npm install
npm run dev
```

访问 `http://localhost:8000` 查看结果（Go 服务器托管的嵌入式前端）。

## 构建

```bash
npm run build
```

输出目录: `out/`

## 技术栈

- Next.js 16.2 (App Router)
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

- 静态导出 (`output: "export"`)，由 Go 服务器通过 `spaFS` 提供服务
- 暗色模式 (`.dark` class on `<html>`)
- 国际化 (`useI18n()` hook)
- WebSocket 实时更新

## 禁止模式

- 使用 CDN 版 Tailwind (`public/js/tailwind.min.js`)
- `@tailwind base;` / `@tailwind components;` / `@tailwind utilities;`
- 使用 `next/font`（使用 Google Fonts CDN 链接）
