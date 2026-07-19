"use client";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { useI18n } from "@/lib/i18n";
import type { Tag } from "./types";
import { Apple, Columns, Monitor, Search, Terminal, Wifi, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";

interface AgentFiltersProps {
  searchInput: string; setSearchInput: (v: string) => void;
  statusFilter: string; setStatusFilter: (v: string) => void;
  osFilter: string; setOsFilter: (v: string) => void;
  tagFilter: string; setTagFilter: (v: string) => void;
  linkedFilter: string; setLinkedFilter: (v: string) => void;
  allTags: Tag[];
  visibleCols: Record<string, boolean>; setVisibleCols: (v: Record<string, boolean> | ((prev: Record<string, boolean>) => Record<string, boolean>)) => void;
  onlineCount: number; staleCount: number; offlineCount: number;
  windowsCount: number; linuxCount: number; darwinCount: number;
}

export function AgentFilters({
  searchInput, setSearchInput,
  statusFilter, setStatusFilter,
  osFilter, setOsFilter,
  tagFilter, setTagFilter,
  linkedFilter, setLinkedFilter,
  allTags,
  visibleCols, setVisibleCols,
  onlineCount, staleCount, offlineCount,
  windowsCount, linuxCount, darwinCount,
}: AgentFiltersProps) {
  const { t } = useI18n();
  return (
    <Card className="p-3 sm:p-4 mb-4 gap-0">
      <div className="flex flex-col sm:flex-row gap-3">
        <div className="flex gap-2 flex-wrap">
          <div className="relative flex-1 sm:max-w-xs">
            <Search className="w-4 h-4" />
            <Input
              id="agent-search"
              type="text"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder={t("agents.search_placeholder_short")}
              className="h-11 pl-9"
            />
            {searchInput && (
              <Button variant="ghost" size="icon-xs" onClick={() => setSearchInput("")} className="absolute right-1 top-1/2 -translate-y-1/2" aria-label="Clear search">
                <X className="w-4 h-4" />
              </Button>
            )}
          </div>
          <Select value={statusFilter} onValueChange={(v) => setStatusFilter(v ?? "")}>
            <SelectTrigger aria-label="Status filter" className="flex-1 sm:flex-none h-11">
              <SelectValue placeholder={t("agents.all_status")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("agents.all_status")}</SelectItem>
              <SelectItem value="online">{t("agents.online")}</SelectItem>
              <SelectItem value="stale">{t("agents.stale")}</SelectItem>
              <SelectItem value="offline">{t("agents.offline")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={osFilter} onValueChange={(v) => setOsFilter(v ?? "")}>
            <SelectTrigger aria-label="OS filter" className="flex-1 sm:flex-none h-11">
              <SelectValue placeholder={t("agents.all_os")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("agents.all_os")}</SelectItem>
              <SelectItem value="windows">{t("agents.windows")}</SelectItem>
              <SelectItem value="linux">{t("agents.linux")}</SelectItem>
              <SelectItem value="darwin">{t("agents.macos")}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={tagFilter} onValueChange={(v) => setTagFilter(v ?? "")}>
            <SelectTrigger aria-label="Tag filter" className="flex-1 sm:flex-none h-11">
              <SelectValue placeholder={t("agents.all_tags")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("agents.all_tags")}</SelectItem>
              {allTags.map((tag) => (
                <SelectItem key={tag.id} value={tag.id}>{tag.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={linkedFilter} onValueChange={(v) => setLinkedFilter(v ?? "")}>
            <SelectTrigger aria-label="Link filter" className="flex-1 sm:flex-none h-11">
              <SelectValue placeholder={t("agents.all_links")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("agents.all_links")}</SelectItem>
              <SelectItem value="direct">{t("agents.direct_c2")}</SelectItem>
              <SelectItem value="chained">{t("agents.p2p_chained")}</SelectItem>
            </SelectContent>
          </Select>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="secondary" size="lg" className="flex-1 sm:flex-none gap-2" title="Toggle columns" />}>
              <Columns className="w-4 h-4" />
              <span className="hidden sm:inline">{t("agents.columns")}</span>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-[160px]">
              {Object.entries(visibleCols).map(([key, vis]) => (
                <DropdownMenuItem key={key} onClick={() => setVisibleCols((p) => ({ ...p, [key]: !vis }))} className="capitalize">
                  <Checkbox
                    checked={vis}
                    onCheckedChange={(checked) => setVisibleCols((p) => ({ ...p, [key]: !!checked }))}
                  />
                  <span>{key.replace("_", " ")}</span>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          <div className="hidden sm:flex items-center gap-1.5 ml-auto text-[11px]">
            {onlineCount > 0 && <Badge variant="secondary" className="gap-1 text-emerald-600 dark:text-emerald-400"><Wifi className="w-4 h-4" />{onlineCount}</Badge>}
            {windowsCount > 0 && <Badge variant="secondary" className="gap-1"><Monitor className="w-4 h-4" />{windowsCount}</Badge>}
            {linuxCount > 0 && <Badge variant="secondary" className="gap-1"><Terminal className="w-4 h-4" />{linuxCount}</Badge>}
            {darwinCount > 0 && <Badge variant="secondary" className="gap-1"><Apple className="w-4 h-4" />{darwinCount}</Badge>}
          </div>
        </div>
      </div>
    </Card>
  );
}
