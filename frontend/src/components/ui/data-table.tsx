"use client";

import * as React from "react";
import { useEffect, useMemo, useRef, useState } from "react";
import { ArrowDown, ArrowUp, ChevronsUpDown } from "lucide-react";
import { cn } from "@/lib/utils";
import { TableHeader, TableBody, TableRow, TableHead, TableCell } from "@/components/ui/table";
import { Pagination } from "@/components/ui/pagination";
import { DataState } from "@/components/ui/data-state";
import { Skeleton } from "@/components/ui/skeleton";
import { computeVirtualRange, VIRTUAL_THRESHOLD } from "@/lib/virtual";
import { useI18n } from "@/lib/i18n";

type SortDirection = "asc" | "desc";

export interface DataTableColumn<T> {
  /** Stable id — used as the React key and sort column identifier. */
  id: string;
  header: React.ReactNode;
  /** Render the cell content from a row. */
  cell: (row: T) => React.ReactNode;
  /** Optional sort key for client-side sorting (auto-sortable when set). */
  sortValue?: (row: T) => string | number;
  headerClassName?: string;
  cellClassName?: string;
  align?: "left" | "right" | "center";
  width?: string | number;
  /** Responsive hiding, e.g. "hidden md:table-cell" to hide below md. */
  hidden?: string;
}

type SortState = { column: string; dir: SortDirection };

/** Default row height used by the virtualizer (override via rowHeight). */
const DEFAULT_ROW_HEIGHT = 40;

interface DataTableProps<T> {
  data: T[];
  columns: DataTableColumn<T>[];
  rowKey: (row: T, index: number) => string;

  // async state
  loading?: boolean;
  error?: string | null;
  onRetry?: () => void;
  loadingSkeletonRows?: number;

  // empty state
  emptyTitle?: string;
  emptyMessage?: string;
  emptyIcon?: React.ComponentType<{ className?: string }>;

  // row interaction
  onRowClick?: (row: T) => void;
  /** Accessible label for an interactive row. Defaults to a localized row number. */
  rowAriaLabel?: (row: T, index: number) => string;

  // toolbar rendered above the scroll container
  toolbar?: React.ReactNode;

  // server-side pagination. Omit to render all rows (client-side sorting applies).
  pagination?: {
    page: number;
    pageSize: number;
    total: number;
    onPageChange: (p: number) => void;
  };

  // client-side sorting. Controlled via sort + onSortChange, or uncontrolled via
  // defaultSort (internal state).
  sort?: SortState | null;
  defaultSort?: SortState;
  onSortChange?: (sort: SortState) => void;

  // virtualization. Enables automatically when rows exceed VIRTUAL_THRESHOLD.
  virtualize?: boolean;
  rowHeight?: number;
  overscan?: number;

  className?: string;
  tableClassName?: string;

  /** Max height of the scroll container (CSS value, e.g. "60vh" or "32rem"). */
  maxHeight?: string;
}

