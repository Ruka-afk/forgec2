"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { phaseColor } from "@/lib/chart-palette";
import { PageSpinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
import { useAgentList } from "@/lib/hooks/useAgentList";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { Card } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { Check, CheckCircle, ChevronDown, CircleX, Zap } from "lucide-react";
import { Progress } from "@/components/ui/progress";
import { cn } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Collapsible, CollapsibleTrigger, CollapsibleContent } from "@/components/ui/collapsible";

interface AttackTechnique {
  id: string;
  name: string;
  tactic: string;
  task_types: string[];
}

interface TacticGroup {
  tactic: string;
  techniques: AttackTechnique[];
  covered: number;
  total: number;
}

interface AttackCoverageResponse {
  tactics: TacticGroup[];
  total_covered: number;
  total: number;
  used_task_types: string[];
}

interface PhaseCoverage {
  phase: string;
  total_tasks: number;
  campaigns_covered: number;
  total_campaigns: number;
}

const KILL_CHAIN_PHASES = [
  "Reconnaissance", "Resource Development", "Initial Access", "Execution",
  "Persistence", "Privilege Escalation", "Defense Evasion", "Credential Access",
  "Discovery", "Lateral Movement", "Collection", "Command and Control",
  "Exfiltration", "Impact",
];

const TACTIC_ORDER = [
  "Execution",
  "Persistence",
  "Privilege Escalation",
  "Defense Evasion",
  "Credential Access",
  "Discovery",
  "Collection",
  "Lateral Movement",
  "Command and Control",
];

const TACTIC_BORDER_COLORS: Record<string, string> = {
  "Execution": "border-rose-500/30",
  "Persistence": "border-blue-500/30",
  "Privilege Escalation": "border-orange-500/30",
  "Defense Evasion": "border-violet-500/30",
  "Credential Access": "border-yellow-500/30",
  "Discovery": "border-cyan-500/30",
  "Collection": "border-pink-500/30",
  "Lateral Movement": "border-emerald-500/30",
  "Command and Control": "border-red-500/30",
};

const TACTIC_HEADER_COLORS: Record<string, string> = {
  "Execution": "bg-rose-500",
  "Persistence": "bg-blue-500",
  "Privilege Escalation": "bg-orange-500",
  "Defense Evasion": "bg-violet-500",
  "Credential Access": "bg-yellow-500",
  "Discovery": "bg-cyan-500",
  "Collection": "bg-pink-500",
  "Lateral Movement": "bg-emerald-500",
  "Command and Control": "bg-red-500",
};

