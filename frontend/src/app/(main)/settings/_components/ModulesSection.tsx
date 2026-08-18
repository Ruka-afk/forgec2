"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentListCached } from "@/lib/agents";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { CardHeaderRow } from "@/components/ui/card-header-row";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { formatTime, formatSize } from "@/lib/utils";
import { FileCode, Rocket, Trash2, Upload } from "lucide-react";

interface ModuleInfo {
  name: string;
  size: number;
  updated_at: string;
}

interface AgentOption {
  id: string;
  hostname?: string;
  username?: string;
  status?: string;
  os?: string;
}

export default function ModulesSection() {
  const { t } = useI18n();
  const [modules, setModules] = useState<ModuleInfo[]>([]);
  const [agents, setAgents] = useState<AgentOption[]>([]);
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [deploying, setDeploying] = useState(false);
  const [hint, setHint] = useState("");
  const [deployModule, setDeployModule] = useState("");
  const [deployAgent, setDeployAgent] = useState("");
  const [deployPath, setDeployPath] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const { confirm, modal } = useConfirm();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const data = await api.get<{ modules?: ModuleInfo[]; hint?: string }>(paths.modules.list);
      setModules(data.modules || []);
      setHint(data.hint || "");
    } catch {
      setModules([]);
      toast.error(t("settings.modules.load_failed"));
    }
    setLoading(false);
  }, [t]);

  const loadAgents = useCallback(async () => {
    try {
      setAgents((await fetchAgentListCached()) as AgentOption[]);
    } catch {
      setAgents([]);
    }
  }, []);

  useEffect(() => {
    load();
    loadAgents();
  }, [load, loadAgents]);

  useEffect(() => {
    if (!deployModule && modules.length > 0) {
      setDeployModule(modules[0].name);
    }
  }, [modules, deployModule]);

  useEffect(() => {
    if (!deployPath && deployModule) {
      setDeployPath(`C:\\Windows\\Temp\\${deployModule}`);
    }
  }, [deployModule, deployPath]);

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setUploading(true);
    try {
      const fd = new FormData();
      fd.append("file", file);
      await api.postFormData(paths.modules.list, fd);
      toast.success(t("settings.modules.uploaded"));
      load();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.modules.upload_failed"));
    }
    setUploading(false);
    e.target.value = "";
  };

  const handleDelete = async (name: string) => {
    if (!(await confirm({ message: t("settings.modules.delete_confirm", { name }) }))) return;
    confirmDelete(name);
  };

  const confirmDelete = async (name: string) => {
    try {
      await api.del(paths.modules.one(name));
      toast.success(t("settings.modules.deleted"));
      if (deployModule === name) setDeployModule("");
      load();
    } catch {
      toast.error(t("settings.modules.delete_failed"));
    }
  };

  const handleDeploy = async () => {
    if (!deployModule || !deployAgent) {
      toast.error(t("settings.modules.deploy_need_selection"));
      return;
    }
    setDeploying(true);
    try {
      const path = deployPath.trim() || `C:\\Windows\\Temp\\${deployModule}`;
      const res = await api.postJson<{ success?: boolean; task_id?: number }>(
        paths.agents.modulesDeploy(deployAgent),
        { name: deployModule, path }
      );
      const taskId = res?.task_id;
      toast.success(
        taskId
          ? t("settings.modules.deployed_task").replace("{id}", String(taskId))
          : t("settings.modules.deployed")
      );
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("settings.modules.deploy_failed"));
    }
    setDeploying(false);
  };

  const agentLabel = (a: AgentOption) => {
    const host = a.hostname || a.id.slice(0, 8);
    const user = a.username ? `\\${a.username}` : "";
    const st = a.status ? ` [${a.status}]` : "";
    return `${host}${user}${st}`;
  };

  return (
    <Card className="overflow-hidden">
      <CardHeaderRow icon={FileCode} tone="violet" title={t("settings.modules.title")} description={t("settings.modules.subtitle")} />
      <div className="p-(--card-spacing) space-y-4">
        <p className="text-xs text-muted-foreground">{hint || t("settings.modules.hint")}</p>
        {(() => {
          const hasMimi = modules.some((m) => /invoke-mimikatz|mimikatz\.ps1/i.test(m.name));
          return (
            <div className={`text-xs rounded-lg px-3 py-2 border ${hasMimi ? "border-success/30 bg-success/10 text-success" : "border-warning/30 bg-warning/10 text-warning-foreground"}`}>
              {hasMimi ? t("settings.modules.mimikatz_ready") : t("settings.modules.mimikatz_missing")}
            </div>
          );
        })()}
        <div className="flex gap-2">
          <Input ref={fileRef} type="file" className="hidden" accept=".ps1,.exe,.dll,.bin,.zip" onChange={handleUpload} />
          <Button onClick={() => fileRef.current?.click()} disabled={uploading}>
            {uploading ? <Spinner size="xs" /> : <Upload className="w-4 h-4" />}
            {t("settings.modules.upload")}
          </Button>
          <Button variant="outline" onClick={() => { load(); loadAgents(); }}>{t("common.refresh")}</Button>
        </div>
        {loading ? (
          <div className="flex justify-center py-8"><Spinner /></div>
        ) : modules.length === 0 ? (          <EmptyState icon={FileCode} title={t("settings.modules.empty")} message={t("settings.modules.empty_desc")} />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("common.name")}</TableHead>
                <TableHead>{t("settings.modules.size")}</TableHead>
                <TableHead>{t("settings.modules.updated")}</TableHead>
                <TableHead className="w-16" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {modules.map((m) => (
                <TableRow key={m.name}>
                  <TableCell className="font-mono text-xs">{m.name}</TableCell>
                  <TableCell className="text-xs">{formatSize(m.size)}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {m.updated_at ? formatTime(m.updated_at) : "-"}
                  </TableCell>
                  <TableCell>
                    <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(m.name)} aria-label={t("common.delete")}>
                      <Trash2 className="w-3.5 h-3.5 text-destructive" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {modal}

        {modules.length > 0 && (
          <div className="rounded-lg border border-border/60 bg-muted/20 p-4 space-y-3">
            <div className="flex items-center gap-2">
              <Rocket className="w-4 h-4 text-chart-6" />
              <h3 className="text-sm font-medium">{t("settings.modules.deploy_title")}</h3>
            </div>
            <p className="text-xs text-muted-foreground">{t("settings.modules.deploy_hint")}</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label className="text-xs">{t("settings.modules.deploy_module")}</Label>
                <Select value={deployModule} onValueChange={(v) => { setDeployModule(v || ""); setDeployPath(v ? `C:\\Windows\\Temp\\${v}` : ""); }}>
                  <SelectTrigger className="text-xs">
                    <SelectValue placeholder={t("settings.modules.deploy_module")} />
                  </SelectTrigger>
                  <SelectContent>
                    {modules.map((m) => (
                      <SelectItem key={m.name} value={m.name}>{m.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">{t("settings.modules.deploy_agent")}</Label>
                <Select value={deployAgent} onValueChange={(v) => setDeployAgent(v || "")}>
                  <SelectTrigger className="text-xs">
                    <SelectValue placeholder={t("settings.modules.deploy_agent_placeholder")} />
                  </SelectTrigger>
                  <SelectContent>
                    {agents.length === 0 ? (
                      <SelectItem value="__none" disabled>{t("settings.modules.no_agents")}</SelectItem>
                    ) : (
                      agents.map((a) => (
                        <SelectItem key={a.id} value={a.id}>{agentLabel(a)}</SelectItem>
                      ))
                    )}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5 sm:col-span-2">
                <Label className="text-xs">{t("settings.modules.deploy_path")}</Label>
                <Input
                  className="text-xs font-mono"
                  value={deployPath}
                  onChange={(e) => setDeployPath(e.target.value)}
                  placeholder={`C:\\Windows\\Temp\\${deployModule || "module.ps1"}`}
                />
              </div>
            </div>
            <Button onClick={handleDeploy} disabled={deploying || !deployModule || !deployAgent || deployAgent === "__none"}>
              {deploying ? <Spinner size="xs" /> : <Rocket className="w-4 h-4" />}
              {t("settings.modules.deploy")}
            </Button>
          </div>
        )}
      </div>
    </Card>
  );
}
