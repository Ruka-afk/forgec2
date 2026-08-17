"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { PageContainer } from "@/components/ui/page-container";
import { Spinner } from "@/components/ui/spinner";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { useTaskResult } from "@/lib/hooks/useTaskResult";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Box, Check, Cpu, PersonStanding, Search } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import type { Agent } from "@/types/agent";

export default function ContainerPage() {
  const { t } = useI18n();
  const { agents } = useAgentList();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [escapeMethod, setEscapeMethod] = useState("generic");
  const [activeTab, setActiveTab] = useState("detect");
  const [loading, setLoading] = useState(false);
  const [dispatchMsg, setDispatchMsg] = useState<string | null>(null);
  const taskPoll = useTaskResult(selectedAgent);

  const getAgentId = (a: Agent) => a.id || "";
  const getHostname = (a: Agent) => a.hostname || "";
  const getIP = (a: Agent) => a.ip || "";

  const handleDetect = async () => {
    if (!selectedAgent) return;
    setLoading(true);
    setDispatchMsg(null);
    taskPoll.reset();
    try {
      const data = await api.postJson<{ task_id?: string | number }>(paths.agents.cmd(selectedAgent, "container_detect"), {});
      const tid = data.task_id;
      setDispatchMsg(t("container.task_dispatched", { id: String(tid ?? "") }));
      if (tid != null) taskPoll.start(tid);
    } catch (e) {
      setDispatchMsg(`${t("common.error")}: ${e instanceof Error ? e.message : String(e)}`);
    }
    setLoading(false);
  };

  const handleEscape = async () => {
    if (!selectedAgent) return;
    setLoading(true);
    setDispatchMsg(null);
    taskPoll.reset();
    let endpoint = "container_escape";
    if (escapeMethod === "docker") endpoint = "container_docker";
    else if (escapeMethod === "k8s") endpoint = "container_k8s";
    try {
      const data = await api.postJson<{ task_id?: string | number }>(paths.agents.cmd(selectedAgent, endpoint), {});
      const tid = data.task_id;
      setDispatchMsg(t("container.task_dispatched", { id: String(tid ?? "") }));
      if (tid != null) taskPoll.start(tid);
    } catch (e) {
      setDispatchMsg(`${t("common.error")}: ${e instanceof Error ? e.message : String(e)}`);
    }
    setLoading(false);
  };

  const statusLabel = () => {
    switch (taskPoll.status) {
      case "pending": return t("container.poll_pending");
      case "running": return t("container.poll_running");
      case "completed": return t("container.poll_completed");
      case "failed": return t("container.poll_failed");
      case "timeout": return t("container.poll_timeout");
      default: return "";
    }
  };

  return (
    <PageContainer title={t("container.title")} subtitle={t("container.subtitle")} contentClassName="space-y-6">
      <Card className="p-3 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("container.experimental_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("container.experimental_desc")}</div>
      </Card>

      <Card className="p-4 sm:p-5">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={Box} color="info" size="xl" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("container.target_agent")}</div>
            <div className="text-xs text-muted-foreground">{t("container.target_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("container.agent")}</span>
            <Select value={selectedAgent} onValueChange={(v) => { setSelectedAgent(v ?? ""); taskPoll.reset(); setDispatchMsg(null); }}>
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

      <Card className="p-4 sm:p-5">
        <div className="flex items-center gap-x-3 mb-5">
          <IconBadge icon={Cpu} color="primary" size="xl" />
          <div>
            <div className="text-sm font-semibold text-foreground">{t("container.operations")}</div>
            <div className="text-xs text-muted-foreground">{t("container.operations_desc")}</div>
          </div>
        </div>
        <Tabs value={activeTab} onValueChange={setActiveTab}>
          <TabsList>
            <TabsTrigger value="detect"><Search className="w-3.5 h-3.5" />{t("container.detection")}</TabsTrigger>
            <TabsTrigger value="escape"><PersonStanding className="w-3.5 h-3.5" />{t("container.escape")}</TabsTrigger>
          </TabsList>
          <TabsContent value="detect" className="space-y-3 mt-4">
            <p className="text-xs text-muted-foreground">{t("container.detect_desc")}</p>
            <Button onClick={handleDetect} disabled={loading || !selectedAgent || taskPoll.polling}>
              {loading || taskPoll.polling ? <Spinner size="xs" /> : <Check className="w-4 h-4" />}
              {loading || taskPoll.polling ? t("container.dispatching") : t("container.detect_btn")}
            </Button>
          </TabsContent>
          <TabsContent value="escape" className="space-y-3 mt-4">
            <p className="text-xs text-muted-foreground">{t("container.escape_desc")}</p>
            <Select value={escapeMethod} onValueChange={(v) => setEscapeMethod(v ?? "generic")}>
              <SelectTrigger className="w-full max-w-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="generic">Generic</SelectItem>
                <SelectItem value="docker">Docker</SelectItem>
                <SelectItem value="k8s">Kubernetes</SelectItem>
              </SelectContent>
            </Select>
            <Button onClick={handleEscape} disabled={loading || !selectedAgent || taskPoll.polling}>
              {loading || taskPoll.polling ? <Spinner size="xs" /> : <PersonStanding className="w-4 h-4" />}
              {loading || taskPoll.polling ? t("container.dispatching") : t("container.escape_btn")}
            </Button>
          </TabsContent>
        </Tabs>
      </Card>

      {(dispatchMsg || taskPoll.status !== "idle") && (
        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-2">
            <div className="text-sm font-semibold text-foreground">{t("container.result")}</div>
            {taskPoll.status !== "idle" && (
              <Badge variant={taskPoll.status === "completed" ? "success" : taskPoll.status === "failed" || taskPoll.status === "timeout" ? "destructive" : "secondary"}>
                {statusLabel()}
              </Badge>
            )}
          </div>
          {dispatchMsg && <pre className="text-xs text-muted-foreground whitespace-pre-wrap mb-2 font-mono">{dispatchMsg}</pre>}
          {taskPoll.polling && (
            <div className="flex items-center gap-2 text-xs text-muted-foreground py-2">
              <Spinner size="xs" /> {t("container.poll_waiting")}
            </div>
          )}
          {taskPoll.result && (
            <pre className="text-xs bg-muted/50 p-3 rounded-lg whitespace-pre-wrap max-h-96 overflow-auto font-mono">{taskPoll.result}</pre>
          )}
        </Card>
      )}
    </PageContainer>
  );
}
