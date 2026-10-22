
import type { ComponentType, ReactNode } from "react";
import { useLocation } from "react-router-dom";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/ui/page-header";
import { Breadcrumb } from "@/components/ui/breadcrumb";
import { PageSpinner } from "@/components/ui/spinner";
import { DataError } from "@/components/ui/data-state";
import { EmptyState } from "@/components/ui/empty-state";
import { isFlushPath } from "@/lib/layout";
import { useI18n } from "@/lib/i18n";

interface PageContainerProps {
  /** Page title; when provided a PageHeader is rendered automatically. */
  title?: ReactNode;
  subtitle?: ReactNode;
  eyebrow?: ReactNode;
  icon?: ReactNode;
  /** Secondary inline metadata rendered next to the subtitle (e.g. status). */
  meta?: ReactNode;
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
  /** Keep the toolbar row pinned under the top bar while the page scrolls. */
  stickyToolbar?: boolean;
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
 * Router context is optional: dialogs and unit tests render pages standalone.
 * `useLocation` throws outside a <Router>, so treat that as "no pathname"
 * (no breadcrumb, no flush layout) instead of failing the whole page.
 */
function useOptionalPathname(): string | null {
  try {
    return useLocation().pathname;
  } catch {
    return null;
  }
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
  meta,
  actions,
  children,
  className,
  embedded = false,
  variant = "wide",
  toolbar,
  stickyToolbar = true,
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
  const { t } = useI18n();
  // null means "outside a Router" (standalone render), so skip the breadcrumb.
  const pathname = useOptionalPathname();
  const breadcrumb = !pathname || isFlushPath(pathname) ? null : <Breadcrumb />;
  const body = loading ? (
    loadingSkeleton ?? <PageSpinner />
  ) : error ? (
    <DataError message={error} onRetry={onRetry} />
  ) : empty ? (
    <EmptyState
      icon={emptyIcon}
      title={emptyTitle ?? t("common.no_data")}
      message={emptyMessage}
      action={emptyAction}
    />
  ) : (
    children
  );

  const hasHeader = title !== undefined;
  const content = body;
  return (
    <div
      data-slot="page-container"
      className={cn(
        "flex min-h-0 w-full flex-col gap-(--page-gap)",
        !embedded && "animate-fade-slide-up",
        !embedded && variant === "standard" && "mx-auto max-w-(--content-standard)",
        !embedded && variant === "wide" && "mx-auto max-w-(--content-wide)",
        variant === "workspace" && "h-full flex-1 gap-0 overflow-hidden",
        className,
        !hasHeader && contentClassName,
      )}
    >
      {hasHeader ? (
        <PageHeader title={title} subtitle={subtitle} eyebrow={eyebrow} icon={icon} meta={meta} breadcrumb={breadcrumb}>
          {actions}
        </PageHeader>
      ) : breadcrumb ? (
        <div className="min-w-0">{breadcrumb}</div>
      ) : null}
      {toolbar && (
        <div
          className={cn(
            "min-w-0",
            // The toolbar row is pinned under the top bar while the page
            // scrolls. It lives in the page container (not in the scrolling
            // body), so the sticky travel budget is the page header's height
            // instead of zero — a toolbar that was the body's first child
            // could never pin, because the sticky box is bounded by its
            // parent's top edge.
            stickyToolbar &&
              "sticky top-0 z-20 -mx-1 rounded-xl bg-background/95 py-1 backdrop-blur supports-[backdrop-filter]:bg-background/85 sm:-mx-2 sm:py-1.5",
          )}
        >
          {toolbar}
        </div>
      )}
      {hasHeader ? (
        <div className={cn("flex min-h-0 flex-1 flex-col gap-(--page-gap)", variant === "workspace" && "gap-0", contentClassName)}>{content}</div>
      ) : (
        content
      )}
    </div>
  );
}
