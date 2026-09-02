"use client";

import { cn } from "@/lib/utils";

export function MdContent({ children, className, dangerouslySetInnerHTML }: { children?: React.ReactNode; className?: string; dangerouslySetInnerHTML?: { __html: string } }) {
  return (
    <div className={cn(
      "break-words text-sm leading-7 text-foreground/85",
      "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
      "[&>p]:my-3 [&>p]:max-w-none",
      "[&_strong]:font-semibold [&_strong]:text-foreground",
      "[&>h1]:mb-3 [&>h1]:mt-5 [&>h1]:border-b [&>h1]:border-border/70 [&>h1]:pb-2 [&>h1]:text-xl [&>h1]:font-semibold [&>h1]:leading-tight [&>h1]:tracking-tight [&>h1]:text-foreground",
      "[&>h2]:mb-2 [&>h2]:mt-5 [&>h2]:border-l-2 [&>h2]:border-primary/60 [&>h2]:pl-2.5 [&>h2]:text-(--fs-prose) [&>h2]:font-semibold [&>h2]:leading-tight [&>h2]:text-foreground",
      "[&>h3]:mb-2 [&>h3]:mt-4 [&>h3]:text-base [&>h3]:font-semibold [&>h3]:leading-tight [&>h3]:text-foreground",
      "[&>ul]:my-3 [&>ul]:list-disc [&>ul]:space-y-1 [&>ul]:pl-5 [&>ul>li]:pl-1 [&>ul>li]:marker:text-primary",
      "[&>ol]:my-3 [&>ol]:list-decimal [&>ol]:space-y-1 [&>ol]:pl-5 [&>ol>li]:pl-1 [&>ol>li]:marker:font-semibold [&>ol>li]:marker:text-primary",
      "[&_a]:font-medium [&_a]:text-primary [&_a]:underline [&_a]:decoration-primary/30 [&_a]:underline-offset-4 [&_a]:transition-colors [&_a]:hover:decoration-primary",
      "[&>blockquote]:my-4 [&>blockquote]:rounded-r-lg [&>blockquote]:border-l-[3px] [&>blockquote]:border-info [&>blockquote]:bg-info/5 [&>blockquote]:px-4 [&>blockquote]:py-2.5 [&>blockquote]:text-foreground/75",
      "[&>hr]:my-5 [&>hr]:border-0 [&>hr]:border-t [&>hr]:border-border/70",
      "[&_p>code]:rounded-md [&_p>code]:bg-primary/8 [&_p>code]:px-1.5 [&_p>code]:py-0.5 [&_p>code]:font-mono [&_p>code]:text-(--fs-compact) [&_p>code]:text-[var(--code-accent)]",
      "[&_li>code]:rounded-md [&_li>code]:bg-primary/8 [&_li>code]:px-1.5 [&_li>code]:py-0.5 [&_li>code]:font-mono [&_li>code]:text-(--fs-compact) [&_li>code]:text-[var(--code-accent)]",
      "[&_.md-code-block]:my-4 [&_.md-code-block]:overflow-hidden [&_.md-code-block]:rounded-xl [&_.md-code-block]:border [&_.md-code-block]:border-border [&_.md-code-block]:bg-[var(--code-bg)] [&_.md-code-block]:shadow-xs",
      "[&_.md-code-head]:flex [&_.md-code-head]:h-8 [&_.md-code-head]:items-center [&_.md-code-head]:border-b [&_.md-code-head]:border-white/10 [&_.md-code-head]:px-3 [&_.md-code-head]:font-mono [&_.md-code-head]:text-(--fs-micro-sm) [&_.md-code-head]:font-medium [&_.md-code-head]:uppercase [&_.md-code-head]:tracking-wide [&_.md-code-head]:text-[var(--code-text)]",
      "[&_.md-code-block_pre]:m-0 [&_.md-code-block_pre]:max-h-96 [&_.md-code-block_pre]:overflow-auto [&_.md-code-block_pre]:p-4 [&_.md-code-block_pre]:text-[var(--code-text)]",
      "[&_.md-code-block_code]:bg-transparent [&_.md-code-block_code]:p-0 [&_.md-code-block_code]:font-mono [&_.md-code-block_code]:text-(--fs-compact) [&_.md-code-block_code]:leading-relaxed [&_.md-code-block_code]:text-inherit",
      "[&_.md-table-wrap]:my-4 [&_.md-table-wrap]:max-w-full [&_.md-table-wrap]:overflow-x-auto [&_.md-table-wrap]:rounded-xl [&_.md-table-wrap]:border [&_.md-table-wrap]:border-border",
      "[&_.md-table-wrap_table]:w-full [&_.md-table-wrap_table]:min-w-max [&_.md-table-wrap_table]:border-collapse [&_.md-table-wrap_table]:text-left [&_.md-table-wrap_table]:text-xs",
      "[&_.md-table-wrap_th]:border-b [&_.md-table-wrap_th]:border-border [&_.md-table-wrap_th]:bg-muted/80 [&_.md-table-wrap_th]:px-3 [&_.md-table-wrap_th]:py-2.5 [&_.md-table-wrap_th]:font-semibold [&_.md-table-wrap_th]:text-foreground",
      "[&_.md-table-wrap_td]:border-b [&_.md-table-wrap_td]:border-border/60 [&_.md-table-wrap_td]:px-3 [&_.md-table-wrap_td]:py-2.5 [&_.md-table-wrap_td]:align-top [&_.md-table-wrap_tr:last-child_td]:border-b-0",
      "[&_.ai-source-ref]:mx-0.5 [&_.ai-source-ref]:inline-flex [&_.ai-source-ref]:max-w-full [&_.ai-source-ref]:items-center [&_.ai-source-ref]:rounded-full [&_.ai-source-ref]:border [&_.ai-source-ref]:border-primary/20 [&_.ai-source-ref]:bg-primary/8 [&_.ai-source-ref]:px-2 [&_.ai-source-ref]:py-0.5 [&_.ai-source-ref]:font-mono [&_.ai-source-ref]:text-(--fs-micro-sm) [&_.ai-source-ref]:leading-5 [&_.ai-source-ref]:text-primary",
      className
    )} dangerouslySetInnerHTML={dangerouslySetInnerHTML}>
      {children}
    </div>
  );
}
