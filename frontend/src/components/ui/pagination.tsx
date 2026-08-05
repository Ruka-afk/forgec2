"use client";

import { Button } from "@/components/ui/button";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export function Pagination({ page, pageSize, total, onPageChange }: {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (p: number) => void;
}) {
  const { t } = useI18n();
  const totalPages = Math.ceil(total / pageSize);
  if (totalPages <= 1) return null;

  const pages: (number | "...")[] = [];
  if (totalPages <= 7) {
    for (let i = 1; i <= totalPages; i++) pages.push(i);
  } else {
    pages.push(1);
    if (page > 3) pages.push("...");
    for (let i = Math.max(2, page - 1); i <= Math.min(totalPages - 1, page + 1); i++) pages.push(i);
    if (page < totalPages - 2) pages.push("...");
    pages.push(totalPages);
  }

  return (
    <nav className="flex items-center justify-between px-4 py-3 border-t border-border" aria-label={t("common.pagination")}>
      <span className="text-xs text-muted-foreground">
        {(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} / {total}
      </span>
      <div className="flex gap-1">
        <Button variant="outline" size="xs" onClick={() => onPageChange(page - 1)} disabled={page <= 1} aria-label={t("common.previous_page")}
          className="px-3">
          <ChevronLeft className="w-3 h-3" />
        </Button>
        {pages.map((p, i) =>
          p === "..." ? (
            <span key={`e${i}`} className="px-2 py-1 text-xs text-muted-foreground" aria-hidden="true">...</span>
          ) : (
            <Button key={p} size="xs" onClick={() => onPageChange(p)}
              aria-label={t("common.page_number").replace("{n}", String(p))} aria-current={p === page ? "page" : undefined}
              className={`px-3 ${p === page ? "bg-primary border-primary text-primary-foreground hover:bg-primary/90" : ""}`}
              variant={p === page ? undefined : "outline"}>
              {p}
            </Button>
          )
        )}
        <Button variant="outline" size="xs" onClick={() => onPageChange(page + 1)} disabled={page >= totalPages} aria-label={t("common.next_page")}
          className="px-3">
          <ChevronRight className="w-3 h-3" />
        </Button>
      </div>
    </nav>
  );
}
