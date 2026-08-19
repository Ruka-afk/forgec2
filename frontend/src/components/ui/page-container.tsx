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
  return (
    <div className={cn(embedded ? undefined : "max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up", className)}>
      {title !== undefined && (
        <PageHeader title={title} subtitle={subtitle} eyebrow={eyebrow} icon={icon}>
          {actions}
        </PageHeader>
      )}
      {loading ? (
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
        <div className={contentClassName}>{children}</div>
      )}
    </div>
  );
}
