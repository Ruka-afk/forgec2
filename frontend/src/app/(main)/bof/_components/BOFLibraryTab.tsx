"use client";

import { useState } from "react";
import type { BOFLibraryItem } from "./types";
import { formatBytes } from "./types";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Cpu, Layers, Play, Trash2, Upload, User } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface BOFLibraryTabProps {
  libraryItems: BOFLibraryItem[];
  loading: boolean;
  agents: Array<{ id: string; hostname: string }>;
  onUploadLibrary: (file: File, arch: string, name: string, desc: string, author: string) => void;
  onRunLibrary: (id: number | string, agentId: string, args: string) => void;
  onDeleteLibrary: (id: number | string) => void;
}

export default function BOFLibraryTab({ libraryItems, loading, agents, onUploadLibrary, onRunLibrary, onDeleteLibrary }: BOFLibraryTabProps) {
  const { t } = useI18n();
  const [showLibUpload, setShowLibUpload] = useState(false);
  const [showLibRun, setShowLibRun] = useState(false);
  const [libUploadName, setLibUploadName] = useState("");
  const [libUploadDesc, setLibUploadDesc] = useState("");
  const [libUploadArch, setLibUploadArch] = useState("x64");
  const [libUploadAuthor, setLibUploadAuthor] = useState("");
  const [libUploadFile, setLibUploadFile] = useState<File | null>(null);
  const [libRunId, setLibRunId] = useState<number | string>(0);
  const [libRunAgent, setLibRunAgent] = useState("");
  const [libRunArgs, setLibRunArgs] = useState("");

  const handleLibUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!libUploadFile) return;
    onUploadLibrary(libUploadFile, libUploadArch, libUploadName, libUploadDesc, libUploadAuthor);
    setShowLibUpload(false);
    setLibUploadFile(null);
    setLibUploadName("");
    setLibUploadDesc("");
    setLibUploadArch("x64");
    setLibUploadAuthor("");
  };

  const handleLibExecute = (e: React.FormEvent) => {
    e.preventDefault();
    if (!libRunAgent) return;
    onRunLibrary(libRunId, libRunAgent, libRunArgs);
    setShowLibRun(false);
  };

  if (loading) {
    return (
      <Card className="overflow-hidden">
        <div className="text-center py-12 text-muted-foreground">
          <Spinner />
        </div>
      </Card>
    );
  }

  return (
    <>
      <div className="flex justify-end mb-4">
        <Button onClick={() => setShowLibUpload(true)}>
          <Upload className="w-4 h-4" />
          {t("bof.upload_library_title")}
        </Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <IconBadge icon={Layers} color="primary" size="xl" className="dark:bg-primary/15" />
          <div>
            <div className="text-xl font-bold text-foreground">{libraryItems.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.library_bofs")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <IconBadge icon={Cpu} color="success" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{libraryItems.filter((i) => i.arch === "x64").length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_x64")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <IconBadge icon={User} color="primary" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{new Set(libraryItems.map((i) => i.author).filter(Boolean)).size}</div>
            <div className="text-xs text-muted-foreground">{t("bof.authors")}</div>
          </div>
        </Card>
      </div>

      <Card className="overflow-hidden">
        {libraryItems.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow className="text-left text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                <TableHead className="px-5 py-3">{t("bof.col_name")}</TableHead>
                <TableHead className="px-5 py-3">{t("bof.col_description")}</TableHead>
                <TableHead className="px-5 py-3">{t("bof.col_arch")}</TableHead>
                <TableHead className="px-5 py-3">{t("bof.col_author")}</TableHead>
                <TableHead className="px-5 py-3">{t("bof.col_size")}</TableHead>
                <TableHead className="px-5 py-3">{t("bof.col_uploaded")}</TableHead>
                <TableHead className="px-5 py-3 text-right">{t("bof.col_actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {libraryItems.map((item: BOFLibraryItem) => {
                const itemId = item.id ?? 0;
                return (
                  <TableRow key={String(itemId)}>
                    <TableCell className="px-5 py-3 font-medium text-foreground">{item.name || t("bof.unnamed")}</TableCell>
                    <TableCell className="px-5 py-3 text-muted-foreground max-w-xs truncate">{item.description || "-"}</TableCell>
                    <TableCell className="px-5 py-3">
                      <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono">{item.arch || "x64"}</Badge>
                    </TableCell>
                    <TableCell className="px-5 py-3 text-muted-foreground">{item.author || "-"}</TableCell>
                    <TableCell className="px-5 py-3 text-muted-foreground">{formatBytes(item.size)}</TableCell>
                    <TableCell className="px-5 py-3 text-muted-foreground text-xs">{item.created_at || ""}</TableCell>
                    <TableCell className="px-5 py-3 text-right">
                      <div className="flex gap-2 justify-end">
                        <Button
                          onClick={() => {
                            setLibRunId(itemId);
                            setLibRunAgent(agents[0]?.id || "");
                            setLibRunArgs("");
                            setShowLibRun(true);
                          }}
                          className="px-3 py-1.5 text-xs bg-success/15 text-success rounded-lg border border-success/30 hover:bg-success/15 transition-colors"
                        >
                          <Play className="w-4 h-4" />{t("bof.run")}
                        </Button>
                        <Button onClick={() => onDeleteLibrary(itemId)} aria-label={t("common.delete")} className="px-3 py-1.5 text-xs bg-destructive/10 text-destructive rounded-lg border border-destructive/20 hover:bg-destructive/10 transition-colors">
                          <Trash2 className="w-4 h-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        ) : (
          <div className="text-center py-12 text-muted-foreground">
            <Layers className="w-4 h-4" />
            <p>{t("bof.no_files")}</p>
            <p className="text-xs mt-1">{t("bof.no_files_lib_hint")}</p>
          </div>
        )}
      </Card>

      <Dialog open={showLibUpload} onOpenChange={setShowLibUpload}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("bof.upload_library_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleLibUpload} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.bof_file")} (.o)</span>
              <Input
                type="file"
                accept=".o"
                onChange={(e) => setLibUploadFile(e.target.files?.[0] || null)}
                required
                aria-label={t("bof.bof_file")}
                className="h-9 bg-muted"
              />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.name")}</span>
              <Input aria-label={t("bof.bof_name")} placeholder={t("bof.bof_name")} required value={libUploadName} onChange={(e) => setLibUploadName(e.target.value)} className="w-full h-9" />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.description")}</span>
              <Input aria-label={t("bof.description")} placeholder={t("bof.brief_desc")} value={libUploadDesc} onChange={(e) => setLibUploadDesc(e.target.value)} className="w-full h-9" />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.architecture")}</span>
                <Select value={libUploadArch} onValueChange={(v) => setLibUploadArch(v ?? "")}>
                  <SelectTrigger aria-label={t("bof.architecture")} className="w-full h-9">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="x64">x64</SelectItem>
                    <SelectItem value="x86">x86</SelectItem>
                    <SelectItem value="arm64">ARM64</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.author")}</span>
                <Input aria-label={t("bof.author")} placeholder={t("bof.author_placeholder")} value={libUploadAuthor} onChange={(e) => setLibUploadAuthor(e.target.value)} className="w-full h-9" />
              </div>
            </div>
            <DialogFooter>
              <Button type="submit" size="lg" className="w-full text-sm font-medium transition-colors">
                {t("bof.upload_library_title")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showLibRun} onOpenChange={setShowLibRun}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("bof.execute_library_title")}</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleLibExecute} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.agent")}</span>
              <Select value={libRunAgent} onValueChange={(v) => setLibRunAgent(v ?? "")}>
                <SelectTrigger aria-label={t("bof.agent")} className="w-full h-9">
                  <SelectValue placeholder={t("bof.select_agent")} />
                </SelectTrigger>
                <SelectContent>
                  {agents.map((a: { id: string; hostname: string }) => (
                    <SelectItem key={a.id} value={a.id}>
                      {a.hostname}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.arguments")}</span>
              <Input aria-label={t("bof.bof_args")} placeholder={t("bof.bof_args")} value={libRunArgs} onChange={(e) => setLibRunArgs(e.target.value)} className="w-full h-9 font-mono" />
            </div>
            <DialogFooter>
              <Button type="submit" size="lg" className="w-full text-sm font-medium transition-colors">
                {t("bof.run")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
