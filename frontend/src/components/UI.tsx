"use client";

const STATUS_CONFIG: Record<string, { dot: string; bg: string; text: string }> = {
  online:    { dot: "bg-emerald-500", bg: "bg-emerald-500/10 dark:bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400" },
  offline:   { dot: "bg-slate-400", bg: "bg-slate-500/10 dark:bg-slate-500/10", text: "text-slate-600 dark:text-[var(--text-tertiary)]" },
  stale:     { dot: "bg-amber-500", bg: "bg-amber-500/10 dark:bg-amber-500/10", text: "text-amber-700 dark:text-amber-400" },
  locked:    { dot: "bg-red-500", bg: "bg-red-500/10 dark:bg-red-500/10", text: "text-red-700 dark:text-red-400" },
  completed: { dot: "bg-emerald-500", bg: "bg-emerald-500/10 dark:bg-emerald-500/10", text: "text-emerald-700 dark:text-emerald-400" },
  failed:    { dot: "bg-red-500", bg: "bg-red-500/10 dark:bg-red-500/10", text: "text-red-700 dark:text-red-400" },
  pending:   { dot: "bg-slate-400", bg: "bg-slate-500/10 dark:bg-slate-500/10", text: "text-slate-600 dark:text-[var(--text-tertiary)]" },
  running:   { dot: "bg-blue-500", bg: "bg-blue-500/10 dark:bg-blue-500/10", text: "text-blue-700 dark:text-blue-400" },
};

export function StatusBadge({ status, pulse }: { status: string; pulse?: boolean }) {
  const cfg = STATUS_CONFIG[status] || STATUS_CONFIG.offline;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium border border-transparent ${cfg.bg} ${cfg.text}`}>
      <span className={`w-1.5 h-1.5 rounded-full ${cfg.dot} ${pulse ? "animate-pulse" : ""}`} />
      {status}
    </span>
  );
}

export function PageHeader({ title, subtitle, children }: { title: React.ReactNode; subtitle?: string; children?: React.ReactNode }) {
  return (
    <div className="flex flex-col sm:flex-row sm:items-center justify-between mb-4 sm:mb-6 gap-3">
      <div>
        <h1 className="page-title">{title}</h1>
        {subtitle && <p className="page-subtitle">{subtitle}</p>}
      </div>
      {children && <div className="flex items-center gap-2 shrink-0">{children}</div>}
    </div>
  );
}

export function SearchInput({ value, onChange, placeholder = "Search...", className = "" }: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  className?: string;
}) {
  return (
    <div className={`relative ${className}`}>
      <i className="fa-solid fa-magnifying-glass absolute left-3 top-1/2 -translate-y-1/2 text-xs text-[var(--text-tertiary)]"></i>
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full pl-9 pr-4 h-9 bg-[var(--card-bg)] border border-[var(--border)] rounded-lg text-sm focus:outline-none focus:border-indigo-500 transition-colors text-[var(--text-primary)] placeholder:text-[var(--text-tertiary)]"
      />
    </div>
  );
}

export function TableCard({ header, children, responsive }: { header?: React.ReactNode; children: React.ReactNode; responsive?: boolean }) {
  return (
    <div className="ui-card overflow-hidden">
      {header && <div className="px-4 py-3 border-b border-[var(--border)] text-sm font-medium text-[var(--text-primary)]">{header}</div>}
      <div className={`${responsive ? 'overflow-x-auto' : 'overflow-x-auto'}`}>
        {children}
      </div>
    </div>
  );
}

export function Pagination({ page, pageSize, total, onPageChange }: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (p: number) => void;
}) {
  const totalPages = Math.ceil(total / pageSize);
  if (totalPages <= 1) return null;

  return (
    <div className="flex items-center justify-between px-4 py-3 border-t border-[var(--border)]">
      <span className="text-xs text-[var(--text-secondary)]">
        {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}
      </span>
      <div className="flex gap-1">
        <button onClick={() => onPageChange(page - 1)} disabled={page <= 1}
          className="px-3 py-1 text-xs rounded-lg border border-[var(--border)] disabled:opacity-50 hover:bg-[var(--card-bg-secondary)] text-[var(--text-secondary)] transition-colors">
          <i className="fa-solid fa-chevron-left text-[10px]"></i>
        </button>
        <button onClick={() => onPageChange(page + 1)} disabled={page >= totalPages}
          className="px-3 py-1 text-xs rounded-lg border border-[var(--border)] disabled:opacity-50 hover:bg-[var(--card-bg-secondary)] text-[var(--text-secondary)] transition-colors">
          <i className="fa-solid fa-chevron-right text-[10px]"></i>
        </button>
      </div>
    </div>
  );
}

export function ConfirmModal({ open, title, message, confirmText = "Confirm", cancelText = "Cancel", danger, onConfirm, onCancel }: {
  open: boolean;
  title: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  if (!open) return null;
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-fade-in" onClick={onCancel}>
      <div className="bg-[var(--card-bg)] rounded-2xl p-6 w-full max-w-sm shadow-lg border border-[var(--border)] mx-4 animate-slide-in" onClick={(e) => e.stopPropagation()}>
        <h3 className="text-base font-semibold text-[var(--text-primary)] mb-2">{title}</h3>
        <p className="text-sm text-[var(--text-secondary)] mb-5">{message}</p>
        <div className="flex items-center justify-end gap-2">
          <button type="button" onClick={onCancel} className="px-4 h-9 bg-[var(--card-bg-secondary)] hover:bg-[var(--border)] text-[var(--text-secondary)] text-xs font-medium rounded-xl transition-colors">{cancelText}</button>
          <button type="button" onClick={onConfirm} className={`px-4 h-9 text-xs font-medium rounded-xl transition-colors text-white ${danger ? "bg-red-600 hover:bg-red-700" : "bg-indigo-600 hover:bg-indigo-700"}`}>{confirmText}</button>
        </div>
      </div>
    </div>
  );
}
