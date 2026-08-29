"use client";

import { useEffect, useRef } from "react";
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
  // Deleting items can leave the parent on a page past the end (e.g. last
  // row of page 3 removed). Clamp and self-correct instead of rendering an
  // empty range like "41–41 / 40".
  const safePage = Math.min(Math.max(1, page), Math.max(1, totalPages));
  const cbRef = useRef(onPageChange);
  cbRef.current = onPageChange;

  useEffect(() => {
    if (safePage !== page) cbRef.current(safePage);
  }, [safePage, page]);

  if (totalPages <= 1) return null;

  const pages: (number | "...")[] = [];
  if (totalPages <= 7) {
    for (let i = 1; i <= totalPages; i++) pages.push(i);
  } else {
    pages.push(1);
    if (safePage > 3) pages.push("...");
    for (let i = Math.max(2, safePage - 1); i <= Math.min(totalPages - 1, safePage + 1); i++) pages.push(i);
    if (safePage < totalPages - 2) pages.push("...");
    pages.push(totalPages);
  }

  return (
    <nav className="flex items-center justify-between border-t border-border bg-muted/30 px-4 py-2.5" aria-label={t("common.pagination")}>
      <span className="text-xs text-muted-foreground">
        {(safePage - 1) * pageSize + 1}–{Math.min(safePage * pageSize, total)} / {total}
      </span>
      <div className="flex gap-1">
        <Button variant="outline" size="xs" onClick={() => onPageChange(safePage - 1)} disabled={safePage <= 1} aria-label={t("common.previous_page")}
          className="px-3">
          <ChevronLeft className="size-3" />
        </Button>
        {pages.map((p, i) =>
          p === "..." ? (
            <span key={`e${i}`} className="px-2 py-1 text-xs text-muted-foreground" aria-hidden="true">...</span>
          ) : (
            <Button key={p} size="xs" onClick={() => onPageChange(p)}
              aria-label={t("common.page_number").replace("{n}", String(p))} aria-current={p === safePage ? "page" : undefined}
              className={`px-3 ${p === safePage ? "bg-primary border-primary text-primary-foreground hover:bg-primary/90" : ""}`}
              variant={p === safePage ? undefined : "outline"}>
              {p}
            </Button>
          )
        )}
        <Button variant="outline" size="xs" onClick={() => onPageChange(safePage + 1)} disabled={safePage >= totalPages} aria-label={t("common.next_page")}
          className="px-3">
          <ChevronRight className="size-3" />
        </Button>
      </div>
    </nav>
  );
}
