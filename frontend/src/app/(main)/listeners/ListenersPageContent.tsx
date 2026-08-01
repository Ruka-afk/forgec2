"use client";

import { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { api } from "@/lib/api";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { EmptyState, PageHeader, ConfirmModal, StatusBadge } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { DataError } from "@/components/ui/data-state";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { ArrowLeftRight, Copy, Info, Pencil, Plug, Plus, Power, Trash2 } from "lucide-react";

interface Listener {
  id?: string;
  ID?: string;
  name?: string;
  Name?: string;
  type?: string;
  Type?: string;
  Scheme?: string;
  scheme?: string;
  Protocol?: string;
  protocol?: string;
  host?: string;
  Host?: string;
  port?: number | string;
  Port?: number | string;
  enabled?: boolean;
  Enabled?: boolean;
  notes?: string;
  Notes?: string;
  tags?: string;
  Tags?: string;
  color?: string;
  Color?: string;
  status?: string;
  Status?: string;
}


export default function ListenersPageContent() {
  const { t } = useI18n();
  const [listeners, setListeners] = useState<Listener[]>([]);
  const [loading, setLoading] = useState(true);
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ name: "", type: "http", host: "0.0.0.0", port: "8443", protocol: "http", tags: "", color: "" });

  const handleTypeChange = (type: string) => {
    setCreateForm(prev => ({ ...prev, type, protocol: type === "https" ? "https" : type }));
  };
  const [creating, setCreating] = useState(false);
  const [editingListener, setEditingListener] = useState<Listener | null>(null);
  const [showEdit, setShowEdit] = useState(false);
  const [editForm, setEditForm] = useState({ name: "", type: "http", host: "0.0.0.0", port: "", protocol: "http", notes: "", tags: "", color: "" });
  const handleEditTypeChange = (type: string) => {
    setEditForm(prev => ({ ...prev, type, protocol: type === "https" ? "https" : type }));
  };
  const [error, setError] = useState<string | null>(null);
  const [cfm, setCfm] = useState<{msg: string; cb: () => void} | null>(null);
  const [agentCountMap, setAgentCountMap] = useState<Record<string, number>>({});

  const loadListeners = useCallback(async () => {
    try {
      const data = await api.get("/listeners");
      setListeners((data.data || data.listeners || []) as Listener[]);
    } catch (e) {
      setListeners([]);
      setError(e instanceof Error ? e.message : t("listeners.toast_load_failed"));
    }
    setLoading(false);
  }, [t]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!createForm.name || !createForm.host || !createForm.port) {
      toast.error(t("listeners.toast_name_host_port_required"));
      return;
    }
    setCreating(true);
    try {
      const data = await api.postJson("/api/listeners", {
        name: createForm.name,
        type: createForm.type,
        host: createForm.host,
        port: parseInt(createForm.port, 10) || 0,
        scheme: createForm.protocol,
        tags: createForm.tags,
        color: createForm.color,
      });
      if (data.success) {
        setShowCreate(false);
        setCreateForm({ name: "", type: "http", host: "0.0.0.0", port: "8443", protocol: "http", tags: "", color: "" });
        loadListeners();
        toast.success(t("listeners.toast_created"));
      } else {
        toast.error((data.error as string) || t("listeners.toast_unknown_error"));
      }
    } catch {
      toast.error(t("listeners.toast_create_failed"));
    }
    setCreating(false);
  };

  const handleCopy = async (address: string) => {
    try {
      await navigator.clipboard.writeText(address);
      toast.success(t("listeners.toast_address_copied"));
    } catch { toast.error(t("listeners.toast_copy_failed")); }
  };

  const handleEdit = (listener: Listener) => {
    setEditingListener(listener);
    setEditForm({
      name: listener.name || "",
      type: listener.type || "http",
      host: listener.host || "",
      port: String(listener.port ?? ""),
      protocol: listener.scheme || listener.protocol || listener.type || "http",
      notes: listener.notes || "",
      tags: listener.tags || "",
      color: listener.color || "",
    });
    setShowEdit(true);
  };

  const handleEditSave = async (e: React.FormEvent) => {
    e.preventDefault();
    const id = editingListener?.id || editingListener?.ID || "";
    if (!id || !editForm.name || !editForm.host || !editForm.port) {
      toast.error(t("listeners.toast_name_host_port_required"));
      return;
    }
    try {
      await api.putJson(`/api/listeners/${id}`, {
        name: editForm.name,
        type: editForm.type,
        host: editForm.host,
        port: parseInt(editForm.port) || 0,
        protocol: editForm.protocol,
        notes: editForm.notes,
        tags: editForm.tags,
        color: editForm.color,
      });
      setShowEdit(false);
      setEditingListener(null);
      loadListeners();
      toast.success(t("listeners.toast_updated"));
    } catch { toast.error(t("listeners.toast_update_failed")); }
  };

  const handleToggle = async (listener: Listener) => {
    const id = listener.id || "";
    if (!id) return;
    const enabled = !(listener.enabled === true);
    try {
      if (enabled) {
        await api.postJson(`/api/listeners/${id}/enable`, {});
      } else {
        await api.postJson(`/api/listeners/${id}/disable`, {});
      }
      loadListeners();
      toast.success(enabled ? t("listeners.toast_enabled") : t("listeners.toast_disabled"));
    } catch { toast.error(t("listeners.toast_toggle_failed")); }
  };

  const handleDelete = (listener: Listener) => {
    const id = listener.id || "";
    if (!id) return;
    setCfm({msg: t("listeners.confirm_delete"), cb: async () => {
      try {
        await api.del(`/api/listeners/${id}`);
        loadListeners();
        toast.success(t("listeners.toast_deleted"));
      } catch { toast.error(t("listeners.toast_delete_failed")); }
    }});
  };

  useEffect(() => {
    const controller = new AbortController();
    loadListeners();
    api.get("/agents?page=1&pageSize=500", { signal: controller.signal })
        .then((d) => {
          const agents = d.agents || [];
          const map: Record<string, number> = {};
          (agents as { listener_id?: number; ListenerID?: number }[]).forEach((a) => {
            const lid = String(a.listener_id ?? "");
            if (lid && lid !== "0") map[lid] = (map[lid] || 0) + 1;
          });
          setAgentCountMap(map);
        })
        .catch(() => setAgentCountMap({}));
    return () => controller.abort();
  }, [loadListeners]);

  const total = listeners.length;
  const enabledCount = listeners.filter(l => l.enabled === true).length;
  const httpCount = listeners.filter(l => (l.type) === "http").length;
  const tcpCount = listeners.filter(l => (l.type) === "tcp").length;
  const dnsCount = listeners.filter(l => (l.type) === "dns").length;

  const filtered = listeners.filter(l => {
    const type = l.type || "";
    if (typeFilter && type !== typeFilter) return false;
    const enabled = l.enabled === true;
    if (statusFilter === "enabled" && !enabled) return false;
    if (statusFilter === "disabled" && enabled) return false;
    return true;
  });

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      {error && (
        <DataError
          message={error}
          onDismiss={() => setError(null)}
          className="mb-4"
        />
      )}
      <PageHeader title={t("listeners.title")} subtitle={t("listeners.subtitle")}>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="w-4 h-4" />
          <span>{t("listeners.create_listener")}</span>
        </Button>
      </PageHeader>

      <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3 sm:gap-4 mb-4 sm:mb-6">
        <Card className="rounded-2xl p-4 sm:p-5">
          <div className="text-(--font-size-micro-sm) sm:text-xs text-muted-foreground">{t("listeners.total")}</div>
          <div className="text-2xl font-bold mt-1 text-foreground">{loading ? "..." : total}</div>
        </Card>
        <Card className="rounded-2xl p-4 sm:p-5">
          <div className="text-(--font-size-micro-sm) sm:text-xs text-muted-foreground">{t("listeners.running")}</div>
          <div className="text-2xl font-bold mt-1 text-emerald-600">{loading ? "..." : enabledCount}</div>
        </Card>
        <Card className="rounded-2xl p-4 sm:p-5">
          <div className="text-(--font-size-micro-sm) sm:text-xs text-muted-foreground">{t("listeners.http")}</div>
          <div className="text-2xl font-bold mt-1 text-foreground">{loading ? "..." : httpCount}</div>
        </Card>
        <Card className="rounded-2xl p-4 sm:p-5">
          <div className="text-(--font-size-micro-sm) sm:text-xs text-muted-foreground">{t("listeners.tcp")}</div>
          <div className="text-2xl font-bold mt-1 text-foreground">{loading ? "..." : tcpCount}</div>
        </Card>
        <Card className="rounded-2xl p-4 sm:p-5 col-span-2 sm:col-span-1">
          <div className="text-(--font-size-micro-sm) sm:text-xs text-muted-foreground">{t("listeners.dns")}</div>
          <div className="text-2xl font-bold mt-1 text-primary">{loading ? "..." : dnsCount}</div>
        </Card>
      </div>

      <div className="flex flex-col sm:flex-row gap-3 mb-4">
        <div className="flex gap-2">
          <Select value={typeFilter} onValueChange={(val) => setTypeFilter(typeof val === "string" ? val : "")}>
            <SelectTrigger className="flex-1 h-11">
              <SelectValue placeholder={t("listeners.all_types")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("listeners.all_types")}</SelectItem>
              <SelectItem value="http">HTTP</SelectItem>
              <SelectItem value="https">HTTPS</SelectItem>
              <SelectItem value="tcp">TCP</SelectItem>
              <SelectItem value="dns">DNS</SelectItem>
              <SelectItem value="smb">SMB</SelectItem>
              <SelectItem value="icmp">ICMP</SelectItem>
              <SelectItem value="ssh">SSH</SelectItem>
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={(val) => setStatusFilter(typeof val === "string" ? val : "")}>
            <SelectTrigger className="flex-1 h-11">
              <SelectValue placeholder={t("listeners.all_statuses")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("listeners.all_statuses")}</SelectItem>
              <SelectItem value="enabled">{t("listeners.enabled")}</SelectItem>
              <SelectItem value="disabled">{t("listeners.disabled")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      <Card className="rounded-2xl overflow-hidden">
        <div className="overflow-x-auto">
        <Table>
          <TableHeader className="bg-muted border-b border-border">
            <TableRow>
              <TableHead className="text-left py-3 px-4 sm:py-4 sm:px-6 font-medium text-muted-foreground min-w-[120px]">{t("listeners.col_name")}</TableHead>
              <TableHead className="max-sm:hidden text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[80px]">{t("listeners.col_type")}</TableHead>
              <TableHead className="max-sm:hidden text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[100px]">{t("listeners.col_tags")}</TableHead>
              <TableHead className="max-sm:hidden text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[160px]">{t("listeners.col_address")}</TableHead>
              <TableHead className="max-sm:hidden text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[60px]">{t("listeners.col_agents")}</TableHead>
              <TableHead className="text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[80px]">{t("listeners.col_status")}</TableHead>
              <TableHead className="max-sm:hidden text-left py-3 px-3 sm:py-4 sm:px-4 font-medium text-muted-foreground min-w-[120px]">{t("listeners.col_notes")}</TableHead>
              <TableHead className="text-right py-3 px-4 sm:py-4 sm:px-6 font-medium text-muted-foreground min-w-[200px]">{t("listeners.col_actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody className="divide-y divide-border">
            {loading ? (
              [1, 2, 3, 4, 5].map(i => (
                <TableRow key={i}>
                  <TableCell className="py-3 px-4 sm:py-4 sm:px-6"><Skeleton className="h-4 w-24" /></TableCell>
                  <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4"><Skeleton className="h-4 w-14" /></TableCell>
                  <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4"><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4"><Skeleton className="h-4 w-32" /></TableCell>
                  <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4"><Skeleton className="h-4 w-16" /></TableCell>
                  <TableCell className="py-3 px-3 sm:py-4 sm:px-4"><Skeleton className="h-4 w-20" /></TableCell>
                  <TableCell className="py-3 px-4 sm:py-4 sm:px-6 text-right"><div className="flex items-center justify-end gap-1 sm:gap-2"><Skeleton className="h-8 w-20 rounded-lg" /><Skeleton className="h-8 w-20 rounded-lg" /><Skeleton className="h-8 w-20 rounded-lg" /><Skeleton className="h-8 w-20 rounded-lg" /></div></TableCell>
                </TableRow>
              ))
            ) : filtered.length > 0 ? (
              filtered.map(l => {
                const id = l.id || "";
                const name = l.name || "";
                const type = l.type || "";
                const scheme = l.scheme || l.protocol || type;
                const host = l.host || "";
                const port = l.port ?? "";
                const enabled = l.enabled === true;
                const notes = l.notes || "-";
                const tags = l.tags || "";
                const tagList = tags.split(",").map(t => t.trim()).filter(Boolean);
                return (
                  <TableRow key={id} className="hover:bg-muted/50 transition-colors">
                    <TableCell className="py-3 px-4 sm:py-4 sm:px-6 font-medium"><Link href={`/listeners/${id}`} className="text-indigo-600 hover:underline">{name}</Link></TableCell>
                    <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4">
                      <Badge variant={type === "http" ? "outline" : "success"} className="text-(--font-size-xs-sm) font-medium">{type}</Badge>
                    </TableCell>
                    <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4">
                      <div className="flex flex-wrap gap-1">
                        {tagList.length > 0 ? tagList.map((tag) => (
                          <Badge key={tag} variant="secondary" className="text-(--font-size-micro-sm) font-medium">{tag}</Badge>
                        )) : <span className="text-xs text-muted-foreground">-</span>}
                      </div>
                    </TableCell>
                    <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4 font-mono text-xs text-muted-foreground">{scheme}://{host}:{port}</TableCell>
                    <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4 text-center">
                      <span className="text-xs font-mono text-muted-foreground">{agentCountMap[id] ?? 0}</span>
                    </TableCell>
                    <TableCell className="py-3 px-3 sm:py-4 sm:px-4">
                      <StatusBadge status={enabled ? "online" : "offline"} />
                    </TableCell>
                    <TableCell className="max-sm:hidden py-3 px-3 sm:py-4 sm:px-4 text-xs text-muted-foreground max-w-[150px] truncate">{notes}</TableCell>
                     <TableCell className="py-3 px-4 sm:py-4 sm:px-6 text-right">
                       <div className="flex items-center justify-end gap-1 sm:gap-2">
                         <Button variant="ghost" onClick={() => handleCopy(`${scheme}://${host}:${port}`)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-secondary hover:bg-secondary/80 rounded-xl flex items-center justify-center gap-x-1 text-muted-foreground">
                            <Copy className="w-4 h-4" />
                            <span className="hidden sm:inline">{t("listeners.copy")}</span>
                         </Button>
                         <Button variant="ghost" onClick={() => handleEdit(l)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-indigo-100 dark:bg-indigo-900/40 hover:bg-indigo-200 dark:hover:bg-indigo-900/60 text-indigo-700 dark:text-indigo-400 rounded-xl flex items-center justify-center gap-x-1">
                            <Pencil className="w-4 h-4" />
                            <span className="hidden sm:inline">{t("listeners.edit")}</span>
                         </Button>
                         <Button variant="ghost" onClick={() => handleToggle(l)} className={`w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs ${enabled ? "bg-amber-100 dark:bg-amber-900/40 text-amber-700 dark:text-amber-400 hover:bg-amber-200 dark:hover:bg-amber-900/60" : "bg-emerald-100 dark:bg-emerald-900/40 text-emerald-700 dark:text-emerald-400 hover:bg-emerald-200 dark:hover:bg-emerald-900/60"} rounded-xl flex items-center justify-center gap-x-1`}>
                            <Power className="w-4 h-4" />
                               <span className="hidden sm:inline">{enabled ? t("listeners.stop") : t("listeners.start")}</span>
                         </Button>
                          <Button variant="ghost" onClick={() => handleDelete(l)} className="w-9 h-9 sm:px-3 sm:py-1 sm:w-auto sm:h-auto text-xs bg-destructive/10 hover:bg-destructive/20 text-destructive rounded-xl flex items-center justify-center gap-x-1">
                           <Trash2 className="w-4 h-4" />
                             <span className="hidden sm:inline">{t("listeners.delete")}</span>
                         </Button>
                       </div>
                     </TableCell>
                  </TableRow>
                );
              })
            ) : (
              <TableRow>
                <TableCell colSpan={8} className="py-12 text-center text-muted-foreground">
                  <EmptyState icon={Plug} title={t("listeners.empty_title")} message={t("listeners.empty_message")} />
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
        </div>
        <div className="sm:hidden px-4 py-2 text-center text-xs text-muted-foreground border-t border-border bg-muted">
          <ArrowLeftRight className="w-4 h-4" /> {t("listeners.swipe_more")}
        </div>
      </Card>

      <div className="mt-4 sm:mt-6 p-3 sm:p-4 bg-primary/10 border border-primary/20 rounded-2xl text-xs text-primary">
        <Info className="w-4 h-4" />
        <strong>{t("listeners.tip")}</strong> {t("listeners.tip_text")}
      </div>

      <Dialog open={showCreate} onOpenChange={(open) => setShowCreate(open)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("listeners.create_dialog_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleCreate}>
            <div className="space-y-3">
              <div>
                <Label className="text-xs mb-1" htmlFor="create-name">{t("listeners.field_name")}</Label>
                <Input id="create-name" value={createForm.name} onChange={(e) => setCreateForm({ ...createForm, name: e.target.value })}
                  placeholder="my-listener" />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="create-type">{t("listeners.field_type")}</Label>
                <Select value={createForm.type} onValueChange={(val) => handleTypeChange(typeof val === "string" ? val : "http")}>
                  <SelectTrigger id="create-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="http">HTTP</SelectItem>
                    <SelectItem value="https">HTTPS</SelectItem>
                    <SelectItem value="tcp">TCP</SelectItem>
                    <SelectItem value="dns">DNS</SelectItem>
                    <SelectItem value="smb">SMB</SelectItem>
                    <SelectItem value="icmp">ICMP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="create-host">{t("listeners.field_host")}</Label>
                <Input id="create-host" value={createForm.host} onChange={(e) => setCreateForm({ ...createForm, host: e.target.value })} />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="create-port">{t("listeners.field_port")}</Label>
                <Input id="create-port" type="number" min="1" max="65535" value={createForm.port} onChange={(e) => setCreateForm({ ...createForm, port: e.target.value })}
                  placeholder="8443" />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="create-tags">{t("listeners.field_tags")}</Label>
                <Input id="create-tags" value={createForm.tags} onChange={(e) => setCreateForm({ ...createForm, tags: e.target.value })}
                  placeholder="prod, internal, http" />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="create-color">{t("listeners.field_color")}</Label>
                <Select value={createForm.color} onValueChange={(val) => setCreateForm({ ...createForm, color: val ?? "" })}>
                  <SelectTrigger id="create-color" className="w-full">
                    <SelectValue placeholder={t("listeners.color_none")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t("listeners.color_none")}</SelectItem>
                    <SelectItem value="red">Red</SelectItem>
                    <SelectItem value="orange">Orange</SelectItem>
                    <SelectItem value="yellow">Yellow</SelectItem>
                    <SelectItem value="green">Green</SelectItem>
                    <SelectItem value="blue">Blue</SelectItem>
                    <SelectItem value="purple">Purple</SelectItem>
                    <SelectItem value="pink">Pink</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" type="button" onClick={() => setShowCreate(false)} className="flex-1">{t("listeners.cancel")}</Button>
              <Button type="submit" disabled={creating || !createForm.name || !createForm.port}
                className="flex-1">
                {creating ? t("listeners.creating") : t("listeners.create_listener")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showEdit} onOpenChange={(open) => { setShowEdit(open); if (!open) setEditingListener(null); }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("listeners.edit_dialog_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleEditSave}>
            <div className="space-y-3">
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-name">{t("listeners.field_name")}</Label>
                <Input id="edit-name" value={editForm.name} onChange={(e) => setEditForm({ ...editForm, name: e.target.value })} />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-type">{t("listeners.field_type")}</Label>
                <Select value={editForm.type} onValueChange={(val) => handleEditTypeChange(typeof val === "string" ? val : "http")}>
                  <SelectTrigger id="edit-type" className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="http">HTTP</SelectItem>
                    <SelectItem value="https">HTTPS</SelectItem>
                    <SelectItem value="tcp">TCP</SelectItem>
                    <SelectItem value="dns">DNS</SelectItem>
                    <SelectItem value="smb">SMB</SelectItem>
                    <SelectItem value="icmp">ICMP</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-host">{t("listeners.field_host")}</Label>
                <Input id="edit-host" value={editForm.host} onChange={(e) => setEditForm({ ...editForm, host: e.target.value })} />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-port">{t("listeners.field_port")}</Label>
                <Input id="edit-port" type="number" min="1" max="65535" value={editForm.port} onChange={(e) => setEditForm({ ...editForm, port: e.target.value })} />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-notes">{t("listeners.field_notes")}</Label>
                <Input id="edit-notes" value={editForm.notes} onChange={(e) => setEditForm({ ...editForm, notes: e.target.value })} />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-tags">{t("listeners.field_tags")}</Label>
                <Input id="edit-tags" value={editForm.tags} onChange={(e) => setEditForm({ ...editForm, tags: e.target.value })}
                  placeholder="prod, internal, http" />
              </div>
              <div>
                <Label className="text-xs mb-1" htmlFor="edit-color">{t("listeners.field_color")}</Label>
                <Select value={editForm.color} onValueChange={(val) => setEditForm({ ...editForm, color: val ?? "" })}>
                  <SelectTrigger id="edit-color" className="w-full">
                    <SelectValue placeholder={t("listeners.color_none")} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t("listeners.color_none")}</SelectItem>
                    <SelectItem value="red">Red</SelectItem>
                    <SelectItem value="orange">Orange</SelectItem>
                    <SelectItem value="yellow">Yellow</SelectItem>
                    <SelectItem value="green">Green</SelectItem>
                    <SelectItem value="blue">Blue</SelectItem>
                    <SelectItem value="purple">Purple</SelectItem>
                    <SelectItem value="pink">Pink</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" type="button" onClick={() => { setShowEdit(false); setEditingListener(null); }} className="flex-1">{t("listeners.cancel")}</Button>
              <Button type="submit" className="flex-1">{t("listeners.save")}</Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <ConfirmModal open={!!cfm} title={t("listeners.confirm_title")} message={cfm?.msg || ""} confirmText={t("listeners.confirm_delete_text")} cancelText={t("listeners.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

