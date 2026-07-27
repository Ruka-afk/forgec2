"use client";

import { useState } from "react";
import type { BOFLibraryItem } from "./types";
import { formatBytes } from "./types";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Cpu, Layers, Play, Trash2, Upload, User } from "lucide-react";

interface BOFLibraryTabProps {
  libraryItems: BOFLibraryItem[];
  loading: boolean;
  agents: Array<{ id: string; hostname: string }>;
  onUploadLibrary: (file: File, arch: string, name: string, desc: string, author: string) => void;
  onRunLibrary: (id: number | string, agentId: string, args: string) => void;
  onDeleteLibrary: (id: number | string) => void;
}

export default function BOFLibraryTab({ libraryItems, loading, agents, onUploadLibrary, onRunLibrary, onDeleteLibrary }: BOFLibraryTabProps) {
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
          Upload to Library
        </Button>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <Card className="p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/20 rounded-xl flex items-center justify-center">
            <Layers className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{libraryItems.length}</div>
            <div className="text-xs text-muted-foreground">Library BOFs</div>
          </div>
        </Card>
        <Card className="p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/20 rounded-xl flex items-center justify-center">
            <Cpu className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{libraryItems.filter((i) => i.arch === "x64").length}</div>
            <div className="text-xs text-muted-foreground">x64 BOFs</div>
          </div>
        </Card>
        <Card className="p-4 flex items-center gap-3">
          <div className="w-10 h-10 bg-primary/10 rounded-xl flex items-center justify-center">
            <User className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{new Set(libraryItems.map((i) => i.author).filter(Boolean)).size}</div>
            <div className="text-xs text-muted-foreground">Authors</div>
          </div>
        </Card>
      </div>

      <Card className="overflow-hidden">
        {libraryItems.length > 0 ? (
          <Table>
            <TableHeader>
              <TableRow className="text-left text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                <TableHead className="px-5 py-3">Name</TableHead>
                <TableHead className="px-5 py-3">Description</TableHead>
                <TableHead className="px-5 py-3">Arch</TableHead>
                <TableHead className="px-5 py-3">Author</TableHead>
                <TableHead className="px-5 py-3">Size</TableHead>
                <TableHead className="px-5 py-3">Uploaded</TableHead>
                <TableHead className="px-5 py-3 text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {libraryItems.map((item: BOFLibraryItem) => {
                const itemId = item.id ?? 0;
                return (
                  <TableRow key={String(itemId)}>
                    <TableCell className="px-5 py-3 font-medium text-foreground">{item.name || "Unnamed"}</TableCell>
                    <TableCell className="px-5 py-3 text-muted-foreground max-w-xs truncate">{item.description || "-"}</TableCell>
                    <TableCell className="px-5 py-3">
                      <Badge variant="secondary" className="text-(--font-size-micro-sm) font-mono">{item.arch || "x64"}</Badge>
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
                          className="px-3 py-1.5 text-xs bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400 rounded-lg border border-emerald-200 dark:border-emerald-700 hover:bg-emerald-100 dark:hover:bg-emerald-900/40 transition-colors"
                        >
                          <Play className="w-4 h-4" />Execute
                        </Button>
                        <Button onClick={() => onDeleteLibrary(itemId)} className="px-3 py-1.5 text-xs bg-destructive/10 text-destructive rounded-lg border border-destructive/20 hover:bg-destructive/10 transition-colors">
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
            <p>No BOFs in library</p>
            <p className="text-xs mt-1">Upload BOFs with arch/author metadata</p>
          </div>
        )}
      </Card>

      <Dialog open={showLibUpload} onOpenChange={setShowLibUpload}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Upload to BOF Library</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleLibUpload} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">BOF File (.o)</span>
              <input
                type="file"
                accept=".o"
                onChange={(e) => setLibUploadFile(e.target.files?.[0] || null)}
                required
                aria-label="BOF file"
                className="w-full bg-muted border border-border rounded-xl px-4 h-10 text-sm file:mr-3 file:py-1 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-indigo-50 file:text-indigo-600 dark:file:bg-indigo-900/20 dark:file:text-indigo-400"
              />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Name</span>
              <Input placeholder="BOF name" required value={libUploadName} onChange={(e) => setLibUploadName(e.target.value)} className="w-full h-10" />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Description</span>
              <Input placeholder="Brief description" value={libUploadDesc} onChange={(e) => setLibUploadDesc(e.target.value)} className="w-full h-10" />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Architecture</span>
                <Select value={libUploadArch} onValueChange={(v) => setLibUploadArch(v ?? "")}>
                  <SelectTrigger className="w-full h-10">
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
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Author</span>
                <Input placeholder="Author name" value={libUploadAuthor} onChange={(e) => setLibUploadAuthor(e.target.value)} className="w-full h-10" />
              </div>
            </div>
            <DialogFooter>
              <Button type="submit" className="w-full h-10 rounded-xl text-sm font-medium transition-colors">
                Upload to Library
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showLibRun} onOpenChange={setShowLibRun}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Execute BOF from Library</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleLibExecute} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Agent</span>
              <Select value={libRunAgent} onValueChange={(v) => setLibRunAgent(v ?? "")}>
                <SelectTrigger className="w-full h-10">
                  <SelectValue placeholder="Select Agent..." />
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
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Arguments</span>
              <Input placeholder="BOF arguments" value={libRunArgs} onChange={(e) => setLibRunArgs(e.target.value)} className="w-full h-10 font-mono" />
            </div>
            <DialogFooter>
              <Button type="submit" className="w-full h-10 bg-emerald-600 hover:bg-emerald-700 rounded-xl text-sm text-white font-medium transition-colors">
                Execute
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}
