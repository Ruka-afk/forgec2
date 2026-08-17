"use client";
import { PageContainer } from "@/components/ui/page-container";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { PageSpinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Plug } from "lucide-react";

interface ListenerDetail {
  ID?: string;
  id?: string;
  Name?: string;
  name?: string;
  Host?: string;
  host?: string;
  Port?: number | string;
  port?: number | string;
  Type?: string;
  type?: string;
  Scheme?: string;
  scheme?: string;
  Protocol?: string;
  protocol?: string;
  Enabled?: boolean | string;
  enabled?: boolean;
  Notes?: string;
  notes?: string;
  CreatedAt?: string;
  created_at?: string;
}

interface ListenerAgent {
  id?: string;
  hostname?: string;
  ip?: string;
  os?: string;
  arch?: string;
  last_seen?: string;
  status?: string;
}

export default function ListenerDetailPage() {
  const params = useParams();
  const id = params?.id as string;
  const [listener, setListener] = useState<ListenerDetail | null>(null);
  const [agents, setAgents] = useState<ListenerAgent[]>([]);
  const [stats, setStats] = useState({ total: 0, active: 0 });
  const [loading, setLoading] = useState(true);
  const { t } = useI18n();

  const loadDetail = useCallback(async () => {
    if (!id) return;
    try {
      const data = await api.get(paths.listeners.one(id));
      setListener(data.listener || data);
      const a: ListenerAgent[] = (data.agents || []) as ListenerAgent[];
      setAgents(a);
      setStats({ total: a.length, active: a.filter(ag => (ag.status) === "online").length });
    } catch {
      setListener(null);
      setAgents([]);
      setStats({ total: 0, active: 0 });
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => { loadDetail(); }, [loadDetail]);

  if (loading) {
    return <PageContainer><PageSpinner /></PageContainer>;
  }

  if (!listener) {
    return (
      <PageContainer>
        <div className="text-center py-20 text-muted-foreground">
          <Plug className="w-4 h-4 mx-auto" />
          <p className="mt-2">{t("listeners.not_found")}</p>
        </div>
      </PageContainer>
    );
  }

  const name = listener.name || "Unknown";
  const scheme = listener.scheme || listener.protocol || listener.type || "http";
  const host = listener.host || "0.0.0.0";
  const port = listener.port ?? 8080;
  const isEnabled = listener.enabled === true;
  const notes = listener.notes || "-";
  const createdAt = listener.created_at || "-";

  return (
    <PageContainer title={name} subtitle={`${scheme}://${host}:${port}`}>
      <div className="flex items-center gap-x-4 mb-6">
        <div>
          
        </div>
        <div className="ml-auto">
          {isEnabled ? (
            <Badge variant="default">{t("listener.enabled")}</Badge>
          ) : (
            <Badge variant="secondary">{t("listener.disabled")}</Badge>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <div className="lg:col-span-1">
          <Card className="p-4 sm:p-5">
            <h3 className="font-semibold mb-4 text-foreground">{t("listener.info_title")}</h3>
            <div className="space-y-3 text-sm">
              <div><span className="text-muted-foreground">{t("listener.scheme")}</span> <span className="text-foreground">{scheme}</span></div>
              <div><span className="text-muted-foreground">{t("listener.address")}</span> <span className="text-foreground">{host}:{port}</span></div>
              <div><span className="text-muted-foreground">{t("listener.transport_type")}</span> <span className="text-foreground">{listener.type || scheme}</span></div>
              <div><span className="text-muted-foreground">{t("listener.notes")}</span> <span className="text-foreground">{notes}</span></div>
              <div><span className="text-muted-foreground">{t("listener.created")}</span> <span className="text-foreground">{createdAt}</span></div>
            </div>
            <div className="mt-6 flex gap-2">
               <Button render={<Link href={`/generate?listener_id=${id}`} />} className="flex-1">{t("listener.generate_implant")}</Button>
            </div>
          </Card>
        </div>

        <div className="lg:col-span-2">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card className="p-4 sm:p-5">
              <div className="text-xs text-muted-foreground">{t("listener.agent_total")}</div>
              <div className="text-4xl font-semibold mt-2 text-foreground">{stats.total}</div>
            </Card>
            <Card className="p-4 sm:p-5">
              <div className="text-xs text-muted-foreground">{t("listener.active_now")}</div>
              <div className="text-4xl font-semibold mt-2 text-success">{stats.active}</div>
            </Card>
            <Card className="p-4 sm:p-5">
              <div className="text-xs text-muted-foreground">{t("listener.lb_hint_title")}</div>
              <div className="text-sm mt-2 text-muted-foreground">{t("listener.lb_hint")}</div>
            </Card>
          </div>
        </div>
      </div>

      <div className="mt-8">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-xl font-bold text-foreground">{t("listener.agents_using")} ({stats.total})</h2>
          <Link href="/agents" className="text-sm text-primary hover:text-primary transition-colors">{t("listener.view_all_agents")}</Link>
        </div>

        <Card className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead className="py-3 px-6">{t("listeners.col_agent")}</TableHead>
                <TableHead className="py-3 px-4">IP</TableHead>
                <TableHead className="py-3 px-4">{t("listener.os")}</TableHead>
                <TableHead className="py-3 px-4">{t("listener.last_seen")}</TableHead>
                <TableHead className="py-3 px-4 text-center">{t("listener.status")}</TableHead>
                <TableHead className="py-3 px-6 text-right">{t("listener.action")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {agents.length > 0 ? (
                agents.map((a, i) => {
                  const aid = a.id || String(i);
                  const status = a.status || "offline";
                  return (
                    <TableRow key={aid}>
                      <TableCell className="py-3 px-6 font-medium text-foreground">{a.hostname || "-"}</TableCell>
                      <TableCell className="py-3 px-4 font-mono text-xs text-muted-foreground">{a.ip || "-"}</TableCell>
                      <TableCell className="py-3 px-4 text-xs text-muted-foreground">{a.os || "-"}/{a.arch || "-"}</TableCell>
                      <TableCell className="py-3 px-4 text-xs text-muted-foreground">{a.last_seen || "-"}</TableCell>
                      <TableCell className="py-3 px-4 text-center">
                        {status === "online" ? (
                          <Badge variant="default">{t("listener.online")}</Badge>
                        ) : (
                          <Badge variant="secondary">{t("listener.offline")}</Badge>
                        )}
                      </TableCell>
                      <TableCell className="py-3 px-6 text-right">
                        <Link href={`/agents/${aid}`} className="text-primary hover:text-primary hover:underline text-sm transition-colors">{t("listener.detail")}</Link>
                      </TableCell>
                    </TableRow>
                  );
                })
              ) : (
                <TableRow>
                  <TableCell colSpan={6} className="py-10 text-center text-muted-foreground">{t("listener.empty")}</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </Card>
      </div>
    </PageContainer>
  );
}
