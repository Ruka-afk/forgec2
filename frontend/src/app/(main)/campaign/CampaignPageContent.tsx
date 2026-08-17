"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";

import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { phaseColor } from "@/lib/chart-palette";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { EmptyState } from "@/components/ui/empty-state";
import { PageContainer } from "@/components/ui/page-container";
import { Spinner } from "@/components/ui/spinner";
import { StatCard } from "@/components/ui/animated-stat-card";
import { toast } from "sonner";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { StatusBadge } from "@/components/ui/status-indicator";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { ArrowLeft, Check, Clock, ListChecks, ListTodo, Play, Plus, Trash2, Zap } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Collapsible, CollapsibleContent } from "@/components/ui/collapsible";
import {
  PHASE_ORDER,
  type Campaign,
  type CampaignMITRE,
  type CampaignStats,
  type KillChainTemplate,
  type PhaseEvent,
  type AgentStat,
  type PhaseTask,
} from "./_components/types";
import { useCampaignData } from "./_components/useCampaignData";

export default function CampaignPageContent() {
  const { t } = useI18n();
  const {
    campaigns,
    loading,
    selectedCampaign,
    setSelectedCampaign,
    campaignStats,
    creating,
    createCampaign,
    deleteCampaign,
    updateStatus,
  } = useCampaignData();

  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState("");
  const [newDesc, setNewDesc] = useState("");
  const { confirm, modal } = useConfirm();

  const handleCreate = async () => {
    const ok = await createCampaign(newName, newDesc);
    if (ok) {
      setNewName("");
      setNewDesc("");
      setShowCreate(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!(await confirm({ message: t("campaign.delete_confirm") }))) return;
    await deleteCampaign(id);
  };

  const handleStatusChange = async (id: string, status: string) => {
    await updateStatus(id, status);
  };

  const selected = campaigns.find((c) => c.id === selectedCampaign);

  if (loading) return <div className="p-8 text-center text-muted-foreground"><Spinner size="sm" /> {t("campaign.loading_stats")}</div>;

  return (
    <>
      <PageContainer title={t("campaign.title")} subtitle={t("campaign.subtitle")} contentClassName="space-y-6" actions={<>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="w-4 h-4" />{t("campaign.new")}
        </Button>
      </>}>

      <Card className="p-3 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("campaign.honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("campaign.honesty_desc")}</div>
      </Card>

      <Dialog open={showCreate} onOpenChange={setShowCreate}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("campaign.dialog_create")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <Input aria-label={t("campaign.a11y_name")} name="campaign-name-0" placeholder={t("campaign.field_name")} value={newName}
              onChange={(e) => setNewName(e.target.value)} />
            <Textarea aria-label={t("campaign.desc_optional")} name="description-optional-1" rows={2} placeholder={t("campaign.field_desc")} value={newDesc}
              onChange={(e) => setNewDesc(e.target.value)} />
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setShowCreate(false)}>{t("common.cancel")}</Button>
            <Button onClick={handleCreate} disabled={creating || !newName.trim()}>
              {creating ? t("common.saving") : t("common.create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {selectedCampaign && selected ? (
        <CampaignDetailView
          campaign={selected}
          stats={campaignStats}
          onBack={() => setSelectedCampaign(null)}
          onDelete={() => handleDelete(selected.id)}
          onStatusChange={(s) => handleStatusChange(selected.id, s)}
          formatTime={formatTime}
        />
      ) : (
        <Card className="p-0 overflow-hidden transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_name")}</TableHead>
                <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_status")}</TableHead>
                <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_agents")}</TableHead>
                <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_created")}</TableHead>
                <TableHead className="text-right py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {campaigns.length === 0 && (
                <TableRow><TableCell colSpan={5} className="text-center text-muted-foreground py-16 sm:py-20"><EmptyState icon={ListTodo} title={t("campaign.empty")} message={t("campaign.subtitle")} /></TableCell></TableRow>
              )}
              {campaigns.map((c) => (
                <TableRow key={c.id} className="cursor-pointer transition-colors"
                  tabIndex={0} role="button"
                  onClick={() => setSelectedCampaign(c.id)}
                  onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") { e.preventDefault(); setSelectedCampaign(c.id); } }}>
                  <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 font-medium text-foreground">{c.name}</TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5">
                    <StatusBadge status={c.status} label={t(`campaign.status_${c.status}`)} />
                  </TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-muted-foreground">{c.agents?.length || 0}</TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-muted-foreground text-sm">{formatTime(c.created_at)}</TableCell>
                  <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-right">
                    <Button variant="ghost" size="icon-xs" className="text-destructive"
                      onClick={(e) => { e.stopPropagation(); handleDelete(c.id); }} aria-label={t("campaign.a11y_delete")}>
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
      {modal}
    </PageContainer>
    </>
  );
}

function CampaignDetailView({
  campaign, stats, onBack, onDelete, onStatusChange, formatTime,
}: {
  campaign: Campaign;
  stats: CampaignStats | null;
  onBack: () => void;
  onDelete: () => void;
  onStatusChange: (s: string) => void;
  formatTime: (t: string) => string;
}) {
  const { t } = useI18n();
  const [mitreData, setMitreData] = useState<CampaignMITRE | null>(null);
  const [templates, setTemplates] = useState<KillChainTemplate[]>([]);
  const [selectedTemplate, setSelectedTemplate] = useState("");
  const [executing, setExecuting] = useState(false);
  const [expandedPhase, setExpandedPhase] = useState<string | null>(null);
  const [phaseTasks, setPhaseTasks] = useState<Record<string, PhaseTask[]>>({});
  const [showTimeline, setShowTimeline] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      try {
        const [mitreJson, tplJson] = await Promise.all([
          api.get<{success: boolean; data?: CampaignMITRE}>(paths.campaigns.mitre(campaign.id), { signal: controller.signal }),
          api.get<{success: boolean; data?: KillChainTemplate[]}>(paths.mitre.templates, { signal: controller.signal }),
        ]);
        if (mitreJson.success) setMitreData(mitreJson.data ?? null);
        if (tplJson.success) setTemplates(tplJson.data || []);
      } catch { toast.error(t("campaign.toast.load_failed")); }
    })();
    return () => controller.abort();
  }, [campaign.id, t]);

  const loadPhaseTasks = async (phase: string) => {
    try {
      const json = await api.get<{success: boolean; data?: PhaseTask[]}>(paths.mitre.timeline(`campaign_id=${campaign.id}`));
      if (json.success) {
        const tasks = (json.data || []).filter((e: PhaseTask) => e.phase === phase);
        setPhaseTasks((prev) => ({ ...prev, [phase]: tasks }));
      }
    } catch { toast.error(t("campaign.toast.load_failed")); }
  };

  const handlePhaseClick = (phase: string) => {
    if (expandedPhase === phase) {
      setExpandedPhase(null);
    } else {
      setExpandedPhase(phase);
      loadPhaseTasks(phase);
    }
  };

  const handleExecuteTemplate = async () => {
    if (!selectedTemplate) return;
    setExecuting(true);
    try {
          await api.postJson(paths.campaigns.killchain(campaign.id), { template: selectedTemplate });
      const mitreJson = await api.get<{success: boolean; data?: CampaignMITRE}>(paths.campaigns.mitre(campaign.id));
      if (mitreJson.success) setMitreData(mitreJson.data ?? null);
    } catch { toast.error(t("campaign.toast.load_failed")); }
    setExecuting(false);
    setSelectedTemplate("");
  };

  const phases = mitreData?.phases || [];

  return (
    <div>
      <div className="flex items-center gap-3 mb-6">
        <Button variant="ghost" size="icon" onClick={onBack} aria-label={t("campaign.a11y_back")}><ArrowLeft className="w-4 h-4" /></Button>
        <div className="flex-1">
          <h2 className="text-2xl font-semibold tracking-tight text-foreground leading-tight">{campaign.name}</h2>
          {campaign.description && <p className="text-muted-foreground text-sm">{campaign.description}</p>}
        </div>
        <div className="flex gap-2">
          <Select value={campaign.status} onValueChange={(v) => { if (v) onStatusChange(v); }}>
            <SelectTrigger className="w-32 text-sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="active">{t("campaign.status_active")}</SelectItem>
              <SelectItem value="completed">{t("campaign.status_completed")}</SelectItem>
              <SelectItem value="archived">{t("campaign.status_archived")}</SelectItem>
            </SelectContent>
          </Select>
          <Button variant="ghost" size="icon" className="text-destructive" onClick={onDelete} aria-label={t("campaign.a11y_delete")}>
            <Trash2 className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {stats ? (
        <>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-6">
            <StatCard label={t("campaign.total_agents")} value={stats.total_agents} />
            <StatCard label={t("campaign.total_tasks")} value={stats.total_tasks} />
            <StatCard label={t("campaign.completed")} value={stats.completed_tasks} color="emerald" />
            <StatCard label={t("campaign.failed")} value={stats.failed_tasks} color="destructive" />
          </div>

          {/* Kill Chain Phase Progress Bar */}
<Card className="mb-6">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-lg font-semibold">{t("campaign.kill_chain")}</h3>
              <Button variant="ghost" size="sm" onClick={() => setShowTimeline(!showTimeline)}>
                {showTimeline ? <ListChecks className="w-4 h-4 mr-1" /> : <Clock className="w-4 h-4 mr-1" />}
                {showTimeline ? t("campaign.kill_chain") : t("campaign.timeline")}
              </Button>
            </div>

            {!showTimeline ? (
              <>
                {/* Horizontal Phase Progress Bar */}
                <Collapsible open={expandedPhase !== null} onOpenChange={(open) => { if (!open) setExpandedPhase(null); }}>
                <div className="w-full overflow-x-auto pb-2">
                  <div className="flex gap-1 min-w-max">
                    {PHASE_ORDER.map((phase) => {
                      const found = phases.find((p) => p.phase === phase);
                      const isCompleted = found?.status === "completed";
                      const isPending = !found || found.task_count === 0;
                      return (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        key={phase}
                        onClick={() => handlePhaseClick(phase)}
                        className={`flex flex-col items-center gap-1 p-2 rounded-lg transition-all cursor-pointer min-w-[80px] h-auto ${
                          expandedPhase === phase ? "ring-2 ring-primary" : ""
                        } ${isCompleted ? "bg-success/15" : isPending ? "" : "bg-warning/15"}`}
                        title={phase}
                      >
                          <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold transition-colors ${
                            isCompleted ? "bg-success text-success-foreground" : isPending ? "bg-muted-foreground text-white" : "bg-warning text-warning-foreground"
                          }`}>
                            {isCompleted ? <Check className="w-4 h-4" /> : isPending ? "" : <Play className="w-4 h-4" />}
                          </div>
                          <span className={`text-(--fs-micro-sm) text-center leading-tight ${isCompleted ? "text-success font-medium" : isPending ? "text-muted-foreground" : "text-warning"}`}>
                            {phase.split(" ").slice(0, 2).join("\n")}
                          </span>
                          {found && (
                            <span className="text-(--fs-micro) text-muted-foreground">{found.task_count}</span>
                          )}
                          </Button>
                      );
                    })}
                  </div>
                </div>

                {/* Expanded phase tasks */}
                <CollapsibleContent>
                {expandedPhase && phaseTasks[expandedPhase] && phaseTasks[expandedPhase].length > 0 && (
                  <div className="mt-3 p-3 rounded-lg bg-muted/50">
                    <h4 className="text-sm font-semibold mb-2">{expandedPhase} {t("campaign.phase_tasks")}</h4>
                    <div className="space-y-1 max-h-48 overflow-y-auto">
                      {phaseTasks[expandedPhase].map((task: PhaseTask, i: number) => (
                        <div key={i} className="flex items-center justify-between text-xs py-1 px-2 rounded bg-card">
                          <span className="font-mono">{task.task_type}</span>
                          <span className="text-muted-foreground">{task.hostname || task.agent_id?.slice(0, 8)}</span>
                          <span className={`${
                            task.status === "completed" ? "text-success" :
                            task.status === "failed" ? "text-destructive" : "text-warning"
                          }`}>{t(`tasks.${task.status}`)}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                </CollapsibleContent>
                </Collapsible>
              </>
            ) : (
              <div className="relative pl-6 border-l-2 border-border">
                {(mitreData?.timeline || []).length === 0 ? (
                  <p className="text-muted-foreground text-sm py-2">{t("campaign.empty_timeline")}</p>
                ) : (
                  (mitreData?.timeline || []).map((event, i) => (
                    <div key={i} className="mb-3 relative">
                      <div className="absolute -left-[calc(1.5rem+1px)] top-1 w-3 h-3 rounded-full border-2 border-card"
                        style={{ background: phaseColor(event.phase) || "#6366f1" }} />
                      <div className="text-sm font-medium">{event.phase}</div>
                      <div className="text-xs text-muted-foreground">{formatTime(event.first_seen)}</div>
                      <div className="text-xs text-muted-foreground">{event.task_count} {t("campaign.tasks")}</div>
                    </div>
                  ))
                )}
              </div>
            )}
          </Card>

          {/* Kill Chain Template Executor */}
<Card className="mb-6">
            <h3 className="text-lg font-semibold mb-3">
              <Zap className="w-4 h-4" />
              {t("campaign.templates")}
            </h3>
            <div className="flex gap-2 items-end">
              <div className="flex-1">
                <Select value={selectedTemplate} onValueChange={(v) => setSelectedTemplate(v ?? "")}>
                  <SelectTrigger className="w-full text-sm">
                    <SelectValue placeholder={t("campaign.field_name")} />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((tpl) => (
                      <SelectItem key={tpl.name} value={tpl.name}>{tpl.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {selectedTemplate && (
                  <p className="text-xs text-muted-foreground mt-1">
                    {templates.find((t) => t.name === selectedTemplate)?.description}
                  </p>
                )}
              </div>
              <Button size="sm" disabled={!selectedTemplate || executing}
                onClick={handleExecuteTemplate}>
                {executing ? <Spinner size="xs" className="mr-1" /> : <Play className="w-4 h-4" />}
                {t("campaign.execute")}
              </Button>
            </div>
            {selectedTemplate && (
              <div className="mt-3">
                <h4 className="text-xs font-semibold text-muted-foreground mb-2">{t("campaign.steps")}</h4>
                <div className="flex flex-wrap gap-1">
                  {templates.find((t) => t.name === selectedTemplate)?.steps?.map((step, i) => (
                    <Badge key={`${step.task_type}-${i}`} variant="secondary" className="inline-flex items-center gap-1 px-2 py-1 rounded text-xs">
                      <span className="w-2 h-2 rounded-full" style={{ background: phaseColor(step.phase) || "gray" }} />
                      {step.task_type}
                    </Badge>
                  )) ?? null}
                </div>
              </div>
            )}
          </Card>

          {/* Phase Timeline (legacy) */}
<Card className="mb-6">
            <h3 className="text-lg font-semibold mb-3">{t("campaign.timeline")}</h3>
            {stats.phase_timeline.length === 0 ? (
              <p className="text-muted-foreground">{t("campaign.empty_timeline")}</p>
            ) : (
              <div className="relative pl-6 border-l-2 border-border">
                {stats.phase_timeline.map((event: PhaseEvent, i: number) => (
                  <div key={i} className="mb-4 relative">
                    <div className="absolute -left-[calc(1.5rem+1px)] top-1 w-3 h-3 rounded-full border-2 border-card"
                      style={{ background: phaseColor(event.phase) || "#6366f1" }} />
                    <div className="text-sm font-medium">{event.phase}</div>
                    <div className="text-xs text-muted-foreground">{formatTime(event.first_seen)}</div>
                    <div className="text-xs text-muted-foreground">{event.task_count} {t("campaign.tasks")}</div>
                  </div>
                ))}
              </div>
            )}
          </Card>

          <Card>
            <h3 className="text-lg font-semibold mb-3">{t("campaign.agent_breakdown")}</h3>
            {stats.agent_breakdown.length === 0 ? (
              <p className="text-muted-foreground">{t("campaign.empty_agents")}</p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_hostname")}</TableHead>
                    <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_user")}</TableHead>
                    <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">IP</TableHead>
                    <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_tasks")}</TableHead>
                    <TableHead className="py-3 px-4 sm:py-3.5 sm:px-5">{t("campaign.col_phases")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {stats.agent_breakdown.map((a: AgentStat) => (
                    <TableRow key={a.agent_id}>
                      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-foreground truncate max-w-[200px]">{a.hostname}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-muted-foreground truncate max-w-[200px]">{a.username}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-muted-foreground">{a.ip}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-muted-foreground">{a.task_count}</TableCell>
                      <TableCell className="py-3 px-4 sm:py-3.5 sm:px-5 text-xs">
                        {Object.entries(a.phases || {}).map(([phase, count]) => (
                          <Badge key={phase} variant="secondary" className="inline-block mr-1 mb-1 px-1.5 py-0.5 rounded text-xs">
                            {phase}:{String(count)}
                          </Badge>
                        ))}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </Card>
        </>
      ) : (
        <p className="text-center text-muted-foreground py-8">
          <Spinner size="sm" /> {t("campaign.loading_stats")}
        </p>
      )}
    </div>
  );
}

