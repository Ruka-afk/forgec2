"use client";

import { useState, useEffect, useCallback } from "react";
import { useParams } from "next/navigation";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { toast } from "sonner";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Spinner } from "@/components/ui/spinner";
import { PageContainer } from "@/components/ui/page-container";
import { AlertCircle, BadgeInfo, Check, CircleDot, Info, Key, List, Pencil, Plus, PlusCircle, RotateCcw, RotateCw, Trash2, User, UserCheck, UserCog, X } from "lucide-react";
import { Card } from "@/components/ui/card";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useI18n } from "@/lib/i18n";
import { formatTime } from "@/lib/utils";

interface Token {
  id?: number;
  ID?: string;
  Domain?: string;
  domain?: string;
  Username?: string;
  username?: string;
  Integrity?: string;
  integrity?: string;
  Source?: string;
  source?: string;
  PID?: number;
  pid?: number;
  ProcessName?: string;
  process_name?: string;
  Active?: boolean;
  active?: boolean;
  CreatedAt?: string;
  created_at?: string;
  TokenType?: string;
  token_type?: string;
  Protocol?: string;
  protocol?: string;
  Note?: string;
  note?: string;
}

interface Process {
  pid: number;
  name: string;
  user?: string;
}


export default function AgentTokenPage() {
  const { t } = useI18n();
  const params = useParams();
  const agentId = params.id as string;
  const [tokens, setTokens] = useState<Token[]>([]);
  const [processes, setProcesses] = useState<Process[]>([]);
  const [loading, setLoading] = useState(true);
  const [stealPid, setStealPid] = useState("");
  const [makeUser, setMakeUser] = useState("");
  const [makeDomain, setMakeDomain] = useState("");
  const [makePass, setMakePass] = useState("");
  const [activeAction, setActiveAction] = useState<string | null>(null);

  const [whoamiResult, setWhoamiResult] = useState<string | null>(null);
  const [tokenNotes, setTokenNotes] = useState<Record<string, string>>({});
  const [noteTargetId, setNoteTargetId] = useState<string | null>(null);

  const loadTokens = useCallback(async () => {
    try {
      const data = await api.get<{ Tokens?: Token[]; tokens?: Token[] }>(paths.agents.tokenList(agentId));
      setTokens(data.tokens || []);
    } catch { toast.error(t("agents.token_load_failed")); }
  }, [agentId, t]);

  const loadProcesses = useCallback(async () => {
    try {
      const data = await api.post<{ processes?: Process[]; data?: Process[] }>(paths.agents.tokenListProcs(agentId));
      setProcesses(data.processes || data.data || []);
    } catch { toast.error(t("agents.token_load_processes_failed")); }
  }, [agentId, t]);

  useEffect(() => {
    setLoading(true);
    Promise.all([loadTokens(), loadProcesses()]).finally(() => setLoading(false));
  }, [loadTokens, loadProcesses]);

  const handleStealToken = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!stealPid) return;
    setActiveAction("steal");
    try {
      const body = new URLSearchParams();
      body.append("pid", stealPid);
      await api.post(paths.agents.tokenSteal(agentId), Object.fromEntries(body.entries()));
      setStealPid("");
      toast.success(t("agents.token_steal_success"));
      await loadTokens();
    } catch {
      toast.error(t("agents.token_steal_failed"));
    } finally {
      setActiveAction(null);
    }
  };  const handleMakeToken = async (e: React.FormEvent) => {    e.preventDefault();
    if (!makeUser) return;
    setActiveAction("make");
    try {
      const body = new URLSearchParams();      body.append("username", makeUser);
      body.append("domain", makeDomain);
      body.append("password", makePass);
      await api.post(paths.agents.tokenMake(agentId), Object.fromEntries(body.entries()));
      setMakeUser(""); setMakeDomain(""); setMakePass("");
      toast.success(t("agents.token_make_success"));
      await loadTokens();
    } catch {
      toast.error(t("agents.token_make_failed"));    } finally {
      setActiveAction(null);
    }
  };

  const handleRevert = async () => {
    setActiveAction("revert");
    try {
      await api.post(paths.agents.tokenRevert(agentId));
      toast.success(t("agents.token_revert_success"));
      setWhoamiResult(null);
      await loadTokens();
    } catch {
      toast.error(t("agents.token_revert_failed"));    } finally {
      setActiveAction(null);
    }
  };  const handleDrop = async (tokenId: string) => {
    setActiveAction(`drop-${tokenId}`);
    try {
      await api.del(paths.agents.tokenOne(agentId, tokenId));
      toast.success(t("agents.token_drop_success"));
      await loadTokens();
    } catch {
      toast.error(t("agents.token_drop_failed"));
    } finally {
      setActiveAction(null);
    }
  };

  const handleImpersonate = async (tokenId: string) => {
    setActiveAction(`impersonate-${tokenId}`);    try {
      await api.post(paths.agents.tokenImpersonate(agentId, tokenId));
      toast.success(t("agents.token_impersonate"));
      await loadTokens();
    } catch {
      toast.error(t("agents.token_drop_failed"));
    } finally {      setActiveAction(null);    }
  };

  const handleWhoami = async () => {
    setActiveAction("whoami");
    try {
      const data = await api.post(paths.agents.tokenWhoami(agentId));
      const d = data as Record<string, unknown>;
      const user = String(d.user || d.username || d.name || JSON.stringify(data));
      setWhoamiResult(user);
      toast(`Identity: ${d.user || d.username || t("agents.unknown")}`);
    } catch (err) {
      setWhoamiResult(`Error: ${err}`);
      toast.error(t("agents.token_whoami_failed"));    } finally {      setActiveAction(null);    }
  };

  const handleNote = async (tokenId: string) => {
    const noteText = tokenNotes[tokenId];    setActiveAction(`note-${tokenId}`);
    try {
      const body = new URLSearchParams();
      body.append("note", noteText || "");
      await api.post(paths.agents.tokenNote(agentId, tokenId), Object.fromEntries(body.entries()));
      toast.success(noteText ? t("agents.token_notes") : t("agents.token_notes"));
      await loadTokens();
    } catch {
      toast.error(t("agents.token_notes"));
    } finally {
      setActiveAction(null);
      setNoteTargetId(null);
    }  };

  const getIntegrityBadge = (integrity: string) => {
    const variants: Record<string, "destructive" | "default" | "secondary" | "outline"> = {
      System: "destructive",
      High: "destructive",
      Medium: "secondary",
      Low: "outline",
    };
    return <Badge variant={variants[integrity] || "secondary"}>{integrity}</Badge>;
  };

  const getTokenTypeBadge = (source: string, tokenType?: string, protocol?: string) => {
    const type = tokenType || protocol || source;
    const icons: Record<string, React.ReactNode> = {
      steal: <UserCog className="w-3 h-3" />,
      make: <PlusCircle className="w-3 h-3" />,
      named_pipe: <CircleDot className="w-3 h-3" />,
      impersonate: <UserCog className="w-3 h-3" />,
      create: <Key className="w-3 h-3" />,
    };
    const icon = icons[type] || <BadgeInfo className="w-3 h-3" />;
    const labels: Record<string, string> = {
      steal: t("agents.token_src_steal"),
      make: t("agents.token_src_make"),
      named_pipe: t("agents.token_src_named_pipe"),
      impersonate: t("agents.token_src_impersonate"),
      create: t("agents.token_src_create"),
    };
    const label = labels[type] || type;
    return (
      <Badge variant={source === "steal" ? "destructive" : "default"}>
        {icon} {label}
      </Badge>
    );
  };  return (
    <PageContainer
      title={t("agents.token_title")}
      subtitle={t("agents.token_subtitle", { hostname: agentId.substring(0, 12) })}
      icon={<BadgeInfo className="w-4 h-4" />}
      actions={
        <>
          <Button variant="outline" size="sm" onClick={handleWhoami} disabled={activeAction === "whoami"}>
            {activeAction === "whoami" ? (
              <><Spinner size="sm" /> {t("agents.token_checking")}</>
            ) : (
              <><UserCheck className="w-4 h-4" /> {t("agents.token_whoami")}</>
            )}
          </Button>
          <Button variant="outline" size="sm" onClick={() => { setLoading(true); Promise.all([loadTokens(), loadProcesses()]).finally(() => setLoading(false)); }}>
            <RotateCw className="w-4 h-4" /> {t("common.refresh")}
          </Button>
        </>
      }
    >

      {whoamiResult && (
        <div className={`border rounded-lg px-4 py-3 text-sm flex items-center gap-2 ${
          whoamiResult.startsWith("Error")            ? "border-destructive/30 bg-destructive/10 text-destructive"
            : "border-primary/20 bg-primary/10 dark:border-primary/40 dark:bg-primary/20 text-primary"
        }`}>
          {whoamiResult.startsWith("Error") ? <AlertCircle className="w-4 h-4" /> : <UserCheck className="w-4 h-4" />}
          <span>{t("agents.token_current_context")} <strong>{whoamiResult}</strong></span>
          <Button variant="ghost" size="sm" onClick={() => setWhoamiResult(null)} className="ml-auto opacity-60 hover:opacity-100">
            <X className="w-4 h-4" />
          </Button>
        </div>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="p-5">
          <h2 className="text-sm font-semibold text-foreground mb-3">            <User className="w-4 h-4" />{t("agents.token_steal")}
          </h2>
          <form onSubmit={handleStealToken} className="space-y-3">
            <div>
              <Label htmlFor="steal-pid" className="text-xs text-muted-foreground mb-1 block">{t("agents.token_pid")}</Label>
              <Select value={stealPid} onValueChange={(v) => setStealPid(v ?? "")}>
                <SelectTrigger id="steal-pid" className="w-full">
                  <SelectValue placeholder={t("agents.token_steal")} />
                </SelectTrigger>
                <SelectContent>
                  {processes.map((p) => (
                    <SelectItem key={p.pid} value={String(p.pid)}>[{p.pid}] {p.name}{p.user ? ` (${p.user})` : ""}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <Button type="submit" disabled={!stealPid || activeAction === "steal"} className="w-full" variant="default">
              {activeAction === "steal" ? <><Spinner size="sm" />{t("agents.token_stealing")}</> : <><User className="w-4 h-4" />{t("agents.token_steal")}</>}
            </Button>
          </form>
        </Card>

        <Card className="p-5">          <h2 className="text-sm font-semibold text-foreground mb-3">            <PlusCircle className="w-4 h-4" />{t("agents.token_make")}
          </h2>
          <form onSubmit={handleMakeToken} className="space-y-3">
            <div>
              <Label htmlFor="make-user" className="text-xs text-muted-foreground mb-1 block">{t("agents.token_username")} <span className="text-destructive">*</span></Label>
              <Input id="make-user" type="text" value={makeUser} onChange={(e) => setMakeUser(e.target.value)} placeholder={t("agents.token_username")} />
            </div>
            <div>
              <Label htmlFor="make-domain" className="text-xs text-muted-foreground mb-1 block">{t("agents.token_domain")}</Label>
              <Input id="make-domain" type="text" value={makeDomain} onChange={(e) => setMakeDomain(e.target.value)} placeholder={t("agents.token_domain")} />
            </div>
            <div>
              <Label htmlFor="make-pass" className="text-xs text-muted-foreground mb-1 block">{t("agents.token_password")}</Label>
              <Input id="make-pass" type="password" value={makePass} onChange={(e) => setMakePass(e.target.value)} placeholder={t("agents.token_password")} />
            </div>
            <Button type="submit" disabled={!makeUser || activeAction === "make"} className="w-full" variant="default">
              {activeAction === "make" ? <><Spinner size="sm" />{t("agents.token_creating")}</> : <><Plus className="w-4 h-4" />{t("agents.token_make")}</>}
            </Button>
          </form>
        </Card>        <Card className="p-5">
          <h2 className="text-sm font-semibold text-foreground mb-3">
            <RotateCcw className="w-4 h-4" />{t("agents.token_quick_actions")}          </h2>
          <div className="space-y-3">
            <Button onClick={handleRevert} disabled={activeAction === "revert"} className="w-full" variant="default">
              {activeAction === "revert" ? <><Spinner size="sm" />{t("agents.token_reverting")}</> : <><RotateCcw className="w-4 h-4" />{t("agents.token_revert")}</>}
            </Button>
            <Button onClick={handleWhoami} disabled={activeAction === "whoami"} className="w-full" variant="secondary">
              {activeAction === "whoami" ? <><Spinner size="sm" />{t("agents.token_querying")}</> : <><UserCheck className="w-4 h-4" />{t("agents.token_whoami")}</>}
            </Button>
            <div className="text-xs text-muted-foreground bg-muted/50 rounded-lg p-3 flex items-start gap-2">
              <Info className="w-4 h-4" />
              <span>{t("agents.token_impersonate_hint")}</span>
            </div>
          </div>        </Card>
      </div>

      <Card className="overflow-hidden">
        <div className="px-5 py-3 border-b border-border flex items-center justify-between">
          <h2 className="text-sm font-semibold text-foreground">
            <List className="w-4 h-4" />{t("agents.token_title")} ({tokens.length})
          </h2>          <div className="flex items-center gap-2">
            <div className="text-xs text-muted-foreground">
              <span className="inline-flex items-center gap-1.5">
                <span className="w-2 h-2 bg-warning rounded-full animate-pulse"></span>                {tokens.filter(t => t.active).length} active
              </span>
            </div>
          </div>
        </div>
        <div className="overflow-x-auto">
          <Table className="w-full text-sm">
            <TableHeader>
              <TableRow className="text-xs text-muted-foreground">
                <TableHead className="text-left px-5 py-3 font-normal">{t("agents.token_username")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_integrity")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_type")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_source")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_process")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_status")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_notes")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_created")}</TableHead>
                <TableHead className="text-left px-4 py-3 font-normal">{t("agents.token_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody className="divide-y divide-border">
              {!loading && tokens.map((tok) => {
                const tid = tok.id ? String(tok.id) : "";
                const domain = tok.domain || "";
                const username = tok.username || "";
                const integrity = tok.integrity || "Medium";
                const source = tok.source || "steal";
                const tokenType = tok.token_type;
                const protocol = tok.protocol;
                const pid = tok.pid;
                const procName = tok.process_name;
                const active = tok.Active !== undefined ? tok.Active : tok.active;
                const createdAt = tok.created_at || "";
                const noteText = tok.note || tokenNotes[tid] || "";
                const isEditingNote = noteTargetId === tid;
                return (
                  <TableRow key={tid} className={`hover:bg-muted/50 transition-colors ${active ? "bg-warning/10" : ""}`}>
                    <TableCell className="px-5 py-3">
                      <span className="font-semibold text-foreground text-sm">{domain ? `${domain}\\${username}` : username || "Unknown"}</span>
                    </TableCell>
                    <TableCell className="px-4 py-3">{getIntegrityBadge(integrity)}</TableCell>
                    <TableCell className="px-4 py-3">{getTokenTypeBadge(source, tokenType, protocol)}</TableCell>
                    <TableCell className="px-4 py-3">
                      <Badge variant={source === "steal" ? "destructive" : source === "make" || source === "create" ? "default" : "secondary"}>
                        {source}
                      </Badge>
                    </TableCell>
                    <TableCell className="px-4 py-3 text-xs font-mono text-muted-foreground">{pid ? `[${pid}]` : ""} {procName || ""}</TableCell>
                    <TableCell className="px-4 py-3">
                      {active ? (
                        <Badge variant="secondary" className="text-xs gap-1.5"><span className="w-2 h-2 bg-warning rounded-full animate-pulse"></span>{t("agents.token_active")}</Badge>
                      ) : (
                        <Badge variant="secondary" className="text-xs gap-1.5"><span className="w-2 h-2 bg-muted rounded-full"></span>{t("agents.token_inactive")}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      {isEditingNote ? (
                        <div className="flex items-center gap-1">
                          <Input
                            type="text"
                            value={tokenNotes[tid] || ""}
                            onChange={(e) => setTokenNotes(prev => ({ ...prev, [tid]: e.target.value }))}
                            className="w-24 h-7 text-xs"
                            placeholder={t("agents.token_note_ph")}
                            autoFocus
                          />
                          <Button variant="ghost" size="sm" onClick={() => handleNote(tid)} className="h-7 w-7 p-0 text-success hover:text-success">
                            <Check className="w-4 h-4" />
                          </Button>
                          <Button variant="ghost" size="sm" onClick={() => setNoteTargetId(null)} className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground">
                            <X className="w-4 h-4" />
                          </Button>
                        </div>
                      ) : (
                        <Tooltip>
                          <TooltipTrigger render={<Button
                              variant="ghost"
                              size="sm"
                              onClick={() => { setNoteTargetId(tid); setTokenNotes(prev => ({ ...prev, [tid]: noteText })); }}
                              className="text-xs text-muted-foreground hover:text-primary"
                            />}>
                              <Pencil className="w-4 h-4" />
                              {noteText ? <span className="truncate max-w-20">{noteText}</span> : <span className="text-muted-foreground">{t("agents.token_add_note")}</span>}
                          </TooltipTrigger>
                          <TooltipContent>{t("agents.token_edit_note")}</TooltipContent>
                        </Tooltip>
                      )}
                    </TableCell>
                    <TableCell className="px-4 py-3 text-xs font-mono text-muted-foreground">{createdAt ? formatTime(createdAt) : ""}</TableCell>
                    <TableCell className="px-4 py-3">
                      <div className="flex items-center gap-1">
                        <Tooltip>
                          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => handleImpersonate(tid)} disabled={!active || activeAction === `impersonate-${tid}`} className={`h-8 w-8 p-0 ${active ? "" : "cursor-not-allowed"}`} />}>
                            {activeAction === `impersonate-${tid}` ? <Spinner size="sm" /> : <User className="w-4 h-4" />}
                          </TooltipTrigger>
                          <TooltipContent>{t("agents.token_impersonate")}</TooltipContent>
                        </Tooltip>
                        <Tooltip>
                          <TooltipTrigger render={<Button variant="ghost" size="sm" onClick={() => handleDrop(tid)} disabled={activeAction === `drop-${tid}`} className={`h-8 w-8 p-0 text-destructive hover:text-destructive/80 hover:bg-destructive/5`} />}>
                            {activeAction === `drop-${tid}` ? <Spinner size="xs" /> : <Trash2 className="w-3 h-3" />}
                          </TooltipTrigger>
                          <TooltipContent>{t("agents.token_drop")}</TooltipContent>
                        </Tooltip>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
              {loading && Array.from({ length: 3 }).map((_, i) => (<TableRow key={i}><TableCell colSpan={8} className="py-3 px-4"><Skeleton className="h-8 w-full" /></TableCell></TableRow>))}
              {!loading && tokens.length === 0 && (<TableRow><TableCell colSpan={8} className="py-16 text-center text-muted-foreground"><BadgeInfo className="w-4 h-4" /><p className="text-sm">{t("agents.token_no_tokens")}</p></TableCell></TableRow>)}
            </TableBody>
          </Table>
        </div>      </Card>
    </PageContainer>
  );
}
