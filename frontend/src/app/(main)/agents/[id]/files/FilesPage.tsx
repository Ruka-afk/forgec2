"use client";

import { useRef, useState } from "react";
import { useParams } from "next/navigation";
import { downloadText } from "@/lib/download";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  ArrowLeft, ArrowRight, CloudUpload, Download, Eye, File, FileText, Folder,
  FolderOpen, FolderTree, HardDrive, ImageIcon, RotateCw, Search, Trash2, X,
} from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Progress } from "@/components/ui/progress";
import { useI18n } from "@/lib/i18n";
import { formatSize, formatTimestamp, isImageFile, joinPath, parentPath, type FileEntry } from "./_components/types";
import { transferPercent } from "./_components/file-task";
import { useAgentFiles } from "./_components/useAgentFiles";
import { safeImageSrc } from "@/lib/safeUrl";

export default function FilesPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showUpload, setShowUpload] = useState(false);
  const [uploadName, setUploadName] = useState("");
  const [showFind, setShowFind] = useState(false);
  const { confirm, modal } = useConfirm();

  const {
    currentPath,
    currentPathInput,
    setCurrentPathInput,
    entries,
    loading,
    selectedFile,
    setSelectedFile,
    uploadProgress,
    uploading,
    pullName,
    pullProgress,
    previewContent,
    previewIsImage,
    showPreview,
    setShowPreview,
    driveOutput,
    showDrives,
    setShowDrives,
    findPattern,
    setFindPattern,
    findResults,
    setFindResults,
    osType,
    quickPaths,
    loadDirectory,
    navigateTo,
    downloadFile,
    readFile,
    deleteFile,
    uploadFile,
    loadDrives,
    findFiles,
  } = useAgentFiles(id);

  const goUp = () => navigateTo(parentPath(currentPath, osType));

  const handleBreadcrumbClick = (index: number) => {
    const sep = osType === "linux" ? "/" : "\\";
    const parts = currentPath.split(/[\\/]/).filter(Boolean);
    if (index === 0) {
      navigateTo(osType === "linux" ? "/" : "C:\\");
    } else {
      const sliced = parts.slice(0, index);
      navigateTo(osType === "linux" ? "/" + sliced.join("/") : sliced.join(sep));
    }
  };

  const handleFileClick = (entry: FileEntry) => {
    if (entry.is_dir) {
      navigateTo(joinPath(currentPath, entry.name, osType));
    } else {
      setSelectedFile(entry.name);
    }
  };

  const handleDelete = async (filename: string) => {
    if (!(await confirm({ message: t("agents.files_confirm_delete") }))) return;
    await deleteFile(filename);
  };

  const handleUpload = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    if (!fileInputRef.current?.files?.length) return;
    const file = fileInputRef.current.files[0];
    uploadFile(file, () => setShowUpload(false));
  };

  const sep = osType === "linux" ? "/" : "\\";
  const rootPath = osType === "linux" ? "/" : "C:\\";
  const pathParts = currentPath.split(/[\\/]/).filter(Boolean);

  return (
    <PageContainer>
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex min-w-0 items-center gap-3">
          <h1 className="flex items-center gap-2 text-2xl font-bold">
            <FolderTree className="size-5" />
            {t("agents.files_title")}
          </h1>
          <Badge variant="secondary" className="font-mono text-xs">{id}</Badge>
        </div>
        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button variant="secondary" size="sm" onClick={() => setShowFind((v) => !v)}>
            <Search className="size-4" />
            {t("agents.files_find")}
          </Button>
          <Button size="sm" onClick={() => setShowUpload(true)}>
            <CloudUpload className="size-4" />
            {t("agents.files_upload")}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => void loadDrives()}>
            <HardDrive className="size-4" />
            {t("agents.files_drives")}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => void loadDirectory(currentPath)}>
            <RotateCw className="size-4" />
            {t("agents.files_refresh")}
          </Button>
        </div>
      </div>

      <p className="mb-4 text-xs text-muted-foreground">{t("agents.files_channel_hint")}</p>

      <div className="mb-4 rounded-lg border border-border bg-muted/40 p-4">
        <div className="mb-3 flex items-center gap-2">
          <FolderOpen className="size-4 shrink-0 text-muted-foreground" />
          <Input
            value={currentPathInput}
            onChange={(e) => setCurrentPathInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") navigateTo(currentPathInput);
            }}
            className="flex-1 font-mono"
            aria-label={t("agents.dock_files_path")}
          />
          <Button onClick={() => navigateTo(currentPathInput)}>
            <ArrowRight className="size-4" /> {t("agents.files_go")}
          </Button>
          <Button variant="secondary" onClick={goUp} aria-label={t("agents.files.go_up")}>
            <ArrowLeft className="size-4" /> {t("agents.files_up")}
          </Button>
        </div>
        <div className="flex flex-wrap items-center gap-1 text-sm">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigateTo(rootPath)}
            className={`font-mono ${currentPath === rootPath ? "bg-primary/10 text-foreground" : "text-primary"}`}
          >
            {rootPath}
          </Button>
          {pathParts.map((part, i) => (
            <span key={`${part}-${i}`} className="flex items-center gap-1">
              <span className="font-mono text-muted-foreground">{sep}</span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleBreadcrumbClick(i + 1)}
                className={`font-mono ${i === pathParts.length - 1 ? "bg-primary/10 text-foreground" : "text-primary"}`}
              >
                {part}
              </Button>
            </span>
          ))}
        </div>
        <div className="mt-3 flex flex-wrap items-center gap-2 text-xs">
          <span className="font-medium text-muted-foreground">{t("agents.files_quick")}</span>
          {quickPaths.map((qp) => (
            <Button key={qp.path} variant="outline" size="sm" onClick={() => navigateTo(qp.path)}>
              {qp.label}
            </Button>
          ))}
        </div>
      </div>

      {showFind && (
        <Card className="mb-4 p-4">
          <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
            <Search className="size-4" />
            {t("agents.files_search_files")}
          </div>
          <div className="flex gap-2">
            <Input
              value={findPattern}
              onChange={(e) => setFindPattern(e.target.value)}
              onKeyDown={(e) => { if (e.key === "Enter") void findFiles(); }}
              placeholder={t("agents.files_find_placeholder")}
              className="flex-1 font-mono"
            />
            <Button onClick={() => void findFiles()}>
              <Search className="size-4" /> {t("common.search")}
            </Button>
            {findResults.length > 0 && (
              <Button variant="secondary" onClick={() => { setFindResults([]); setFindPattern(""); }}>
                <X className="size-4" /> {t("agents.files_clear")}
              </Button>
            )}
          </div>
          {findResults.length > 0 && (
            <div className="mt-3 max-h-48 overflow-y-auto rounded-lg border border-border bg-muted/50">
              {findResults.map((result) => (
                <Button
                  key={result}
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    const parentDir = result.substring(0, result.lastIndexOf(sep)) || currentPath;
                    const fileName = result.split(/[\\/]/).pop() || "";
                    navigateTo(parentDir);
                    setSelectedFile(fileName);
                  }}
                  className="w-full justify-start gap-2 border-b border-border px-4 py-2 font-mono text-xs last:border-0"
                >
                  <File className="size-3.5" />
                  {result}
                </Button>
              ))}
            </div>
          )}
        </Card>
      )}

      <Dialog open={showDrives} onOpenChange={setShowDrives}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <HardDrive className="size-4" />
              {t("agents.files_drives_output")}
            </DialogTitle>
          </DialogHeader>
          {driveOutput ? (
            <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/50 p-3 font-mono text-xs">
              {driveOutput}
            </pre>
          ) : (
            <p className="py-4 text-center text-sm text-muted-foreground">{t("agents.files_no_drives")}</p>
          )}
        </DialogContent>
      </Dialog>

      {pullName && pullProgress && (
        <Card className="p-3">
          <div className="mb-1 flex justify-between text-xs text-muted-foreground">
            <span>{t("agents.files_pulling", { filename: pullName })}</span>
            <span className="font-mono">
              {t("agents.files_pull_progress", {
                current: pullProgress.chunkIndex,
                total: pullProgress.chunkCount,
                pct: transferPercent(pullProgress),
              })}
            </span>
          </div>
          <Progress value={transferPercent(pullProgress)} />
        </Card>
      )}

      {loading && (
        <div className="flex items-center justify-center gap-3 py-10">
          <Spinner size="sm" />
          <p className="text-sm text-muted-foreground">{t("agents.dock_files_waiting")}</p>
        </div>
      )}

      {!loading && entries.length === 0 && (
        <Card className="flex flex-col items-center gap-2 py-10">
          <FolderOpen className="size-6 text-muted-foreground" />
          <p className="text-muted-foreground">{t("agents.files_empty")}</p>
        </Card>
      )}

      {!loading && entries.length > 0 && (
        <Card className="overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/50">
                <TableHead className="w-10">{t("common.type")}</TableHead>
                <TableHead>{t("agents.files_col_name")}</TableHead>
                <TableHead className="w-24">{t("agents.files_col_size")}</TableHead>
                <TableHead className="w-40">{t("agents.files_col_modified")}</TableHead>
                <TableHead className="w-52 text-right">{t("common.actions")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {entries.map((entry) => (
                <TableRow
                  key={entry.name}
                  className={`cursor-pointer ${selectedFile === entry.name ? "bg-primary/10" : ""}`}
                  onClick={() => handleFileClick(entry)}
                >
                  <TableCell className="text-center">
                    {entry.is_dir ? (
                      <Folder className="mx-auto size-4 text-primary" />
                    ) : isImageFile(entry.name) ? (
                      <ImageIcon className="mx-auto size-4 text-muted-foreground" />
                    ) : (
                      <File className="mx-auto size-4 text-muted-foreground" />
                    )}
                  </TableCell>
                  <TableCell>
                    <span className={`font-mono text-sm ${entry.is_dir ? "font-medium text-primary" : ""}`}>
                      {entry.name}
                    </span>
                  </TableCell>
                  <TableCell className="font-mono text-sm text-muted-foreground">
                    {entry.is_dir ? "" : formatSize(entry.size)}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">
                    {formatTimestamp(entry.mod_time)}
                  </TableCell>
                  <TableCell className="text-right" onClick={(e) => e.stopPropagation()}>
                    {!entry.is_dir && (
                      <div className="flex items-center justify-end gap-1">
                        <Button variant="ghost" size="sm" onClick={() => void readFile(entry.name)}>
                          <Eye className="size-4" /> {t("agents.files_preview_btn")}
                        </Button>
                        <Button variant="ghost" size="sm" disabled={pullName === entry.name} onClick={() => void downloadFile(entry.name)}>
                          <Download className="size-4" /> {t("agents.files_pull")}
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-sm"
                          onClick={() => void handleDelete(entry.name)}
                          className="text-destructive"
                          aria-label={t("common.delete")}
                        >
                          <Trash2 className="size-4" />
                        </Button>
                      </div>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      <Dialog open={showUpload} onOpenChange={(open) => { if (!uploading) setShowUpload(open); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <CloudUpload className="size-4" />
              {t("agents.files_upload_file")}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpload} className="space-y-4">
            <p className="text-xs text-muted-foreground">{t("agents.files_upload_hint")}</p>
            <div>
              <Label className="mb-1.5 block text-xs text-muted-foreground">{t("agents.files_dest_file")}</Label>
              <Input
                value={uploadName ? joinPath(currentPath, uploadName, osType) : currentPath}
                readOnly
                className="font-mono"
              />
            </div>
            <div>
              <Label className="mb-1.5 block text-xs text-muted-foreground">{t("agents.files_select_file")}</Label>
              <Input
                ref={fileInputRef}
                type="file"
                aria-label={t("agents.files.select_upload")}
                onChange={(e) => setUploadName(e.target.files?.[0]?.name || "")}
              />
            </div>
            {uploading && (
              <div>
                <div className="mb-1 flex justify-between text-xs text-muted-foreground">
                  <span>{t("agents.files_uploading")}</span>
                  <span>{uploadProgress}%</span>
                </div>
                <Progress value={uploadProgress} />
              </div>
            )}
            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setShowUpload(false)} disabled={uploading} className="flex-1">
                <X className="size-4" /> {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={uploading} className="flex-1">
                {uploading ? (
                  <><Spinner size="sm" /> {t("agents.files_uploading")}</>
                ) : (
                  <><CloudUpload className="size-4" /> {t("agents.files_upload")}</>
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={showPreview && !!previewContent} onOpenChange={setShowPreview}>
        <DialogContent className="flex max-h-[80vh] max-w-3xl flex-col">
          <DialogHeader>
            <DialogTitle className="flex items-center justify-between gap-2">
              <span className="flex items-center gap-2">
                {previewIsImage ? (
                  <><ImageIcon className="size-4" /> {t("agents.files_image")} {selectedFile}</>
                ) : (
                  <><FileText className="size-4" /> {t("agents.files_preview")} {selectedFile}</>
                )}
              </span>
              {!previewIsImage && (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        onClick={() => downloadText(previewContent, selectedFile || "preview.txt")}
                        aria-label={t("agents.files_download")}
                      >
                        <Download className="size-4" />
                      </Button>
                    }
                  />
                  <TooltipContent>{t("agents.files_download_content")}</TooltipContent>
                </Tooltip>
              )}
            </DialogTitle>
          </DialogHeader>
          {previewIsImage ? (
            <div className="flex flex-1 items-center justify-center overflow-auto bg-muted/50 p-4">
                <img
                src={safeImageSrc(previewContent)}
                alt={selectedFile || t("agents.files_preview_btn")}
                className="max-h-[70vh] max-w-full rounded-lg object-contain"
                loading="lazy"
                onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }}
              />
            </div>
          ) : (
            <pre className="flex-1 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-muted/50 p-4 font-mono text-sm">
              {previewContent}
            </pre>
          )}
        </DialogContent>
      </Dialog>
      {modal}
    </PageContainer>
  );
}
