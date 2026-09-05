"use client";

import { memo, type ReactNode, type RefObject } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { EmptyState } from "@/components/ui/empty-state";
import { Pagination } from "@/components/ui/pagination";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ArrowLeftRight, Plus, Radio } from "lucide-react";
import { AgentRow } from "./AgentRow";
import type { Beacon } from "./types";
import type { AgentMenuPoint } from "./agent-menu-actions";
import type { AgentSortKey } from "./useAgentFilters";
import type { AgentTag } from "./useAgentData";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface AgentsTableProps {
  t: TKey;
  beacons: Beacon[];
  visibleBeacons: Beacon[];
  selected: Set<string>;
  toggleSelect: (id: string, checked: boolean) => void;
  toggleSelectAll: (v: boolean) => void;
  sortKey: AgentSortKey;
  sortDir: "asc" | "desc";
  toggleSort: (key: AgentSortKey) => void;
  sortIcon: (field: AgentSortKey) => ReactNode;
  handleSortKeyDown: (field: AgentSortKey) => (e: React.KeyboardEvent) => void;
  visibleCols: Record<string, boolean>;
  loading: boolean;
  emptyColSpan: number;
  agentVirtualized: boolean;
  agentScrollRef: RefObject<HTMLDivElement | null>;
  onAgentScroll: () => void;
  agentOffsetTop: number;
  agentTotalHeight: number;
  rowHeight: number;
  statusFilter: string;
  osFilter: string;
  page: number;
  total: number;
  setPage: (p: number) => void;
  onSelectAgent: (id: string) => void;
  onMenu: (point: AgentMenuPoint) => void;
  onQuickNav: (beacon: Beacon, view: "shell" | "files" | "screen") => void;
  onEditNotes: (beacon: Beacon) => void;
  taskCountMap: Record<string, number>;
  agentLocks: Record<string, string>;
  operatorPresence: Record<string, string[]>;
  tagsByAgent: Record<string, AgentTag[]>;
}

