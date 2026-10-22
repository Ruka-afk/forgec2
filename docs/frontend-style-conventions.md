# Frontend Style Conventions

操作型控制台的统一约定。新增页面/组件必须遵守；存量代码按批次收敛
（`docs/frontend-empty-state-backlog.md` 跟踪空态残留 —— Class-3 清单已于
2026-10-02 全部修复或审计干净，见该文件内记录）。对比度/键盘/反馈/窄屏
等待修复偏差的清单与批次归属见 `docs/UIUX_AUDIT.md`（2026-10 审计）。

## 1. 图标尺寸

- 一律使用 Tailwind v4 的 `size-N`（如 `size-4`），禁止 `w-4 h-4` 成对写法。
- 门禁：`check-design-tokens.mjs` Rule 4 会对相邻对直接报错并给出修复命令。
- 单轴尺寸（如控件高度 `h-6`）与响应式变体（`sm:size-4`）不受限。

## 2. 空态 / 加载 / 错误

- 页面级与表格内空态一律 `<EmptyState>`；表格单元格内使用 `compact` 变体。
  禁止手写 `text-center text-muted-foreground` 居中 div。
- 图表/迷你面板内的单行数据注记（"暂无数据"）可保留 muted 单行文本——它们是
  数据注记，不是空态组件的职责范围。
- 加载用既有 `Skeleton`/`Spinner`；**读取失败**一律 `DataError`（107 处，
  约定：刷新失败保留上次成功数据、仅在"屏上无数据"时显示首败提示，见
  `useApiResource` 的 `error` 语义），泛型错误展示用 `ErrorState`；结果性
  反馈用 `Banner`。禁止原生 `confirm()` —— 一律 `useConfirm()`。

## 3. 表单

- 必填字段：`<Label required htmlFor=...>`（自动渲染红色 `*`）。
- 标签统一 `text-xs`（或默认 text-sm），左对齐置于控件上方。
- 校验失败走表单自身的 aria-invalid 链路，不手写红框。

### 3.1 表单字段用 `<Field>`

`components/ui/field.tsx` 是字段的唯一结构：标签 → 控件 → 提示/错误，
间距由它负责（`gap-1.5`），错误通过 `aria-describedby` 指向控件。

```tsx
<Field
  label={t("cred.field_username")}
  required
  htmlFor="add-cred-username"
  error={formErrors.username}
>
  <Input
    id="add-cred-username"
    aria-invalid={!!formErrors.username}
    aria-describedby={formErrors.username ? fieldErrorId("add-cred-username") : undefined}
  />
</Field>
```

- 标签样式由 `Label` 原语决定（`text-xs font-medium text-muted-foreground`），
  **调用点不要再写** `text-xs` / `font-medium` / `text-muted-foreground`，
  只写布局类（`mb-1.5`、`block`）。2026-10 之前 233 处调用点重复了这三个
  类，已机械归一。
- 必填标记用 `required` 属性，不要在标签文本里手写 `*`。
- 错误文案用 `Field` 的 `error`（内部即 `FieldError`），不要再手写
  `<FieldError>` 再手算 id。

## 4. 尺寸与间距

- **间距（p/m/gap）必须使用 spacing scale**（`p-(--card-spacing)`、`gap-3`…），
  禁止 `p-[13px]` 类任意值。当前代码库间距类违例为 0，靠 review 维持。
- **布局维度**允许任意 px：表格列宽（`max-w-[200px]` 截断、`min-w-[80px]`
  列下限）、滚动区帽（`max-h-[400px]`）、结构偏移（`top-[96px]`）、触达目标
  （`min-h-[44px]`）。这些是内容相关的布局参数，spacing scale 不适用。
- 图标外的元素尺寸同理按需使用任意值。

### 4.1 字号必须用 `text-<name>`，不能用 `text-(--fs-*)`

字号阶梯在 `@theme` 里以 `--text-<name>` 声明（`micro` / `micro-sm` /
`xs-sm` / `compact` / `page-title` / `section` / `stat-value` / `prose`），
每个都指向 `--fs-*`，因此 `[data-density="compact"]` 能整体收紧。

```tsx
<h1 className="text-page-title" />   {/* 24px，density 感知 */}
<span className="text-micro-sm" />   {/* 10px，表格头/徽标 */}
```

**禁止** `text-(--fs-compact)`：Tailwind 把它编译成
`color: var(--fs-compact)`，即把长度塞进颜色槽——浏览器直接丢弃该声明。
2026-10 之前 360 处调用点因此全部静默退回继承的 14/15px：页面标题实测
15px（应为 24px）、表头 14px（应为 10px）、徽标 14px（应为 10px）。
`check-design-tokens.mjs` 的 Rule 5/6 已把该写法与缺失的 `--text-*`
声明都变成红拦截；迁移脚本为 `scripts/migrate-fs-utilities.mjs`。

### 4.2 页面骨架节奏：`--page-pad-x` / `--page-pad-y` / `--page-gap`

页面级留白与块间距只有一套令牌，定义在 `globals.css` 的 `:root`
（`<640px` 16/16/16；`≥640px` 24/20/20；`≥1024px` 32/24/20）：

- `AppLayout` 只负责**页面外边距**（`px-(--page-pad-x) py-(--page-pad-y)`）；
  内容宽度归 `PageContainer`（`standard` 80rem / `wide` 96rem / `workspace`
  满宽）。壳层不再自带 `max-w`，避免两层同值裁切。
