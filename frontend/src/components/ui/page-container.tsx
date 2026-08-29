"use client";

import type { ComponentType, ReactNode } from "react";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/ui/page-header";
import { PageSpinner } from "@/components/ui/spinner";
import { DataError } from "@/components/ui/data-state";
import { EmptyState } from "@/components/ui/empty-state";

interface PageContainerProps {
  /** Page title; when provided a PageHeader is rendered automatically. */
  title?: ReactNode;
  subtitle?: ReactNode;
  eyebrow?: ReactNode;
  icon?: ReactNode;
  /** Header actions (buttons, menus). Rendered in the PageHeader. */
  actions?: ReactNode;
  children?: ReactNode;
  /** Extra classes on the outer wrapper (e.g. `flex flex-col h-[...]`, `space-y-6`). */
  className?: string;
  /** Embedded mode: render without the page-level max-width / padding / entrance animation (used inside dialogs). */
  embedded?: boolean;
  /** Page width/scroll contract. Standard is for forms, wide for data pages, workspace fills its parent. */
  variant?: "standard" | "wide" | "workspace";
  /** Optional page-level filters/actions rendered between the header and content. */
  toolbar?: ReactNode;
  /** Optional contextual rail rendered beside the main content on wide screens. */
  aside?: ReactNode;
  /** Keep the toolbar visible inside a scrolling standard/wide page. */
  stickyHeader?: boolean;
  /** Extra classes on the inner content region. */
  contentClassName?: string;
  loading?: boolean;
  /** Custom loading placeholder (e.g. <PageSkeleton/>); defaults to a centered spinner. */
  loadingSkeleton?: ReactNode;
  error?: string | null;
  empty?: boolean;
  emptyIcon?: ComponentType<{ className?: string }>;
  emptyTitle?: string;
  emptyMessage?: string;
  emptyAction?: ReactNode;
  onRetry?: () => void;
}

/**
 * Standard page shell: content-width wrapper + entrance animation + optional
 * header and loading/error/empty states. Replaces the duplicated wrapper div
 * and PageHeader scaffolding repeated across ~70 pages.
 */
export function PageContainer({
  title,
  subtitle,
  eyebrow,
  icon,
  actions,
  children,
  className,
  embedded = false,
  variant = "wide",
  toolbar,
  aside,
  stickyHeader = false,
  contentClassName,
  loading = false,
  loadingSkeleton,
  error = null,
  empty = false,
  emptyIcon,
  emptyTitle,
  emptyMessage,
  emptyAction,
  onRetry,
}: PageContainerProps) {
  const body = loading ? (
    loadingSkeleton ?? <PageSpinner />
  ) : error ? (
    <DataError message={error} onRetry={onRetry} />
  ) : empty ? (
    <EmptyState
      icon={emptyIcon}
      title={emptyTitle ?? ""}
      message={emptyMessage}
      action={emptyAction}
    />
  ) : (
    children
  );

  const hasHeader = title !== undefined;
  const content = aside ? (
    <div className="grid min-h-0 flex-1 gap-5 xl:grid-cols-[minmax(0,1fr)_20rem] xl:gap-6">
      <div className="min-w-0">{body}</div>
      <aside className="min-w-0 xl:sticky xl:top-[calc(var(--shell-breadcrumb-height)+1rem)] xl:self-start">{aside}</aside>
    </div>
  ) : body;
  return (
    <div
      className={cn(
        "flex min-h-0 w-full flex-col gap-5 sm:gap-6",
        !embedded && "animate-fade-slide-up",
        !embedded && variant === "standard" && "mx-auto max-w-(--content-standard)",
        !embedded && variant === "wide" && "mx-auto max-w-(--content-wide)",
        variant === "workspace" && "h-full flex-1 gap-0 overflow-hidden",
        className,
        !hasHeader && contentClassName,
      )}
    >
      {hasHeader && (
        <PageHeader title={title} subtitle={subtitle} eyebrow={eyebrow} icon={icon}>
          {actions}
        </PageHeader>
      )}
      {toolbar && (
        <div className={cn(stickyHeader && "sticky top-[calc(var(--shell-breadcrumb-height)+0.5rem)] z-20")}>{toolbar}</div>
      )}
      {hasHeader ? (
        <div className={cn("flex min-h-0 flex-1 flex-col gap-5 sm:gap-6", variant === "workspace" && "gap-0", contentClassName)}>{content}</div>
      ) : (
        content
      )}
    </div>
  );
}
