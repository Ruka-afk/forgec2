"use client";

import { PageContainer } from "@/components/ui/page-container";
import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { useLiveTaskResult } from "@/lib/hooks/useLiveTaskResult";
import { Spinner } from "@/components/ui/spinner";
import { Banner } from "@/components/ui/banner";
import { Shield, Play, CheckCircle2, XCircle, AlertTriangle, Lock, Database } from "lucide-react";
import { toast } from "sonner";

interface Agent {
  id: string;
  hostname: string;
  status: string;
  os: string;
}

interface SpraySummary {
  total: number;
  valid: number;
  invalid: number;
  locked: number;
  errors: number;
}

interface SprayResultRow {
  user: string;
  status: string;
  error?: string;
}

interface SprayOutput {
  summary: SpraySummary;
  results: SprayResultRow[];
}

function parseSprayOutput(raw: string): SprayOutput | null {
  try {
    const data = JSON.parse(raw) as SprayOutput;
    if (!data || !Array.isArray(data.results)) return null;
    return data;
  } catch {
    return null;
  }
}

export default function PasswordSprayPage() {
  const { t } = useI18n();
  const [selectedAgent, setSelectedAgent] = useState("");
  const [password, setPassword] = useState("");
  const [domain, setDomain] = useState("");
  const [dc, setDc] = useState("");
  const [delayMs, setDelayMs] = useState("500");
  const [usernames, setUsernames] = useState("");
  const [lastResult, setLastResult] = useState<{
    task_id: number;
    summary?: SpraySummary;
    results?: SprayResultRow[];
    raw?: string;
  } | null>(null);
  const live = useLiveTaskResult({ timeoutMs: 600_000 });

  const { data } = useApiResource<{ agents: Agent[] }>({
    fetcher: async () => {
      const d = await api.get<{ agents: Agent[] }>(paths.agents.list());
      return d;
    },
    pollMs: 30_000,
    errorMessage: t("spray.load_failed"),
  });

  const agents = data?.agents ?? [];
  const onlineAgents = agents.filter((a) => a.status === "online");

  const usernameList = usernames.split("\n").filter((u) => u.trim());
  const running = live.running;
  const canSubmit = selectedAgent && password && domain && usernameList.length > 0 && !running;

  async function handleSpray() {
    if (!canSubmit) return;
    setLastResult(null);
    toast.success(t("spray.task_sent"));
    const final = await live.run(selectedAgent, paths.agents.cmd(selectedAgent, "password_spray"), {
      password,
      domain,
      dc,
      delay_ms: delayMs,
      usernames: usernames.trim(),
    });
    if (!final) {
      toast.error(t("spray.send_failed") + ": " + live.error);
      return;
    }
    if (final.status === "completed") {
      const parsed = parseSprayOutput(final.result);
      if (parsed) {
        setLastResult({ task_id: final.id, summary: parsed.summary, results: parsed.results });
      } else {
        setLastResult({ task_id: final.id, raw: final.result });
      }
    } else {
      toast.error(t("spray.send_failed") + ": " + (final.error || live.error));
    }
  }

  const statusIcon = (status: string) => {
    switch (status) {
      case "valid": return <CheckCircle2 className="w-4 h-4 text-success" />;
      case "locked": return <Lock className="w-4 h-4 text-warning" />;
      case "error": return <AlertTriangle className="w-4 h-4 text-warning" />;
      default: return <XCircle className="w-4 h-4 text-destructive" />;
    }
  };

  return (
    <PageContainer
      title={t("spray.title")}
      subtitle={t("spray.subtitle")}
      actions={
        <Select value={selectedAgent} onValueChange={(v) => setSelectedAgent(v ?? "")}>
          <SelectTrigger className="w-64">
            <SelectValue placeholder={t("spray.select_agent")} />
          </SelectTrigger>
          <SelectContent>
            {onlineAgents.map((a) => (
              <SelectItem key={a.id} value={a.id}>
                {a.hostname} ({a.os})
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      }
    >
      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="p-6 space-y-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <Shield className="w-5 h-5 text-primary" />
            {t("spray.config_title")}
          </h3>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label>{t("spray.domain")} *</Label>
              <Input value={domain} onChange={(e) => setDomain(e.target.value)} placeholder="CORP" />
            </div>
            <div className="space-y-1.5">
              <Label>{t("spray.dc_ip")}</Label>
              <Input value={dc} onChange={(e) => setDc(e.target.value)} placeholder={t("spray.dc_hint")} />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label>{t("spray.password")} *</Label>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} placeholder={t("spray.password_placeholder")} />
            </div>
            <div className="space-y-1.5">
              <Label>{t("spray.delay_ms")}</Label>
              <Input type="number" value={delayMs} onChange={(e) => setDelayMs(e.target.value)} min="0" max="30000" />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label>{t("spray.usernames")} * <span className="text-xs text-muted-foreground">({usernameList.length} {t("spray.users_counted")})</span></Label>
            <Textarea
              value={usernames}
              onChange={(e) => setUsernames(e.target.value)}
              placeholder={t("spray.usernames_placeholder")}
              rows={10}
              className="font-mono text-sm"
            />
          </div>

          <Button onClick={handleSpray} disabled={!canSubmit} variant="gradient" className="w-full">
            <Play className="w-4 h-4" />
            {running ? t("spray.sending") : t("spray.execute")}
          </Button>
        </Card>

        <Card className="p-6 space-y-4">
          <h3 className="text-lg font-semibold">{t("spray.results_title")}</h3>

          {!lastResult ? (
            <div className="flex items-center justify-center gap-3 h-64 text-muted-foreground">
              {running && <Spinner className="w-5 h-5" />}
              {running ? t("spray.waiting_result") : t("spray.no_results")}
            </div>
          ) : (
            <div className="space-y-4">
              {lastResult.summary && lastResult.summary.valid > 0 && (
                <Banner tone="success" icon={<Database className="w-4 h-4" />}>
                  {t("spray.creds_imported")}
                </Banner>
              )}
              {lastResult.raw && (
                <pre className="max-h-96 overflow-y-auto text-xs font-mono bg-muted/50 rounded-lg p-3 whitespace-pre-wrap">
                  {lastResult.raw}
                </pre>
              )}
              {lastResult.summary && (
                <div className="grid grid-cols-5 gap-2">
                  {[
                    { label: t("spray.total"), value: lastResult.summary.total, variant: "outline" as const },
                    { label: t("spray.valid"), value: lastResult.summary.valid, variant: "success" as const },
                    { label: t("spray.invalid"), value: lastResult.summary.invalid, variant: "destructive" as const },
                    { label: t("spray.locked"), value: lastResult.summary.locked, variant: "warning" as const },
                    { label: t("spray.errors"), value: lastResult.summary.errors, variant: "secondary" as const },
                  ].map((s) => (
                    <div key={s.label} className="text-center">
                      <div className="text-2xl font-bold">{s.value}</div>
                      <Badge variant={s.variant} className="text-xs">{s.label}</Badge>
                    </div>
                  ))}
                </div>
              )}

              {lastResult.results && (
                <div className="max-h-96 overflow-y-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border">
                        <th className="text-left py-2">{t("spray.col_status")}</th>
                        <th className="text-left py-2">{t("spray.col_username")}</th>
                        <th className="text-left py-2">{t("spray.col_detail")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {lastResult.results.map((r, i) => (
                        <tr key={i} className="border-b border-border/50">
                          <td className="py-1.5">{statusIcon(r.status)}</td>
                          <td className="py-1.5 font-mono">{r.user}</td>
                          <td className="py-1.5 text-muted-foreground">{r.error || r.status}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {!lastResult.summary && !lastResult.raw && lastResult.task_id && (
                <div className="flex items-center justify-center h-32 text-muted-foreground">
                  {t("spray.waiting_result")}
                </div>
              )}
            </div>
          )}
        </Card>
      </div>
    </PageContainer>
  );
}