export default function AttackPage() {
  const { t } = useI18n();
  const { agents } = useAgentList();
  const [data, setData] = useState<AttackCoverageResponse | null>(null);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [loading, setLoading] = useState(true);
  const [expandedTactic, setExpandedTactic] = useState<string | null>(null);

  const fetchCoverage = useCallback(async (agentId: string, signal?: AbortSignal) => {
    try {
      const params = new URLSearchParams();
      if (agentId) params.set("agent_id", agentId);
      const json = await api.get<AttackCoverageResponse>(`/attack/coverage?${params}`, { signal });
      setData(json);
    } catch {
      toast.error(t("attack.fetch_failed"));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetchCoverage(selectedAgent, controller.signal);
    return () => controller.abort();
  }, [selectedAgent, fetchCoverage]);

  // Build a set of used task types for quickly checking technique coverage
  const usedTaskTypes = new Set(data?.used_task_types ?? []);

  // Helper to check if a technique is covered
  const isTechniqueCovered = (tech: AttackTechnique): boolean => {
    return tech.task_types.some((tt) => usedTaskTypes.has(tt));
  };

  const sortedTactics = data
    ? [...data.tactics].sort((a, b) => {
        const ai = TACTIC_ORDER.indexOf(a.tactic);
        const bi = TACTIC_ORDER.indexOf(b.tactic);
        return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi);
      })
    : [];

  const percentage = data && data.total > 0 ? Math.round((data.total_covered / data.total) * 100) : 0;

  if (loading && !data) {
    return (
      <PageContainer title={t("attack.title")} subtitle={t("attack.subtitle")}>
        <PageSpinner />
      </PageContainer>
    );
  }

  return (
    <PageContainer title={t("attack.title")} subtitle={t("attack.subtitle")} contentClassName="space-y-6" actions={<>
        <Select value={selectedAgent || "all"} onValueChange={(v) => setSelectedAgent(v === "all" ? "" : v ?? "")}>
          <SelectTrigger className="max-w-[250px]">
            <SelectValue placeholder={t("attack.all_agents")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("attack.all_agents")}</SelectItem>
            {agents
              .filter((a) => a.id && a.hostname)
              .map((a) => (
                <SelectItem key={a.id || ""} value={a.id || ""}>
                  {a.hostname} ({a.ip || (a.id || "").slice(0, 8)})
                </SelectItem>
              ))}
          </SelectContent>
        </Select>
      </>}>

      <Card className="p-3 border-warning/40 bg-warning/10 text-sm text-warning-foreground">
        <div className="font-semibold">{t("attack.honesty_title")}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{t("attack.honesty_desc")}</div>
      </Card>

      {/* Summary Card */}
      <Card className="p-4 sm:p-5">
        <div className="flex items-center justify-between flex-wrap gap-4">
          <div>
            <div className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
              {t("attack.total_coverage")}
            </div>
            <div className="text-4xl font-bold mt-2 text-foreground">
              {data?.total_covered ?? 0}
              <span className="text-xl text-muted-foreground font-normal">/{data?.total ?? 0}</span>
            </div>
            <div className="text-sm mt-1 text-muted-foreground">
              {t("attack.techniques_used")}
            </div>
          </div>
          <div className="flex flex-col items-center">
            <div className="relative w-24 h-24">
              <svg className="w-24 h-24 -rotate-90" viewBox="0 0 36 36" role="img" aria-label={t("attack.progress_ring")}>
                <circle cx="18" cy="18" r="15.5" fill="none" stroke="currentColor" strokeWidth="3"
                  className="text-border" />
                <circle cx="18" cy="18" r="15.5" fill="none" stroke="currentColor" strokeWidth="3"
                  strokeDasharray={`${percentage * 0.9722} 100`}
                  className="text-primary"
                  strokeLinecap="round" />
              </svg>
              <div className="absolute inset-0 flex items-center justify-center">
                <span className="text-2xl font-bold text-primary">{percentage}%</span>
              </div>
            </div>
            <span className="text-xs text-muted-foreground mt-1">{t("attack.coverage")}</span>
          </div>
        </div>
      </Card>

      {/* Kill Chain Phase Coverage */}
      <PhaseCoverageCard />

      {/* Tactic Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {sortedTactics.map((tactic) => {
          const isExpanded = expandedTactic === tactic.tactic;
          const headerColor = TACTIC_HEADER_COLORS[tactic.tactic] || "bg-primary";
          const borderColor = TACTIC_BORDER_COLORS[tactic.tactic] || "border-primary/30";
          const tacticPct = tactic.total > 0 ? Math.round((tactic.covered / tactic.total) * 100) : 0;

          return (
            <Card
              key={tactic.tactic}
              className={`overflow-hidden shadow-sm ${borderColor} border`}
            >
              <Collapsible open={isExpanded} onOpenChange={(open) => setExpandedTactic(open ? tactic.tactic : null)}>
              <CollapsibleTrigger className="flex items-center justify-between px-4 py-3 cursor-pointer hover:text-muted-foreground transition-opacity w-full">
                <div className="flex items-center gap-3">
                  <div className={`w-3 h-3 rounded-full ${headerColor}`}></div>
                  <div>
                    <div className="text-sm font-semibold text-foreground">
                      {tactic.tactic}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {tactic.covered}/{tactic.total} {t("attack.techniques")}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <div className="flex items-center gap-1.5">
                    <Progress value={tacticPct} className="w-20" indicatorClassName={cn(
                      "h-full rounded-full transition-all duration-500",
                      tacticPct >= 50 ? "bg-primary" : tacticPct >= 25 ? "bg-warning" : "bg-destructive"
                    )} />
                    <span className="text-xs font-mono text-muted-foreground tabular-nums w-8 text-right">
                      {tacticPct}%
                    </span>
                  </div>
                   <ChevronDown className="w-3 h-3 text-muted-foreground" />
                </div>
              </CollapsibleTrigger>

              <CollapsibleContent>
                <div className="border-t border-border divide-y divide-border">
                  {tactic.techniques.map((tech) => {
                    const covered = isTechniqueCovered(tech);
                    return (
                      <div
                        key={tech.id}
                        className={`px-4 py-2.5 flex items-center justify-between transition-colors ${
                          covered
                            ? "bg-success/10"
                            : "hover:bg-muted/30"
                        }`}
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          {/* Covered indicator */}
                          {covered ? (
                            <CheckCircle className="w-4 h-4" />
                          ) : (
                            <CircleX className="w-4 h-4 text-muted-foreground shrink-0" />
                          )}
                          <code className="text-xs font-mono text-primary shrink-0 w-20">
                            {tech.id}
                          </code>
                          <span className={`text-sm truncate ${
                            covered
                              ? "text-foreground font-medium"
                              : "text-muted-foreground"
                          }`}>
                            {tech.name}
                          </span>
                        </div>
                        <div className="flex items-center gap-2 shrink-0 ml-2">
                          {tech.task_types.slice(0, 3).map((tt) => (
                            <Badge
                              key={tt}
                              variant={usedTaskTypes.has(tt) ? "success" : "outline"}
                              className="text-(--fs-micro-sm) font-mono"
                            >
                              {tt}
                            </Badge>
                          ))}
                          {tech.task_types.length > 3 && (
                            <span className="text-(--fs-micro-sm) text-muted-foreground">+{tech.task_types.length - 3}</span>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              </CollapsibleContent>
              </Collapsible>
            </Card>
          );
        })}
      </div>
    </PageContainer>
  );
}

