"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { useI18n } from "@/lib/i18n";
import { ConfirmModal, PageHeader, Spinner } from "@/components/UI";
import { toast } from "sonner";
import { formatTime } from "@/lib/utils";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Cloud, HeartPulse, Info, Plus, RotateCw, Trash2 } from "lucide-react";

interface FrontDomain {
  domain: string;
  healthy: boolean;
  active: boolean;
  last_check: string;
  error?: string;
}

export default function DomainFrontingPage() {
  const { t } = useI18n();
  const [domains, setDomains] = useState<FrontDomain[]>([]);
  const [autoFailover, setAutoFailover] = useState(true);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [newDomain, setNewDomain] = useState("");
  const [saving, setSaving] = useState(false);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);



  const fetchStatus = useCallback(async () => {
    try {
      const data = await api.postJson("/infra/front/list", {});
      setDomains((data.domains as FrontDomain[]) || []);
      setAutoFailover((data.auto_failover ?? true) as boolean);
    } catch {
      setDomains([]);
      toast.error("Failed to fetch domain status");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchStatus(); }, [fetchStatus]);

  const handleCheck = async () => {
    setChecking(true);
    try {
      const data = await api.postJson("/infra/front/check", {});
      setDomains((data.domains as FrontDomain[]) || []);
      toast.success("Health check completed");
    } catch {
      toast.error("Health check failed");
    } finally {
      setChecking(false);
    }
  };

  const saveConfig = async (cfgDomains: string[], cfgAuto: boolean) => {
    setSaving(true);
    try {
      const data = await api.postJson("/infra/front/config", { domains: cfgDomains, auto_failover: cfgAuto });
      setDomains((data.domains as FrontDomain[]) || []);
      setAutoFailover(cfgAuto);
      toast.success("Configuration saved");
    } catch {
      toast.error("Failed to save configuration");
    } finally {
      setSaving(false);
    }
  };

  const addDomain = () => {
    const d = newDomain.trim();
    if (!d) return;
    if (domains.some((x) => x.domain === d)) {
      toast.error("Domain already in list");
      return;
    }
    const updated = [...domains.map((x) => x.domain), d];
    saveConfig(updated, autoFailover);
    setNewDomain("");
  };

  const removeDomain = (domain: string) => {
    setCfm({msg: t("domain_fronting.remove_domain", { domain }), cb: () => {
      const updated = domains.filter((x) => x.domain !== domain).map((x) => x.domain);
      saveConfig(updated, autoFailover);
    }});
  };

  const toggleAutoFailover = () => {
    saveConfig(domains.map((x) => x.domain), !autoFailover);
  };

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={<><Cloud className="w-4 h-4" />Domain Fronting</>} subtitle="CDN front domains with automatic failover and health monitoring">
        <Button onClick={handleCheck} disabled={checking || loading} size="sm">
          <HeartPulse className={`w-4 h-4 ${checking ? "animate-pulse" : ""}`} />
          {checking ? "Checking..." : "Health Check"}
        </Button>
        <Button onClick={fetchStatus} variant="ghost" size="sm">
          <RotateCw className="w-4 h-4" /> Refresh
        </Button>
      </PageHeader>

      {/* Auto-failover toggle */}
      <Card className="p-4 sm:p-5 mb-6 flex items-center justify-between">
        <div>
          <div className="font-medium text-foreground">Auto-Failover</div>
          <div className="text-sm text-muted-foreground mt-0.5">
            Automatically switch to a healthy domain when the active one becomes unreachable
          </div>
        </div>
        <Switch checked={autoFailover} onCheckedChange={toggleAutoFailover} disabled={saving} />
      </Card>

      {/* Active domain indicator */}
      <Card className="p-4 sm:p-5 mb-6">
        <div className="text-sm text-muted-foreground mb-1">Current Active Domain</div>
        <div className="flex items-center gap-3">
          {domains.filter((d) => d.active).length > 0 ? (
            <span className="text-lg font-semibold text-foreground">
              {domains.find((d) => d.active)?.domain}
            </span>
          ) : (
            <span className="text-sm text-muted-foreground">No domain configured</span>
          )}
          {domains.filter((d) => d.active && d.healthy).length > 0 && (
            <Badge variant="success">
              Healthy
            </Badge>
          )}
        </div>
      </Card>

      {/* Domain list */}
      <Card className="overflow-hidden mb-6">
        <div className="px-4 py-3 border-b border-border flex items-center justify-between">
          <h2 className="font-semibold text-foreground">Front Domains</h2>
          <span className="text-xs text-muted-foreground">{domains.length} domains</span>
        </div>

        {loading ? (
          <div className="p-4 sm:p-5 text-center text-muted-foreground">
            <Spinner size="md" />
            <div>Loading...</div>
          </div>
        ) : domains.length === 0 ? (
          <div className="p-4 sm:p-5 text-center text-muted-foreground">
            <Cloud className="w-4 h-4" />
            <div className="text-sm">No front domains configured</div>
            <div className="text-xs mt-1">Add a CDN domain below to get started</div>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Domain</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="w-12"></TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {domains.map((d) => (
                <TableRow key={d.domain}>
                  <TableCell>
                    <div className="flex items-center gap-3 min-w-0">
                      <span
                        className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                          d.healthy ? "bg-emerald-500" : "bg-destructive"
                        } ${d.active ? "animate-pulse" : ""}`}
                        title={d.healthy ? "Healthy" : "Unhealthy"}
                      />
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-foreground truncate">{d.domain}</span>
                          {d.active && (
                            <Badge variant="outline" className="text-[10px] px-1.5 py-px font-medium">
                              Active
                            </Badge>
                          )}
                        </div>
                        <div className="text-xs text-muted-foreground mt-0.5">
                          {d.last_check ? (
                            <>Last checked: {formatTime(d.last_check)}</>
                          ) : (
                            "Not checked yet"
                          )}
                          {d.error && (
                            <span className="text-destructive ml-2">Error: {d.error}</span>
                          )}
                        </div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline" className={`${
                      d.healthy
                        ? "bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400 border-emerald-200 dark:border-emerald-800"
                        : "bg-destructive/10 text-destructive border-destructive/20"
                    }`}>
                      {d.healthy ? "Healthy" : "Unhealthy"}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => removeDomain(d.domain)}
                      className="text-muted-foreground hover:text-destructive hover:bg-destructive/10"
                      title="Remove domain"
                    >
                      <Trash2 className="w-4 h-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* Add domain form */}
        <div className="px-4 py-3 border-t border-border bg-muted/50">
          <div className="flex items-center gap-2">
            <Input
              aria-label="cdn.cloudflare.com"
              name="input-0"
              type="text"
              value={newDomain}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => setNewDomain(e.target.value)}
              onKeyDown={(e: React.KeyboardEvent<HTMLInputElement>) => e.key === "Enter" && addDomain()}
              placeholder="cdn.cloudflare.com"
              className="flex-1"
            />
            <Button
              onClick={addDomain}
              disabled={!newDomain.trim() || saving}
            >
              <Plus className="w-4 h-4" />
              Add
            </Button>
          </div>
          <p className="text-xs text-muted-foreground mt-1.5">
            Enter a CDN front domain (e.g. <code className="text-indigo-500">cdn.cloudflare.com</code>). The first domain will be the active one.
          </p>
        </div>
      </Card>

      {/* Info card */}
      <div className="mt-6 p-4 bg-amber-50 dark:bg-amber-900/10 border border-amber-200 dark:border-amber-800 rounded-2xl">
        <div className="flex items-start gap-3">
          <Info className="w-4 h-4" />
          <div className="text-sm text-amber-800 dark:text-amber-200">
            <strong>How it works:</strong> The monitor performs periodic HEAD requests to each front domain using the path{" "}
            <code className="text-amber-600 dark:text-amber-400">/api/v1/beacon</code>. If the active domain fails (non-2xx/4xx response),
            auto-failover rotates to the next healthy domain in the list. Agents should be generated with the active domain set as
            their C2 URL.
          </div>
        </div>
      </div>
      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.remove")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}