- 页面块的纵向间距用 `gap-(--page-gap)`（容器层）或
  `flex flex-col gap-(--page-gap)`（页面自带的分组根节点）。
  **禁止** `space-y-4/5/6` 出现在页面级分组根节点上：它不随断点变化，
  同一屏里就会出现 16 / 20 / 24 三种块间距。表单、弹窗、对话框内部的
  `space-y-4` 不受此限。
- **Tailwind v4 不编译 `space-y` 的自定义属性写法**：`space-y-(--page-gap)`
  与 `space-y-[var(--page-gap)]` 都不产出 CSS（实测），页面级分组只能用
  `gap-(--page-gap)`。
- 吸顶筛选条只有一个实现：`PageContainer` 的 `toolbar` 槽位
  （`stickyToolbar`，默认吸顶）。槽位是页面容器的直接子元素，sticky 的
  位移预算 = 页头高度；把筛选条放在 body 的**第一个**位置时，sticky 会被
  父盒顶边锁死为 0（父盒顶边即筛选条顶边），永远无法吸顶——这正是
  Dashboard 的视图切换条此前随页面滚走的原因。

### 4.3 卡片内距与列表行

- 卡片内距只有一个值：`--card-spacing`（16px，≥640px 为 20px）。
  `<Card className="p-4">` 这类硬编码一律改成 `p-(--card-spacing)`；
  贴边表格卡保留 `p-0`。
- 列表行用 `components/ui/list-row.tsx`（左侧槽 + 两行文字 + 右侧槽 +
  共享 hover 面），不要在页面里复制 `flex items-center justify-between
  gap-2 px-4 py-2 hover:bg-secondary/50`。
- 宽表格必须钉住操作列：给末列的 `TableHead` / `TableCell` 加
  `sticky-end`。该类把单元格粘在右边缘，并绘制行自身底色（斑马纹/hover），
  滚动时列从下方穿过也不会透出。**横向溢出的表格若未钉住任何列，视为
  布局违约**（`scripts/audit-layout.mjs` 门禁）。
- 次要列随视口逐级让位（`max-md:hidden` / `max-lg:hidden` /
  `max-xl:hidden` / `max-2xl:hidden`），不要只在 `sm` 以下隐藏。

### 4.4 对话框与抽屉

- 对话框宽度用 `size` 属性：`sm` / `md`（默认）/ `lg` / `xl` / `2xl` /
  `full`。**禁止**在 `DialogContent` 的 `className` 里写 `max-w-*` 或
  `w-[..vw]`（72 处曾散落 9 种宽度 + 1 个裸 vw）。
- 对话框内距是令牌 `--dialog-padding`；页脚的负边距用同一个令牌抵消它，
  因此两者不会各自漂移。页脚底色 `--dialog-footer-bg` 为不透明解析，
  固定后不会透出下方表单。
- 长表单的标题栏与操作栏固定（`DialogHeader` / `DialogFooter` 自带
  `sticky`），表单在两者之间滚动；标题栏用伪元素盖住弹窗自身的上内边距，
  **不要**给标题栏加负外边距（那会把下方内容拉到标题底下）。
- 抽屉宽度用 `SheetContent` 的 `size`：`xs` / `sm` / `md` / `lg` / `xl` /
  `full`（仅左右侧栏生效）。

## 5. 颜色 / 圆角 / 阴影

- 颜色一律 token（`--primary`、`--chart-*`…）；裸 hex 由门禁拦截（豁免清单见
  check-design-tokens.mjs）。
- 不写 `/100` 透明度修饰（`text-muted-foreground/100` 等价于无修饰，纯噪声）。
- 状态色有四职责（2026-10 U1a 起）：`--X` 是基础色（fill / border / dot，**不做浅底上的文字**）；
  `--X-solid` 是实心填充（dark 下 warning/destructive 会加深，保证浅墨对比）；
  `--X-foreground` 是实心填充上的墨色；`--X-ink` 是中性面与 `bg-X/8..20` 淡底上的文字/图标色。
  映射见 `lib/ui/statusStyles.ts`；浅底禁用裸 `text-X`（U1b 已全量迁移；
  `check:contrast` 已入 `npm run check`，违例即红拦截）。
- Card 契约：`rounded-lg` + `shadow-sm→md`（映射 elevation-1/2）。xl 仅用于
  hero/图标容器。
- 动效：Card 基元已收窄为 `transition-[transform,box-shadow]`；新增交互动画
  时长 150–220ms ease-out，且尊重 prefers-reduced-motion（全局熔断已存在）。

## 6. 文案

- 所有用户可见文案经 `t("key")`，en/zh 双语同批补齐（`check:i18n` 门禁）。
- i18n 边缘规则（2026-10-02 两批清扫后固化）：
  - 库层（无 hook 上下文）的错误串 / 控制流标记（如 `lib/agents.ts`、`lib/errors.ts`
    的 `"Aborted"` token）可保留英文回退；**UI 调用点必须包裹** `t(key, { message })`
    （如 `agents.list_unreadable`），禁止把库层原文直接 toast / 渲染。
  - 客户端查找表渲染一律走 i18n 键：bof quick 库用 `descKey` 模式、generate 的
    `FORMAT_META` 用 `generate.format_desc_*`、回退标签复用/新增
    （`agents.unknown` / `listeners.unknown` / `cloud.raw` / `generate.unknown_listener`）。
  - 纯数据映射函数无法调 hook 时保留空串哨兵（`credential-types` 模式），
    组件层再渲染 `dashboard.unknown`。
  - 通用短形标注可原样保留：REC / WS / N/A / fps。
- 图标键名迁移等机械操作优先脚本化（参考 `scripts/unify-icon-sizes.mjs`），
  并同步扩展对应 checker 形成防回潮闭环。
