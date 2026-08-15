"use client";

import { cn } from "@/lib/utils";

export function MdContent({ children, className, dangerouslySetInnerHTML }: { children?: React.ReactNode; className?: string; dangerouslySetInnerHTML?: { __html: string } }) {
  return (
    <div className={cn(
      "text-sm leading-relaxed text-muted-foreground break-words",
      "[&>*:first-child]:mt-0 [&>*:last-child]:mb-0",
      "[&>p]:my-2",
      "[&>h1]:mt-3 [&>h1]:mb-1.5 [&>h1]:font-semibold [&>h1]:text-foreground [&>h1]:leading-tight [&>h1]:text-lg",
      "[&>h2]:mt-3 [&>h2]:mb-1.5 [&>h2]:font-semibold [&>h2]:text-foreground [&>h2]:leading-tight [&>h2]:text-(--fs-prose)",
      "[&>h3]:mt-3 [&>h3]:mb-1.5 [&>h3]:font-semibold [&>h3]:text-foreground [&>h3]:leading-tight [&>h3]:text-base",
      "[&>ul]:my-2 [&>ul]:pl-5 [&>ul]:list-disc",
      "[&>ol]:my-2 [&>ol]:pl-5 [&>ol]:list-decimal",
      "[&>li]:my-0.5",
      "[&>a]:text-primary [&>a]:underline [&>a]:transition-colors [&>a]:hover:text-primary/80",
      "[&>blockquote]:my-2 [&>blockquote]:py-1 [&>blockquote]:px-3 [&>blockquote]:border-l-[3px] [&>blockquote]:border-primary [&>blockquote]:text-muted-foreground [&>blockquote]:bg-primary/5 [&>blockquote]:rounded-r-md",
      "[&>hr]:my-3 [&>hr]:border-0 [&>hr]:border-t [&>hr]:border-border",
      "[&>code]:font-mono [&>code]:text-(--fs-compact) [&>code]:bg-primary/10 [&>code]:text-[var(--code-accent)] [&>code]:py-0.5 [&>code]:px-1.5 [&>code]:rounded-sm",
      "[&>pre]:my-2.5 [&>pre]:p-3 [&>pre]:bg-[var(--code-bg)] [&>pre]:text-[var(--code-text)] [&>pre]:rounded-xl [&>pre]:overflow-x-auto [&>pre]:border [&>pre]:border-border",
      "[&>pre>code]:bg-transparent [&>pre>code]:text-inherit [&>pre>code]:p-0 [&>pre>code]:text-(--fs-compact) [&>pre>code]:leading-relaxed",
      className
    )} dangerouslySetInnerHTML={dangerouslySetInnerHTML}>
      {children}
    </div>
  );
}
