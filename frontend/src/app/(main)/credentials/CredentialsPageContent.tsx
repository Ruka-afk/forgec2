"use client";

import { toast } from "sonner";
import { useState, useMemo, useCallback } from "react";
import { api } from "@/lib/api";
import { downloadText } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { PageHeader } from "@/components/UI";
import { DataState } from "@/components/ui/data-state";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Badge } from "@/components/ui/badge";
import { Checkbox } from "@/components/ui/checkbox";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Label } from "@/components/ui/label";
import { AlertTriangle, Download, Filter, Lock, Plus, Tag } from "lucide-react";
import { CRED_TYPES, TYPE_BADGE_VARIANT, type VaultEntry } from "./_components/types";
import { useCredentialsData } from "./_components/useCredentialsData";
import { CredentialRow } from "./_components/CredentialRow";

export default function CredentialsPage() {
  const { t } = useI18n();
  const { data, loading, error, reload, loadData } = useCredentialsData();
  const [searchQuery, setSearchQuery] = useState("");
  const [typeFilter, setTypeFilter] = useState("all");
  const [confirmedFilter, setConfirmedFilter] = useState("");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [showAddModal, setShowAddModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showBatchModal, setShowBatchModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<string | null>(null);
  const [editTarget, setEditTarget] = useState<VaultEntry | null>(null);

  const [showPasswords, setShowPasswords] = useState<Set<string>>(new Set());

  const [form, setForm] = useState({
    domain: "",
    username: "",
    password: "",
    hash: "",
    type: "cleartext",
    source: "",
    tags: "",
    notes: "",
  });

  const [batchTags, setBatchTags] = useState("");

  const showToastNotify = (text: string, type: "success" | "error" | "info" = "info") => {
    if (type === "success") toast.success(text);
    else if (type === "error") toast.error(text);
    else toast.info(text);
  };

  const handleAdd = async () => {
    if (!form.username) return showToastNotify(t("cred.toast.username_required"), "error");
    try {
      await api.post("/credentials/add", {
        domain: form.domain,
        username: form.username,
        password: form.password,
        hash: form.hash,
        type: form.type,
        source: form.source || "manual",
        tags: form.tags,
        notes: form.notes,
      });
      showToastNotify(t("cred.toast.added"), "success");
      setShowAddModal(false);
      resetForm();
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    try {
      const body: Record<string, string> = {};
      if (form.tags) body.tags = form.tags;
      if (form.notes) body.notes = form.notes;
      await api.put(`/credentials/${editTarget.id}`, body);
      showToastNotify(t("cred.toast.updated"), "success");
      setShowEditModal(false);
      setEditTarget(null);
      resetForm();
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const requestDelete = useCallback((id: string) => setShowDeleteConfirm(id), []);

  const handleDelete = useCallback(async (id: string) => {
    try {
      await api.del(`/credentials/${id}`);
      showToastNotify(t("cred.toast.deleted"), "success");
      setShowDeleteConfirm(null);
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  }, [t, loadData]);

  const handleToggleConfirm = useCallback(async (entry: VaultEntry) => {
    try {
      await api.post(`/credentials/${entry.id}/confirm`);
      loadData();
    } catch { toast.error(t("cred.toast.confirm_failed")); }
  }, [loadData, t]);

  const handleBatchTags = async () => {
    if (!batchTags || selectedIds.size === 0) return;
    try {
      await api.postJson("/credentials/batch/tags", {
        ids: Array.from(selectedIds).map((id) => Number(id)),
        tags: batchTags.split(",").map((tag) => tag.trim()).filter(Boolean),
      });
      showToastNotify(t("cred.toast.tags_added", { count: selectedIds.size }), "success");
      setShowBatchModal(false);
      setBatchTags("");
      setSelectedIds(new Set());
      loadData();
    } catch (err) {
      showToastNotify(String(err), "error");
    }
  };

  const exportCSV = () => {
    const headers = ["Type", "Domain", "Username", "Password", "Hash", "Source", "Tags", "Confirmed", "Notes"];
    const rows = filteredEntries.map(e => [
      e.type,
      e.domain || "",
      e.username,
      e.password || "",
      e.hash || "",
      e.source || "",
      e.tags || "",
      e.confirmed ? "Yes" : "No",
      e.notes || "",
    ]);
    const csv = [headers, ...rows].map(r => r.map(c => `"${String(c).replace(/"/g, '""')}"`).join(",")).join("\n");
    downloadText(csv, `credentials_${new Date().toISOString().slice(0, 10)}.csv`, "text/csv");
    showToastNotify(t("cred.toast.csv_exported"), "success");
  };

  const openEdit = useCallback((entry: VaultEntry) => {
    setEditTarget(entry);
    setForm({
      domain: entry.domain || "",
      username: entry.username || "",
      password: entry.password || "",
      hash: entry.hash || "",
      type: entry.type || "cleartext",
      source: entry.source || "",
      tags: entry.tags || "",
      notes: entry.notes || "",
    });
    setShowEditModal(true);
  }, []);

  const resetForm = () => {
    setForm({
      domain: "",
      username: "",
      password: "",
      hash: "",
      type: "cleartext",
      source: "",
      tags: "",
      notes: "",
    });
  };

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredEntries.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(filteredEntries.map(e => e.id)));
    }
  };

  const togglePasswordVisibility = useCallback((id: string) => {
    setShowPasswords(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const entries = useMemo(() => data?.VaultEntries || [], [data?.VaultEntries]);

  const filteredEntries = entries.filter(entry => {
    if (searchQuery) {
      const q = searchQuery.toLowerCase();
      if (
        !entry.username.toLowerCase().includes(q) &&
        !entry.domain?.toLowerCase().includes(q) &&
        !entry.notes?.toLowerCase().includes(q)
      ) return false;
    }
    if (typeFilter !== "all" && entry.type !== typeFilter) return false;
    if (confirmedFilter === "true" && !entry.confirmed) return false;
    if (confirmedFilter === "false" && entry.confirmed) return false;
    return true;
  });

  const stats = useMemo(() => ({
    total: data?.VaultCount || 0,
    confirmed: entries.filter(e => e.confirmed).length || 0,
    unconfirmed: entries.filter(e => !e.confirmed).length || 0,
    byType: CRED_TYPES.slice(1).map(ct => ({
      type: ct,
      count: entries.filter(e => e.type === ct).length || 0,
    })),
  }), [entries, data?.VaultCount]);

  return (
    <>
      <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("cred.title")} subtitle={t("cred.subtitle")}>
        {filteredEntries.length > 0 && (
          <Button
            onClick={exportCSV}
            size="lg"
            className="gap-x-2"
          >
            <Download className="w-4 h-4" />
            <span>{t("cred.export_csv")}</span>
          </Button>
        )}
        <Button
          onClick={() => { resetForm(); setShowAddModal(true); }}
          size="lg"
          className="gap-x-2"
        >
          <Plus className="w-4 h-4" />
          <span>{t("cred.add")}</span>
        </Button>
      </PageHeader>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="text-2xl font-bold">{stats.total}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("cred.stat_total")}</div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="text-2xl font-bold text-primary dark:text-emerald-400">{stats.confirmed}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("cred.stat_confirmed")}</div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="text-2xl font-bold text-amber-600 dark:text-amber-400">{stats.unconfirmed}</div>
          <div className="text-xs text-muted-foreground mt-1">{t("cred.stat_unconfirmed")}</div>
        </Card>
        <Card className="p-4 sm:p-5 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-lg dark:hover:shadow-black/30">
          <div className="flex flex-wrap gap-1 mt-1">
            {stats.byType.map(s => s.count > 0 && (
              <Badge key={s.type} variant={TYPE_BADGE_VARIANT[s.type] || "outline"}>
                {s.type}: {s.count}
              </Badge>
            ))}
          </div>
          <div className="text-xs text-muted-foreground mt-1">{t("cred.stat_by_type")}</div>
        </Card>
      </div>

      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex flex-wrap items-center gap-3">
          <Input
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            placeholder={t("cred.search_placeholder")}
            className="flex-1 min-w-[200px]"
          />
          <Select value={typeFilter} onValueChange={(v) => setTypeFilter(v ?? "")}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {CRED_TYPES.map(ct => (
                <SelectItem key={ct} value={ct}>{ct === "all" ? t("cred.filter_all_types") : ct.charAt(0).toUpperCase() + ct.slice(1)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={confirmedFilter} onValueChange={(v) => setConfirmedFilter(v ?? "")}>
            <SelectTrigger className="w-[140px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="">{t("cred.filter_all_status")}</SelectItem>
              <SelectItem value="true">{t("cred.filter_confirmed")}</SelectItem>
              <SelectItem value="false">{t("cred.filter_unconfirmed")}</SelectItem>
            </SelectContent>
          </Select>
          <Button
            onClick={() => reload()}
            size="lg"
          >
            <Filter className="w-4 h-4" />{t("cred.filter")}
          </Button>
          <Button
            onClick={() => { setSearchQuery(""); setTypeFilter("all"); setConfirmedFilter(""); }}
            variant="outline"
            size="lg"
          >
            {t("cred.clear")}
          </Button>
          {selectedIds.size > 0 && (
            <Button
              onClick={() => setShowBatchModal(true)}
              size="lg"
              className="gap-x-2"
            >
              <Tag className="w-4 h-4" />
              <span>{t("cred.batch_tags")} ({selectedIds.size})</span>
            </Button>
          )}
        </div>
      </Card>

      {data?.AllTags && data.AllTags.length > 0 && (
        <Card className="p-4 sm:p-5 mb-6">
          <div className="font-medium text-sm text-foreground flex items-center gap-2 mb-3">
            <Tag className="w-4 h-4" />
            <span>{t("cred.tags")}</span>
          </div>
          <div className="flex flex-wrap gap-2">
            {data.AllTags.map(tag => (
              <Badge
                key={tag}
                variant="outline"
                className="cursor-pointer"
                onClick={() => setSearchQuery(tag)}
              >
                {tag}
              </Badge>
            ))}
          </div>
        </Card>
      )}

      <Card className="overflow-hidden mb-8">
        <div className="px-6 py-4 border-b border-border flex items-center justify-between">
          <div className="font-semibold text-foreground flex items-center gap-x-2">
            <Lock className="w-4 h-4" />
            <span>{t("cred.vault_title")}</span>
            {filteredEntries.length > 0 && (
              <Badge variant="outline" className="font-mono">
                {filteredEntries.length}
              </Badge>
            )}
          </div>
        </div>

        <DataState
          loading={loading}
          error={error}
          empty={filteredEntries.length === 0}
          emptyIcon={Lock}
          emptyTitle={t("cred.empty")}
          onRetry={reload}
          loadingSkeleton={
            <div className="p-4 sm:p-5 text-center text-muted-foreground">
              <div className="flex flex-col items-center gap-2">
                <Skeleton className="h-8 w-8 rounded-full" />
                <Skeleton className="h-4 w-20" />
              </div>
            </div>
          }
        >
          <div className="overflow-x-auto">
            <Table className="w-full text-sm">
              <TableHeader className="bg-muted/50 border-b border-border">
                <TableRow className="text-xs text-muted-foreground">
                  <TableHead className="text-left py-3 px-2 font-normal">
                    <Checkbox
                      checked={selectedIds.size === filteredEntries.length && filteredEntries.length > 0}
                      onCheckedChange={() => toggleSelectAll()}
                    />
                  </TableHead>
                  <TableHead className="text-left py-3 px-4 font-normal">{t("cred.col_type")}</TableHead>
                  <TableHead className="text-left py-3 px-4 font-normal">{t("cred.col_username")}</TableHead>
                  <TableHead className="text-left py-3 px-4 font-normal">{t("cred.col_password")}</TableHead>
                  <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal">{t("cred.col_domain")}</TableHead>
                  <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal">{t("cred.col_source")}</TableHead>
                  <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal">{t("cred.col_confirmed")}</TableHead>
                  <TableHead className="max-sm:hidden text-left py-3 px-4 font-normal">{t("cred.col_tags")}</TableHead>
                  <TableHead className="text-center py-3 px-4 font-normal">{t("cred.col_actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody className="divide-y divide-border">
                {filteredEntries.map(entry => (
                  <CredentialRow
                    key={entry.id}
                    entry={entry}
                    isSelected={selectedIds.has(entry.id)}
                    showPassword={showPasswords.has(entry.id)}
                    onToggleSelect={toggleSelect}
                    onToggleConfirm={handleToggleConfirm}
                    onEdit={openEdit}
                    onDelete={requestDelete}
                    togglePasswordVisibility={togglePasswordVisibility}
                    t={t}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        </DataState>
      </Card>

      <Dialog open={showAddModal} onOpenChange={setShowAddModal}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("cred.add_title")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_type")} *</Label>
                <Select value={form.type} onValueChange={(v) => setForm({ ...form, type: v ?? "cleartext" })}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CRED_TYPES.filter(ct => ct !== "all").map(ct => (
                      <SelectItem key={ct} value={ct}>{ct.charAt(0).toUpperCase() + ct.slice(1)}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_username")} *</Label>
                <Input
                  value={form.username}
                  onChange={e => setForm({ ...form, username: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_password")}</Label>
                <Input
                  value={form.password}
                  onChange={e => setForm({ ...form, password: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_domain")}</Label>
                <Input
                  value={form.domain}
                  onChange={e => setForm({ ...form, domain: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_source")}</Label>
                <Input
                  value={form.source}
                  onChange={e => setForm({ ...form, source: e.target.value })}
                  placeholder={t("cred.ph_source")}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_hash")}</Label>
                <Input
                  value={form.hash}
                  onChange={e => setForm({ ...form, hash: e.target.value })}
                  className="font-mono text-xs"
                />
              </div>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_tags")}</Label>
              <Input
                value={form.tags}
                onChange={e => setForm({ ...form, tags: e.target.value })}
                placeholder={t("cred.ph_tags")}
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_notes")}</Label>
              <Textarea
                value={form.notes}
                onChange={e => setForm({ ...form, notes: e.target.value })}
                rows={2}
                className="resize-none"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowAddModal(false)} className="flex-1">
              {t("common.cancel")}
            </Button>
            <Button onClick={handleAdd} className="flex-1">
              {t("cred.btn.add")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showEditModal} onOpenChange={(open) => { if (!open) { setShowEditModal(false); setEditTarget(null); } }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>{t("cred.edit_title")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_type")}</Label>
                <Select value={form.type} onValueChange={(v) => setForm({ ...form, type: v ?? "cleartext" })}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {CRED_TYPES.filter(ct => ct !== "all").map(ct => (
                      <SelectItem key={ct} value={ct}>{ct}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_username")} *</Label>
                <Input
                  value={form.username}
                  onChange={e => setForm({ ...form, username: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_password")}</Label>
                <Input
                  value={form.password}
                  onChange={e => setForm({ ...form, password: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_domain")}</Label>
                <Input
                  value={form.domain}
                  onChange={e => setForm({ ...form, domain: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_source")}</Label>
                <Input
                  value={form.source}
                  onChange={e => setForm({ ...form, source: e.target.value })}
                />
              </div>
              <div>
                <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_hash")}</Label>
                <Input
                  value={form.hash}
                  onChange={e => setForm({ ...form, hash: e.target.value })}
                  className="font-mono text-xs"
                />
              </div>
            </div>
            <div>
              <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_tags")}</Label>
              <Input
                value={form.tags}
                onChange={e => setForm({ ...form, tags: e.target.value })}
              />
            </div>
            <div>
              <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_notes")}</Label>
              <Textarea
                value={form.notes}
                onChange={e => setForm({ ...form, notes: e.target.value })}
                rows={2}
                className="resize-none"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => { setShowEditModal(false); setEditTarget(null); }} className="flex-1">
              {t("common.cancel")}
            </Button>
            <Button onClick={handleEdit} className="flex-1">
              {t("common.save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={showBatchModal} onOpenChange={setShowBatchModal}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("cred.batch_title")}</DialogTitle>
          </DialogHeader>
          <p className="text-sm text-muted-foreground">
            {t("cred.batch_desc", { count: selectedIds.size })}
          </p>
          <div>
            <Label className="text-xs text-muted-foreground block mb-1">{t("cred.field_tags")}</Label>
            <Input
              value={batchTags}
              onChange={e => setBatchTags(e.target.value)}
              placeholder="high-value, production, dc"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowBatchModal(false)} className="flex-1">
              {t("common.cancel")}
            </Button>
            <Button onClick={handleBatchTags} className="flex-1">
              {t("cred.batch_btn")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={!!showDeleteConfirm} onOpenChange={(open) => { if (!open) setShowDeleteConfirm(null); }}>
        <DialogContent className="sm:max-w-sm">
          <div className="flex flex-col items-center gap-2 py-4">
            <AlertTriangle className="w-4 h-4" />
            <DialogTitle>{t("cred.delete_title")}</DialogTitle>
            <p className="text-sm text-muted-foreground">{t("cred.delete_message")}</p>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setShowDeleteConfirm(null)} className="flex-1">
              {t("common.cancel")}
            </Button>
            <Button variant="destructive" onClick={() => showDeleteConfirm && handleDelete(showDeleteConfirm)} className="flex-1">
              {t("common.delete")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      </div>
    </>
  );
}