function PhaseCoverageCard() {
  const { t } = useI18n();
  const { data: resp, loading } = useApiResource<{ success?: boolean; data?: PhaseCoverage[] }>({
    fetcher: async () => {
      const json = await api.get(paths.mitre.phases);
      return json;
    },
    toastThrottleMs: 10_000,
    errorMessage: t("attack.load_phases_failed"),
  });
  const phases = resp?.data ?? [];

  if (loading || phases.length === 0) return null;

  const maxCoverage = Math.max(...phases.map((p) => p.campaigns_covered), 1);

  return (
    <Card className="p-4 sm:p-5">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h3 className="text-lg font-semibold text-foreground">
            <Zap className="w-4 h-4" />{t("attack.kill_chain_title")}
          </h3>
          <p className="text-xs text-muted-foreground mt-0.5">
            {t("attack.kill_chain_subtitle")}
          </p>
        </div>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-7 gap-2">
        {KILL_CHAIN_PHASES.map((phase) => {
          const found = phases.find((p) => p.phase === phase);
          const isCovered = found && found.campaigns_covered > 0;
          const pct = found ? Math.round((found.campaigns_covered / maxCoverage) * 100) : 0;
          return (
            <div key={phase} className={`p-3 rounded-lg border text-center transition-colors ${
              isCovered ? "border-success/30 bg-success/10" : "border-border bg-card"
            }`}>
              <div className="w-6 h-6 rounded-full mx-auto mb-1.5 flex items-center justify-center"
                style={{ background: phaseColor(phase) || "#6366f1" }}>
                {isCovered ? (
                  <Check className="w-4 h-4" />
                ) : (
                  <span className="text-white text-(--fs-micro-sm) font-bold">-</span>
                )}
              </div>
              <div className="text-(--fs-micro-sm) font-medium text-foreground leading-tight mb-1">
                {phase.split(" ").slice(0, 2).join(" ")}
              </div>
              <div className="w-full h-1.5 rounded-full bg-secondary overflow-hidden">
                <div className={`h-full rounded-full transition-all ${isCovered ? "bg-success" : "bg-muted-foreground"}`}
                  style={{ width: `${pct}%` }} />
              </div>
              {found && (
                <div className="text-(--fs-micro) text-muted mt-1">
                  {found.total_tasks} {t("attack.tasks")}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </Card>
  );
}
