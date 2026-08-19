"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { paths } from "@/lib/api-paths";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/ui/page-container";
import { formatTime } from "@/lib/utils";
import { Card, CardContent } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
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

  return (
    <PageContainer
      title={t("tokens.title")} icon={<BadgeInfo className="w-4 h-4" />}
      subtitle={`${t("tokens.subtitle")} ${filtered.length} ${t("tokens.count")}`}
      actions={
        <Button onClick={() => refresh()} className="gap-2">
          <RotateCw className="w-4 h-4" /> {t("tokens.refresh")}
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
                aria-label={level}
                className="absolute inset-0 z-10 h-auto rounded-lg"
              />
              <div className="text-xs text-muted-foreground">{level}</div>
              <div className="text-lg font-bold">{count}</div>
            </Card>
          );
        })}
      </div>

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("tokens.col_agent")}</TableHead>
                <TableHead>{t("tokens.col_user")}</TableHead>
                <TableHead>{t("tokens.col_integrity")}</TableHead>
                <TableHead>{t("tokens.col_source")}</TableHead>
                <TableHead>{t("tokens.col_process")}</TableHead>
                <TableHead>{t("tokens.col_status")}</TableHead>
                <TableHead>{t("tokens.col_time")}</TableHead>
                <TableHead>{t("tokens.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {!loading && filtered.map((token) => {
                const tid = token.id || "";
                const domain = token.domain || "";
                const username = token.username || "";
                const integrity = token.integrity || "Medium";
                const source = token.source || "steal";
                const pid = token.pid;
                const procName = token.process_name;
                const active = token.active;
                const createdAt = token.created_at || "";
                const agentHostname = agentMap[token.agent_id] || token.agent_id?.substring(0, 8) || "-";
                return (
                  <TableRow key={tid}>
                    <TableCell><span className="text-xs font-mono text-primary font-medium">{agentHostname}</span></TableCell>
                    <TableCell><span className="font-semibold text-sm">{domain}\{username}</span></TableCell>
                    <TableCell><Badge variant={getIntegrityVariant(integrity)}>{integrity}</Badge></TableCell>
                    <TableCell><Badge variant={getSourceVariant(source)}>{source}</Badge></TableCell>
                    <TableCell className="text-xs font-mono text-muted-foreground">{pid ? `[${pid}]` : ""} {procName || ""}</TableCell>
                    <TableCell>
                      {active ? (
                        <Badge variant="warning" className="gap-1.5 text-xs">
                          <span className="w-2 h-2 bg-warning rounded-full animate-pulse"></span>{t("tokens.active")}
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="text-xs">{t("tokens.inactive")}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs font-mono text-muted-foreground">{createdAt ? formatTime(createdAt) : "-"}</TableCell>
                    <TableCell>
                      <Tooltip>
                        <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => handleRevert(token)} aria-label={t("tokens.revert")} />}>
                        <RotateCcw className="w-4 h-4" />
                        </TooltipTrigger>
                        <TooltipContent>{t("tokens.revert")}</TooltipContent>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                );
              })}
              {loading && Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  <TableCell colSpan={8}><Skeleton className="h-8 w-full" /></TableCell>
                </TableRow>
              ))}
              {!loading && filtered.length === 0 && (
                <TableRow>
                  <TableCell colSpan={8} className="py-20 text-center text-muted-foreground">
                    <EmptyState icon={Lock} title={t("tokens.empty_title")} message={t("tokens.empty_message")} />
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </Card>
    </PageContainer>
  );
}

