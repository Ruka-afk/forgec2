# Frontend Style Conventions

操作型控制台的统一约定。新增页面/组件必须遵守；存量代码按批次收敛
（`docs/frontend-empty-state-backlog.txt` 跟踪空态残留）。

## 1. 图标尺寸

- 一律使用 Tailwind v4 的 `size-N`（如 `size-4`），禁止 `w-4 h-4` 成对写法。
- 门禁：`check-design-tokens.mjs` Rule 4 会对相邻对直接报错并给出修复命令。
- 单轴尺寸（如控件高度 `h-6`）与响应式变体（`sm:size-4`）不受限。

## 2. 空态 / 加载 / 错误

- 页面级与表格内空态一律 `<EmptyState>`；表格单元格内使用 `compact` 变体。
  禁止手写 `text-center text-muted-foreground` 居中 div。
- 图表/迷你面板内的单行数据注记（"暂无数据"）可保留 muted 单行文本——它们是
  数据注记，不是空态组件的职责范围。
- 加载用既有 `Skeleton`/`Spinner`，错误提示用 `ErrorState`，结果性反馈用
  `Banner`。禁止原生 `confirm()` —— 一律 `useConfirm()`。

## 3. 表单

- 必填字段：`<Label required htmlFor=...>`（自动渲染红色 `*`）。
- 标签统一 `text-xs`（或默认 text-sm），左对齐置于控件上方。
- 校验失败走表单自身的 aria-invalid 链路，不手写红框。

## 4. 尺寸与间距

- **间距（p/m/gap）必须使用 spacing scale**（`p-(--card-spacing)`、`gap-3`…），
  禁止 `p-[13px]` 类任意值。当前代码库间距类违例为 0，靠 review 维持。
- **布局维度**允许任意 px：表格列宽（`max-w-[200px]` 截断、`min-w-[80px]`
  列下限）、滚动区帽（`max-h-[400px]`）、结构偏移（`top-[96px]`）、触达目标
  （`min-h-[44px]`）。这些是内容相关的布局参数，spacing scale 不适用。
- 图标外的元素尺寸同理按需使用任意值。

## 5. 颜色 / 圆角 / 阴影

- 颜色一律 token（`--primary`、`--chart-*`…）；裸 hex 由门禁拦截（豁免清单见
  check-design-tokens.mjs）。
- Card 契约：`rounded-lg` + `shadow-sm→md`（映射 elevation-1/2）。xl 仅用于
  hero/图标容器。
- 动效：Card 基元已收窄为 `transition-[transform,box-shadow]`；新增交互动画
  时长 150–220ms ease-out，且尊重 prefers-reduced-motion（全局熔断已存在）。

## 6. 文案

- 所有用户可见文案经 `t("key")`，en/zh 双语同批补齐（`check:i18n` 门禁）。
- 图标键名迁移等机械操作优先脚本化（参考 `scripts/unify-icon-sizes.mjs`），
  并同步扩展对应 checker 形成防回潮闭环。