/** Virtualized agents table card with sortable header + pagination. */
export default memo(function AgentsTable(props: AgentsTableProps) {
  const {
    t, beacons, visibleBeacons, selected, toggleSelect, toggleSelectAll,
    sortKey, sortDir, toggleSort, sortIcon, handleSortKeyDown,
    visibleCols, loading, emptyColSpan,
    agentVirtualized, agentScrollRef, onAgentScroll,
    agentOffsetTop, agentTotalHeight, rowHeight,
    statusFilter, osFilter, page, total, setPage,
    onSelectAgent, onMenu, onQuickNav, onEditNotes,
    taskCountMap, agentLocks, operatorPresence, tagsByAgent,
  } = props;

  const allVisibleSelected = beacons.length > 0 && beacons.every((b) => b.id && selected.has(b.id));
  const someVisibleSelected = selected.size > 0 && !allVisibleSelected;

  const sortableHead = (
    field: AgentSortKey,
    labelKey: string,
  ) => (
    <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 cursor-pointer select-none" tabIndex={0} role="columnheader" aria-sort={sortKey === field ? (sortDir === "asc" ? "ascending" : "descending") : "none"} onClick={() => toggleSort(field)} onKeyDown={handleSortKeyDown(field)}>
      {t(labelKey)} {sortIcon(field)}
    </TableHead>
  );

  return (
    <Card className="overflow-hidden shadow-sm hover:shadow-md transition-shadow duration-200">
      <div
        ref={agentScrollRef}
        onScroll={onAgentScroll}
        className={agentVirtualized ? "overflow-auto max-h-[min(70vh,720px)] scrollbar-thin" : "overflow-auto scrollbar-thin"}
      >
      <Table className="text-sm min-w-[850px]">
        <TableHeader className="sticky top-0 z-10 bg-card/95 backdrop-blur supports-[backdrop-filter]:bg-card/90 border-b border-border">
          <TableRow className="hover:bg-transparent">
            <TableHead className="text-left py-3 px-4 sm:py-3.5 sm:px-5 w-10">
              <Checkbox aria-label={t("agents.select_all")} name="input-4"
                checked={allVisibleSelected || someVisibleSelected}
                indeterminate={someVisibleSelected && !allVisibleSelected}
                onCheckedChange={(v) => toggleSelectAll(v !== false)}
              />
            </TableHead>
            {visibleCols.hostname && sortableHead("hostname", "agents.col_hostname")}
            {visibleCols.username && sortableHead("username", "agents.col_user")}
            {visibleCols.os && sortableHead("os", "agents.col_os")}
            {visibleCols.ip && sortableHead("ip", "agents.col_ip")}
            {visibleCols.last_seen && sortableHead("last_seen", "agents.col_last_seen")}
            {visibleCols.window && (
            <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_window")}</TableHead>
            )}
            {visibleCols.lock && (
            <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_lock")}</TableHead>
            )}
            {visibleCols.tasks && (
            <TableHead className="text-center py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_tasks")}</TableHead>
            )}
            {visibleCols.version && <TableHead className="text-left py-3 px-3 sm:py-3.5 sm:px-4 max-sm:hidden">{t("agents.col_version")}</TableHead>}
            {visibleCols.status && sortableHead("status", "agents.col_status")}
            <TableHead className="text-right py-3 px-4 sm:py-3.5 sm:px-5">{t("agents.col_actions")}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody className="divide-y divide-border">
          {loading && Array.from({ length: 6 }).map((_, i) => (
            <TableRow key={`skel-${i}`}>
              {Array.from({ length: emptyColSpan }).map((_, j) => (
                <TableCell key={j} className="py-3 px-3 sm:py-3.5 sm:px-4">
                  <Skeleton className="h-4 w-3/4" />
                </TableCell>
              ))}
            </TableRow>
          ))}
          {!loading && agentVirtualized && agentOffsetTop > 0 && (
            <TableRow aria-hidden className="hover:bg-transparent">
              <TableCell colSpan={emptyColSpan} style={{ height: agentOffsetTop, padding: 0, border: 0 }} />
            </TableRow>
          )}
          {!loading && visibleBeacons.map((beacon) => (
            <AgentRow
              key={beacon.id || ""}
              beacon={beacon}
              isSelected={selected.has(beacon.id || "")}
              onToggleSelect={toggleSelect}
              onInteract={onSelectAgent}
              onDetails={onSelectAgent}
              onMenu={onMenu}
              onQuickNav={onQuickNav}
              onEditNotes={onEditNotes}
              taskCount={taskCountMap[beacon.id || ""] ?? 0}
              lockUser={agentLocks[beacon.id || ""] || null}
              presenceUsers={operatorPresence[beacon.id || ""] || null}
              visibleCols={visibleCols}
              tags={tagsByAgent[beacon.id || ""] || []}
            />
          ))}
          {!loading && agentVirtualized && agentTotalHeight - agentOffsetTop - visibleBeacons.length * rowHeight > 0 && (
            <TableRow aria-hidden className="hover:bg-transparent">
              <TableCell colSpan={emptyColSpan} style={{ height: agentTotalHeight - agentOffsetTop - visibleBeacons.length * rowHeight, padding: 0, border: 0 }} />
            </TableRow>
          )}
          {!loading && beacons.length === 0 && (
            <TableRow>
              <TableCell colSpan={emptyColSpan} className="py-10">
                <EmptyState
                  icon={Radio}
                  title={t("agents.no_beacons")}
                  message={statusFilter || osFilter ? t("agents.no_beacons_filtered") : t("agents.no_beacons_hint")}
                  action={!statusFilter && !osFilter ? (
                    <Button render={<Link href="/generate" />}>
                      <Plus className="size-4" />
                      <span>{t("agents.generate_implant")}</span>
                    </Button>
                  ) : undefined}
                />
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
      </div>
      <div className="sm:hidden px-4 py-2 text-center text-xs text-muted-foreground border-t border-border bg-muted">
        <ArrowLeftRight className="size-4" /> {t("agents.swipe_hint")}
      </div>

      <Pagination page={page} pageSize={50} total={total} onPageChange={setPage} />
    </Card>
  );
});
