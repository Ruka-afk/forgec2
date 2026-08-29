"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { paths } from "@/lib/api-paths";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PageContainer } from "@/components/ui/page-container";
import { formatTime } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { DataTable } from "@/components/ui/data-table";
import type { DataTableColumn } from "@/components/ui/data-table";
import { toast } from "sonner";
import { BadgeInfo, Lock, RotateCcw, RotateCw } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface Token {
  id: string;
  agent_id: string;
  domain?: string;
  username?: string;
  integrity?: string;
  source?: string;
  pid?: number;
  process_name?: string;
  active?: boolean;
  notes?: string;
  created_at?: string;
}

interface Agent {
  id: string;
  hostname: string;
}

export default function TokensPage() {
  const { t } = useI18n();
  const [integrityFilter, setIntegrityFilter] = useState("");
  const [sourceFilter, setSourceFilter] = useState("");

  const { data, loading, refresh } = useApiResource<{ tokens: Token[]; agents: Agent[] }>({
    fetcher: async () => {
      const [tokenData, agentData] = await Promise.all([
        api.get(paths.tokens.list),
        api.get(paths.agents.list("page=1&pageSize=200")),
      ]) as Record<string, unknown>[];
      return {
        tokens: (tokenData.tokens || tokenData.data || tokenData || []) as Token[],
        agents: (agentData.agents || agentData.data || agentData || []) as Agent[],
      };
    },
    errorMessage: t("tokens.load_failed"),
  });
  const tokens = data?.tokens ?? [];
  const agents = data?.agents ?? [];

  const agentMap: Record<string, string> = {};
  agents.forEach((a) => { agentMap[a.id] = a.hostname || a.id?.substring(0, 8) || ""; });

  const filtered = tokens.filter((t) => {
    const integ = t.integrity || "Medium";
    const src = t.source || "";
    if (integrityFilter && integ !== integrityFilter) return false;
    if (sourceFilter && src !== sourceFilter) return false;
    return true;
  });

  const handleRevert = async (token: Token) => {
    try {
      if (!token.agent_id) {
        toast.error(t("tokens.revert_failed"));
        return;
      }
      await api.post(paths.agents.tokenRevert(token.agent_id));
      refresh();
    } catch { toast.error(t("tokens.revert_failed")); }
  };

  const integrityLabel = (integrity: string): string => {
    const map: Record<string, string> = {
      System: t("tokens.integrity_system"),
      High: t("tokens.integrity_high"),
      Medium: t("tokens.integrity_medium"),
      Low: t("tokens.integrity_low"),
    };
    return map[integrity] || integrity;
  };

  const getIntegrityVariant = (integrity: string): "destructive" | "warning" | "default" | "secondary" => {
    const map: Record<string, "destructive" | "warning" | "default" | "secondary"> = {
      System: "destructive",
      High: "warning",
      Medium: "default",
      Low: "secondary",
    };
    return map[integrity] || "default";
  };

  const getSourceVariant = (source: string): "warning" | "success" | "secondary" => {
    const map: Record<string, "warning" | "success" | "secondary"> = {
      steal: "warning",
      make_token: "success",
    };
    return map[source] || "secondary";
  };

  const agentHostname = (token: Token): string =>
    agentMap[token.agent_id] || token.agent_id?.substring(0, 8) || "-";

  const columns: DataTableColumn<Token>[] = [
    {
      id: "agent",
      header: t("tokens.col_agent"),
      sortValue: agentHostname,
      cell: (token) => <span className="text-xs font-mono text-primary font-medium">{agentHostname(token)}</span>,
    },
    {
      id: "user",
      header: t("tokens.col_user"),
      sortValue: (token) => `${token.domain || ""}\\${token.username || ""}`,
      cell: (token) => <span className="font-semibold text-sm">{token.domain || ""}\{token.username || ""}</span>,
    },
    {
      id: "integrity",
      header: t("tokens.col_integrity"),
      sortValue: (token) => token.integrity || "Medium",
      cell: (token) => <Badge variant={getIntegrityVariant(token.integrity || "Medium")}>{token.integrity || "Medium"}</Badge>,
    },
    {
      id: "source",
      header: t("tokens.col_source"),
      sortValue: (token) => token.source || "steal",
      cell: (token) => <Badge variant={getSourceVariant(token.source || "steal")}>{token.source || "steal"}</Badge>,
    },
    {
      id: "process",
      header: t("tokens.col_process"),
      cell: (token) => (
        <span className="text-xs font-mono text-muted-foreground">
          {token.pid ? `[${token.pid}] ` : ""}{token.process_name || ""}
        </span>
      ),
    },
    {
      id: "status",
      header: t("tokens.col_status"),
      cell: (token) =>
        token.active ? (
          <Badge variant="warning" className="gap-1.5 text-xs">
            <span className="size-2 bg-warning rounded-full animate-pulse" />{t("tokens.active")}
          </Badge>
        ) : (
          <Badge variant="secondary" className="text-xs">{t("tokens.inactive")}</Badge>
        ),
    },
    {
      id: "time",
      header: t("tokens.col_time"),
      sortValue: (token) => token.created_at || "",
      cell: (token) => (
        <span className="text-xs font-mono text-muted-foreground">{token.created_at ? formatTime(token.created_at) : "-"}</span>
      ),
    },
    {
      id: "actions",
      header: t("tokens.col_actions"),
      align: "right",
      cell: (token) => (
        <Tooltip>
          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => handleRevert(token)} aria-label={t("tokens.revert")} />}>
            <RotateCcw className="size-4" />
          </TooltipTrigger>
          <TooltipContent>{t("tokens.revert")}</TooltipContent>
        </Tooltip>
      ),
    },
  ];

  return (
    <PageContainer
      title={t("tokens.title")} icon={<BadgeInfo className="size-4" />}
      subtitle={`${t("tokens.subtitle")} ${filtered.length} ${t("tokens.count")}`}
      actions={
        <Button onClick={() => refresh()} className="gap-2">
          <RotateCw className="size-4" /> {t("tokens.refresh")}
        </Button>
      }
    >

      <Card className="mb-4">
        <CardContent className="p-(--card-spacing)">
          <div className="flex flex-col sm:flex-row gap-3">
            <Select value={integrityFilter || "all"} onValueChange={(v) => setIntegrityFilter(v === "all" ? "" : v ?? "")}>
              <SelectTrigger className="w-full sm:w-48" aria-label={t("tokens.integrity_filter")}>
                <SelectValue placeholder={t("tokens.all_integrity")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("tokens.all_integrity")}</SelectItem>
                <SelectItem value="System">{t("tokens.integrity_system")}</SelectItem>
                <SelectItem value="High">{t("tokens.integrity_high")}</SelectItem>
                <SelectItem value="Medium">{t("tokens.integrity_medium")}</SelectItem>
                <SelectItem value="Low">{t("tokens.integrity_low")}</SelectItem>
              </SelectContent>
            </Select>
            <Select value={sourceFilter || "all"} onValueChange={(v) => setSourceFilter(v === "all" ? "" : v ?? "")}>
              <SelectTrigger className="w-full sm:w-48" aria-label={t("tokens.source_filter")}>
                <SelectValue placeholder={t("tokens.all_sources")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t("tokens.all_sources")}</SelectItem>
                <SelectItem value="steal">steal</SelectItem>
                <SelectItem value="make_token">make_token</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mb-4">
        {["System", "High", "Medium", "Low"].map((level) => {
          const count = tokens.filter((t) => (t.integrity || "Medium") === level).length;
          return (
            <Card key={level} className={`relative p-3 transition-all ${integrityFilter === level ? "ring-2 ring-primary" : ""}`}>
              <Button
                type="button"
                variant="ghost"
                onClick={() => setIntegrityFilter(integrityFilter === level ? "" : level)}
                aria-pressed={integrityFilter === level}
                aria-label={integrityLabel(level)}
                className="absolute inset-0 z-10 h-auto rounded-lg"
              />
              <div className="text-xs text-muted-foreground">{integrityLabel(level)}</div>
              <div className="text-lg font-bold">{count}</div>
            </Card>
          );
        })}
      </div>

      <Card className="overflow-hidden">
        <DataTable<Token>
          data={filtered}
          loading={loading}
          columns={columns}
          emptyTitle={t("tokens.empty_title")}
          emptyMessage={t("tokens.empty_message")}
          emptyIcon={Lock}
          rowKey={(token, i) => token.id || `${token.agent_id}-${i}`}
        />
      </Card>
    </PageContainer>
  );
}