export function DataTable<T>({
  data,
  columns,
  rowKey,
  loading = false,
  error = null,
  onRetry,
  loadingSkeletonRows = 5,
  emptyTitle,
  emptyMessage,
  emptyIcon,
  onRowClick,
  rowAriaLabel,
  toolbar,
  pagination,
  sort: sortProp,
  defaultSort,
  onSortChange,
  virtualize = true,
  rowHeight = DEFAULT_ROW_HEIGHT,
  overscan = 6,
  className,
  tableClassName,
  maxHeight = "min(60vh, 36rem)",
}: DataTableProps<T>) {
  const { t } = useI18n();
  const [internalSort, setInternalSort] = useState<SortState | null>(defaultSort ?? null);
  const sort = sortProp !== undefined ? sortProp : internalSort;

  const handleSort = (columnId: string) => {
    const next: SortState =
      sort?.column === columnId && sort.dir === "asc"
        ? { column: columnId, dir: "desc" }
        : { column: columnId, dir: "asc" };
    if (onSortChange) onSortChange(next);
    else setInternalSort(next);
  };

  const sorted = useMemo(() => {
    if (!sort) return data;
    const col = columns.find((c) => c.id === sort.column);
    if (!col?.sortValue) return data;
    const dir = sort.dir === "asc" ? 1 : -1;
    return [...data].sort((a, b) => {
      const va = col.sortValue!(a);
      const vb = col.sortValue!(b);
      if (typeof va === "number" && typeof vb === "number") return (va - vb) * dir;
      return String(va).localeCompare(String(vb), undefined, { numeric: true }) * dir;
    });
  }, [data, sort, columns]);

  // `pagination` is server-controlled: `data` already contains the requested
  // page. Slicing again would turn every page after page 1 into an empty table.
  const rows = sorted;

  const scrollRef = useRef<HTMLDivElement>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewportHeight, setViewportHeight] = useState(0);
  const [measuredRowHeight, setMeasuredRowHeight] = useState(rowHeight);
  const rowHeightRef = useRef(rowHeight);
  useEffect(() => { rowHeightRef.current = rowHeight; setMeasuredRowHeight(rowHeight); }, [rowHeight]);

  const shouldVirtualize = virtualize && !pagination && rows.length > VIRTUAL_THRESHOLD;

  useEffect(() => {
    if (!shouldVirtualize) return;
    const el = scrollRef.current;
    if (!el) return;
    const measure = () => setViewportHeight(el.clientHeight);
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [shouldVirtualize]);

  useEffect(() => {
    if (!shouldVirtualize) return;
    const el = scrollRef.current;
    if (!el) return;
    const probe = () => {
      const tr = el.querySelector<HTMLElement>("tbody tr[data-row-key]");
      if (tr && tr.offsetHeight > 0 && Math.abs(tr.offsetHeight - rowHeightRef.current) > 1) {
        rowHeightRef.current = tr.offsetHeight;
        setMeasuredRowHeight(tr.offsetHeight);
      }
    };
    probe();
    if (typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver(probe);
    ro.observe(el);
    return () => ro.disconnect();
  }, [shouldVirtualize]);

  const effRowHeight = shouldVirtualize ? measuredRowHeight : rowHeight;
  const range = useMemo(
    () => computeVirtualRange(rows.length, effRowHeight, scrollTop, viewportHeight, overscan),
    [rows.length, effRowHeight, scrollTop, viewportHeight, overscan],
  );

  const visibleRows = shouldVirtualize ? rows.slice(range.start, range.end) : rows;
  const spacerTop = shouldVirtualize ? range.offsetTop : 0;
  const spacerBottom = shouldVirtualize
    ? range.totalHeight - range.offsetTop - visibleRows.length * effRowHeight
    : 0;

  return (
    <div className={cn("w-full", className)}>
      {toolbar}
      <DataState
        loading={loading}
        error={error}
        onRetry={onRetry}
        empty={!loading && !error && data.length === 0}
        emptyTitle={emptyTitle}
        emptyMessage={emptyMessage}
        emptyIcon={emptyIcon}
        loadingSkeleton={
          <div className="overflow-x-auto rounded-lg border border-border">
            <table className="w-full caption-bottom text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  {columns.map((col) => (
                    <th
                      key={col.id}
                      className={cn(
                        "h-9 px-3 text-left text-(--fs-micro-sm) uppercase tracking-(--tracking-sublabel) text-muted-foreground",
                        col.headerClassName,
                      )}
                    >
                      {col.header}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {Array.from({ length: loadingSkeletonRows }).map((_, i) => (
                  <tr key={i} className="border-b">
                    {columns.map((col) => (
                      <td key={col.id} className="p-2.5 px-3">
                        <Skeleton className="h-4 w-full" />
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        }
      >
        <div
          ref={scrollRef}
          onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)}
          className={cn(
            "relative overflow-auto scrollbar-thin [scrollbar-gutter:stable]",
            shouldVirtualize && "border rounded-lg border-border bg-background",
          )}
          style={shouldVirtualize ? { maxHeight } : undefined}
        >
          <table className={cn("w-full caption-bottom text-sm border-collapse", tableClassName)}>
            <TableHeader className="sticky top-0 z-10 bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
              <tr className="border-b border-border">
                {columns.map((col) => {
                  const sortable = col.sortValue != null;
                  const active = sort?.column === col.id;
                  const dir = active ? sort.dir : undefined;
                  return (
                    <TableHead
                      key={col.id}
                      className={cn(
                        "p-0",
                        col.headerClassName,
                        col.align === "right" && "text-right",
                        col.align === "center" && "text-center",
                        col.hidden,
                      )}
                      style={col.width != null ? { width: col.width } : undefined}
                      aria-sort={active ? (dir === "asc" ? "ascending" : "descending") : undefined}
                    >
                      {sortable ? (
                        <button
                          type="button"
                          onClick={() => handleSort(col.id)}
                          className={cn(
                            "inline-flex h-10 w-full select-none items-center gap-1 px-3 outline-none hover:bg-muted/70 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                            col.align === "right" && "flex-row-reverse justify-start",
                            col.align === "center" && "justify-center",
                          )}
                        >
                          {col.header}
                          {active ? (
                            dir === "asc" ? <ArrowUp className="size-3.5" /> : <ArrowDown className="size-3.5" />
                          ) : (
                            <ChevronsUpDown className="size-3.5 opacity-40" />
                          )}
                        </button>
                      ) : (
                        <span
                          className={cn(
                            "inline-flex h-10 w-full items-center px-3",
                            col.align === "right" && "justify-end",
                            col.align === "center" && "justify-center",
                          )}
                        >
                          {col.header}
                        </span>
                      )}
                    </TableHead>
                  );
                })}
              </tr>
            </TableHeader>
            <TableBody>
              {shouldVirtualize && spacerTop > 0 && (
                <tr aria-hidden="true" className="pointer-events-none">
                  <td colSpan={columns.length} style={{ height: spacerTop }} />
                </tr>
              )}
              {visibleRows.map((row, i) => {
                const index = shouldVirtualize ? range.start + i : i;
                return (
                  <TableRow
                    key={rowKey(row, index)}
                    data-row-key={rowKey(row, index)}
                    className={cn(
                      onRowClick && "cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring",
                    )}
                    style={shouldVirtualize ? { height: effRowHeight } : undefined}
                    onClick={onRowClick ? () => onRowClick(row) : undefined}
                    tabIndex={onRowClick ? 0 : undefined}
                    aria-label={onRowClick ? (rowAriaLabel?.(row, index) ?? t("common.open_row", { n: index + 1 })) : undefined}
                    onKeyDown={
                      onRowClick
                        ? (e) => {
                            if (e.key === "Enter" || e.key === " ") {
                              e.preventDefault();
                              onRowClick(row);
                            }
                          }
                        : undefined
                    }
                  >
                    {columns.map((col) => (
                      <TableCell
                        key={col.id}
                        className={cn(
                          "align-middle text-sm whitespace-nowrap p-2.5",
                          col.cellClassName,
                          col.hidden,
                          col.align === "right" && "text-right",
                          col.align === "center" && "text-center",
                        )}
                      >
                        {col.cell(row)}
                      </TableCell>
                    ))}
                  </TableRow>
                );
              })}
              {shouldVirtualize && spacerBottom > 0 && (
                <tr aria-hidden="true" className="pointer-events-none">
                  <td colSpan={columns.length} style={{ height: spacerBottom }} />
                </tr>
              )}
            </TableBody>
          </table>
        </div>
      </DataState>

      {pagination && (
        <div className="mt-2" data-testid="data-table-pagination">
          <Pagination page={pagination.page} pageSize={pagination.pageSize} total={pagination.total} onPageChange={pagination.onPageChange} />
        </div>
      )}
    </div>
  );
}
