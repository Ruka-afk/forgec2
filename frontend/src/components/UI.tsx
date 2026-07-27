// Re-export all components from their individual files
export { Spinner, PageSpinner } from "@/components/ui/spinner";
export { PageHeader } from "@/components/ui/page-header";
export { EmptyState } from "@/components/ui/empty-state";
export { StatusBadge } from "@/components/ui/status-badge";
export { Pagination } from "@/components/ui/pagination";
export { ConfirmModal } from "@/components/ui/confirm-modal";
export { CopyButton } from "@/components/ui/copy-button";
export { MdContent } from "@/components/ui/md-content";
export { AvatarRoot, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
export { Breadcrumb } from "@/components/ui/breadcrumb";
export { ToastProvider, toast } from "@/components/ui/toast";
export { StatusIndicator } from "@/components/ui/status-indicator";
export { IconBadge } from "@/components/ui/icon-badge";
export { StatCard } from "@/components/ui/animated-stat-card";
export { PageSection } from "@/components/ui/page-section";
export { DataState, DataSpinner, DataError } from "@/components/ui/data-state";
export { ChartCard } from "@/components/ChartCard";
export { Separator } from "@/components/ui/separator";
export { TableSkeleton, CardGridSkeleton, ChartSkeleton, ListSkeleton, StatSkeleton, AgentGridSkeleton } from "@/components/ui/skeletons";

/* ── TableCard (2 consumers — keep here) ── */

export function TableCard({ header, children }: { header?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="rounded-2xl border border-border bg-card shadow-sm overflow-hidden">
      {header && <div className="px-5 py-3.5 border-b border-border text-sm font-medium text-foreground">{header}</div>}
      <div className="overflow-x-auto">
        {children}
      </div>
    </div>
  );
}
