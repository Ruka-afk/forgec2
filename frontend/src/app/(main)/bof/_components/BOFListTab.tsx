"use client";

import { useState } from "react";
import type { BOFFile } from "./types";
import { formatBytes } from "./types";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Box, Info, Pencil, Play, Trash2, Upload } from "lucide-react";

interface BOFListTabProps {
  files: BOFFile[];
  loading: boolean;
  onUpload: (file: File, arch: string, name: string, desc: string) => void;
  onDelete: (id: number) => void;
  onRun: (id: number, agentId: string, args: string) => void;
  onEdit: (id: string, name: string, description: string) => void;
  agents: Array<{ id: string; hostname: string }>;
}

export default function BOFListTab({ files, loading, onUpload, onDelete, onRun, onEdit, agents }: BOFListTabProps) {
  const [showUpload, setShowUpload] = useState(false);
  const [showRun, setShowRun] = useState(false);
  const [showInfo, setShowInfo] = useState<BOFFile | null>(null);
  const [editTarget, setEditTarget] = useState<BOFFile | null>(null);
  const [uploadName, setUploadName] = useState("");
  const [uploadDesc, setUploadDesc] = useState("");
  const [uploadArch, setUploadArch] = useState("x64");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [runBofId, setRunBofId] = useState<number>(0);
  const [runAgent, setRunAgent] = useState("");
  const [runArgs, setRunArgs] = useState("");

  const handleUpload = (e: React.FormEvent) => {
    e.preventDefault();
    if (!uploadFile) return;
    onUpload(uploadFile, uploadArch, uploadName, uploadDesc);
    setShowUpload(false);
    setUploadFile(null);
    setUploadName("");
    setUploadDesc("");
    setUploadArch("x64");
  };

  const handleRun = (e: React.FormEvent) => {
    e.preventDefault();
    if (!runAgent) return;
    onRun(runBofId, runAgent, runArgs);
    setShowRun(false);
  };

  const handleEdit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!editTarget) return;
    onEdit(String(editTarget.id || ""), String(editTarget.name || ""), String(editTarget.description || ""));
    setEditTarget(null);
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
        <Button onClick={() => setShowUpload(true)}>
          <Upload className="w-4 h-4" />
          Upload BOF
        </Button>
      </div>

      <Card className="overflow-hidden">
        {files.length > 0 ? (
          <div className="divide-y divide-border">
            {files.map((b: BOFFile, i: number) => {
              const bid = b.id || String(i);
              const bname = b.name || "Unknown";
              const bdesc = b.description || "";
              const bsize = formatBytes(b.size);
              const barch = b.architecture || "x64";
              return (
                <div key={bid} className="px-5 py-4 hover:bg-muted transition-colors">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3 flex-1 min-w-0">
                      <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/20 rounded-lg flex items-center justify-center text-indigo-600 dark:text-indigo-400 text-xs font-bold shrink-0">
                        <Box className="w-4 h-4" />
                      </div>
                      <div className="min-w-0">
                        <div className="text-sm font-medium text-foreground truncate">{bname}</div>
                        <div className="text-xs text-muted-foreground mt-0.5">
                          {bsize} · {barch} {bdesc ? `· ${bdesc}` : ""}
                        </div>
                      </div>
                    </div>
                    <div className="flex gap-2 ml-4">
                      <Button
                        onClick={() => {
                          setRunBofId(Number(bid) || 0);
                          setRunAgent(agents[0]?.id || "");
                          setRunArgs("");
                          setShowRun(true);
                        }}
                        className="px-3 py-1.5 text-xs bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400 rounded-lg border border-emerald-200 dark:border-emerald-700 hover:bg-emerald-100 dark:hover:bg-emerald-900/40 transition-colors"
                      >
                        <Play className="w-4 h-4" />Run
                      </Button>
                      <Button
                        onClick={() => setShowInfo(b)}
                        className="px-3 py-1.5 text-xs bg-secondary text-muted-foreground rounded-lg border border-border hover:bg-secondary transition-colors"
                      >
                        <Info className="w-4 h-4" />
                      </Button>
                      <Button
                        onClick={() => setEditTarget(b)}
                        className="px-3 py-1.5 text-xs bg-secondary text-muted-foreground rounded-lg border border-border hover:bg-secondary transition-colors"
                      >
                        <Pencil className="w-4 h-4" />
                      </Button>
                      <Button
                        onClick={() => onDelete(Number(bid) || 0)}
                        className="px-3 py-1.5 text-xs bg-destructive/10 text-destructive rounded-lg border border-destructive/20 hover:bg-destructive/10 transition-colors"
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="text-center py-12 text-muted-foreground">
            <Box className="w-4 h-4" />
            <p>No BOF files uploaded</p>
            <p className="text-xs mt-1">Upload from local or import from BOF Repo</p>
          </div>
        )}
      </Card>

      <Dialog open={showUpload} onOpenChange={setShowUpload}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Upload BOF</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpload} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">BOF File (.o)</span>
              <input
                type="file"
                accept=".o"
                onChange={(e) => setUploadFile(e.target.files?.[0] || null)}
                required
                aria-label="BOF file"
                className="w-full bg-muted border border-border rounded-xl px-4 h-10 text-sm file:mr-3 file:py-1 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-indigo-50 file:text-indigo-600 dark:file:bg-indigo-900/20 dark:file:text-indigo-400"
              />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Name</span>
              <Input
                placeholder="BOF name"
                required
                value={uploadName}
                onChange={(e) => setUploadName(e.target.value)}
                className="w-full h-10"
              />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Description</span>
              <Input
                placeholder="Brief description"
                value={uploadDesc}
                onChange={(e) => setUploadDesc(e.target.value)}
                className="w-full h-10"
              />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Architecture</span>
              <Select value={uploadArch} onValueChange={(v) => setUploadArch(v ?? "")}>
                <SelectTrigger className="w-full h-10">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="x64">x64</SelectItem>
                  <SelectItem value="x86">x86</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <DialogFooter>
              <Button type="submit" className="w-full h-10 rounded-xl text-sm font-medium transition-colors">
                Upload
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showRun} onOpenChange={setShowRun}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Execute BOF</DialogTitle>
          </DialogHeader>
          <form onSubmit={handleRun} className="space-y-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Agent</span>
              <Select value={runAgent} onValueChange={(v) => setRunAgent(v ?? "")}>
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
              <Input placeholder="BOF arguments" value={runArgs} onChange={(e) => setRunArgs(e.target.value)} className="w-full h-10 font-mono" />
            </div>
            <DialogFooter>
              <Button type="submit" className="w-full h-10 bg-emerald-600 hover:bg-emerald-700 rounded-xl text-sm text-white font-medium transition-colors">
                Execute
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={!!showInfo} onOpenChange={() => setShowInfo(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>BOF Info</DialogTitle>
          </DialogHeader>
          {showInfo && (
            <div className="space-y-3">
              <div className="flex justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">Name</span>
                <span className="text-sm font-medium text-foreground font-mono">{showInfo.name}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">Size</span>
                <span className="text-sm text-foreground">{formatBytes(showInfo.size)}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">Architecture</span>
                <span className="text-sm text-foreground">{showInfo.architecture || "x64"}</span>
              </div>
              <div className="flex justify-between py-2 border-b border-border">
                <span className="text-sm text-muted-foreground">Description</span>
                <span className="text-sm text-foreground max-w-[60%] text-right">{showInfo.description || "-"}</span>
              </div>
              <div className="flex justify-between py-2">
                <span className="text-sm text-muted-foreground">Uploaded</span>
                <span className="text-sm text-foreground">{showInfo.created_at || "-"}</span>
              </div>
            </div>
          )}
        </DialogContent>
      </Dialog>

      <Dialog open={!!editTarget} onOpenChange={() => setEditTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit BOF</DialogTitle>
          </DialogHeader>
          {editTarget && (
            <form onSubmit={handleEdit} className="space-y-4">
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Name</span>
                <Input value={editTarget.name || ""} onChange={(e) => setEditTarget({ ...editTarget, Name: e.target.value })} className="w-full h-10 font-mono" />
              </div>
              <div>
                <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Description</span>
                <Input value={editTarget.description || ""} onChange={(e) => setEditTarget({ ...editTarget, Description: e.target.value })} className="w-full h-10" />
              </div>
              <DialogFooter>
                <Button type="submit" className="w-full h-10 rounded-xl text-sm font-medium transition-colors">
                  Save
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}

