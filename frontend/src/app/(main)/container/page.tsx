"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { PageHeader, Spinner } from "@/components/UI";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Box, Check, Cpu, PersonStanding, Search } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { Agent } from "@/types/agent";


export default function ContainerPage() {
  const { t } = useI18n();
  const { agents, refresh: refreshAgents } = useAgentList();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [escapeMethod, setEscapeMethod] = useState("generic");
  const [activeTab, setActiveTab] = useState("detect");
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<string | null>(null);

  useEffect(() => { refreshAgents(); }, [refreshAgents]);

  const getAgentId = (a: Agent) => a.id || "";
  const getHostname = (a: Agent) => a.hostname || "";
  const getIP = (a: Agent) => a.ip || "";

  const handleDetect = async () => {
    if (!selectedAgent) return;
    setLoading(true);
    setResult(null);
    try {
      const data = await api.postJson(`/agents/${selectedAgent}/container_detect`, {});
      setResult(`Task dispatched successfully. Task ID: ${data.task_id}\n\nCheck the Tasks page for agent results.`);
    } catch (e) {
      setResult(`Error: ${e instanceof Error ? e.message : String(e)}`);
    }
    setLoading(false);
  };

  const handleEscape = async () => {
    if (!selectedAgent) return;
    setLoading(true);
    setResult(null);
    let endpoint = "/container_escape";
    if (escapeMethod === "docker") endpoint = "/container_docker";
    else if (escapeMethod === "k8s") endpoint = "/container_k8s";
    try {
      const data = await api.postJson(`/agents/${selectedAgent}${endpoint}`, {});
      setResult(`Task dispatched successfully. Task ID: ${data.task_id}\n\nCheck the Tasks page for agent results.`);
    } catch (e) {
      setResult(`Error: ${e instanceof Error ? e.message : String(e)}`);
    }
    setLoading(false);
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("container.title")} subtitle={t("container.subtitle")} />


      {/* Agent Selector */}
      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-cyan-100 dark:bg-cyan-900/30 rounded-xl flex items-center justify-center">
            <Box className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("container.target_agent")}</div>
            <div className="text-xs text-muted-foreground">{t("container.target_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("container.agent")}</span>
            <Select value={selectedAgent} onValueChange={(v) => setSelectedAgent(v ?? "")}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("container.select_agent")} />
              </SelectTrigger>
              <SelectContent>
                {agents.map(a => {
                  const id = getAgentId(a);
                  return <SelectItem key={id} value={String(id)}>{getHostname(a)} ({getIP(a)})</SelectItem>;
                })}
              </SelectContent>
            </Select>
          </div>
        </div>
      </Card>

      {/* Tabs: Detect | Escape */}
      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-x-3 mb-5">
          <div className="w-10 h-10 bg-primary/10 rounded-xl flex items-center justify-center">
            <Cpu className="w-4 h-4" />
          </div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("container.operations")}</div>
            <div className="text-xs text-muted-foreground">{t("container.operations_desc")}</div>
          </div>
        </div>

        {/* Tab Buttons */}
        <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v ?? "detect")}>
          <TabsList className="mb-6">
            <TabsTrigger value="detect" className="gap-2">
              <Search className="w-4 h-4" />{t("container.detection")}
            </TabsTrigger>
            <TabsTrigger value="escape" className="gap-2">
              <PersonStanding className="w-4 h-4" />{t("container.escape")}
            </TabsTrigger>
          </TabsList>

        {/* Detection Tab */}
        <TabsContent value="detect">
          <div>
            <p className="text-sm text-muted-foreground mb-4">
              {t("container.detect_desc")}
            </p>
            <Button onClick={handleDetect} disabled={loading || !selectedAgent}>
              {loading ? <Spinner size="xs" /> : <Box className="w-4 h-4" />}
              <span>{loading ? t("container.dispatching") : t("container.detect_btn")}</span>
            </Button>
          </div>
        </TabsContent>

        {/* Escape Tab */}
        <TabsContent value="escape">
          <div>
            <p className="text-sm text-muted-foreground mb-4">
              {t("container.escape_desc")}
            </p>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("container.escape_method")}</span>
                <Select value={escapeMethod} onValueChange={(v) => setEscapeMethod(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="generic">Generic Escape</SelectItem>
                    <SelectItem value="docker">Docker Socket Escape</SelectItem>
                    <SelectItem value="k8s">K8s Service Account Abuse</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <Button variant="destructive" onClick={handleEscape} disabled={loading || !selectedAgent}>
              {loading ? <Spinner size="xs" /> : <PersonStanding className="w-4 h-4" />}
              <span>{loading ? t("container.dispatching") : t("container.escape_btn")}</span>
            </Button>
          </div>
        </TabsContent>
        </Tabs>
      </Card>

      {/* Results Viewer */}
      {result && (
        <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
          <div className="flex items-center gap-x-3 mb-4">
            <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/30 rounded-xl flex items-center justify-center">
              <Check className="w-4 h-4" />
            </div>
            <div>
              <div className="text-sm font-semibold text-foreground">{t("container.result")}</div>
              <div className="text-xs text-muted-foreground">{t("container.result_desc")}</div>
            </div>
          </div>
          <pre className="bg-card text-foreground p-4 rounded-xl text-xs font-mono overflow-x-auto whitespace-pre-wrap">
            {result}
          </pre>
        </Card>
      )}
    </div>
  );
}

