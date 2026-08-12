"use client";

import { useRef, useState } from "react";
import { useParams } from "next/navigation";
import { downloadText } from "@/lib/download";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Spinner } from "@/components/UI";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ArrowLeft, ArrowRight, CloudUpload, Download, Eye, File, FileText, FolderOpen, FolderTree, HardDrive, ImageIcon, RotateCw, Search, Trash2, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { useI18n } from "@/lib/i18n";
import {
  formatSize,
  formatTimestamp,
  getFileIcon,
  joinPath,
  parentPath,
  type FileEntry,
} from "./_components/types";
import { useAgentFiles } from "./_components/useAgentFiles";

export default function FilesPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showUpload, setShowUpload] = useState(false);
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
    previewContent,
    previewIsImage,
    showPreview,
    setShowPreview,
    drives,
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

  const handleFind = async () => {
    await findFiles();
  };

  const sep = osType === "linux" ? "/" : "\\";
  const rootPath = osType === "linux" ? "/" : "C:\\";
  const pathParts = currentPath.split(/[\\/]/).filter(Boolean);

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <h1 className="text-2xl font-bold">          <FolderTree className="w-4 h-4" />
          {t("agents.files_title")}
          </h1>
          <Badge variant="secondary" className="text-xs font-mono">{id}</Badge>
        </div>        <div className="flex items-center gap-2">
          <Button variant="default" size="sm" onClick={() => setShowFind(!showFind)} className="bg-primary hover:bg-primary/90 text-primary-foreground">
            <Search className="w-4 h-4" />
            {t("agents.files_find")}
          </Button>
          <Button variant="default" size="sm" onClick={() => setShowUpload(true)}>
            <CloudUpload className="w-4 h-4" />
            {t("agents.files_upload")}
          </Button>
          <Button variant="default" size="sm" onClick={loadDrives}>
            <HardDrive className="w-4 h-4" />
            {t("agents.files_drives")}
          </Button>
          <Button variant="secondary" size="sm" onClick={() => loadDirectory(currentPath)}>
            <RotateCw className="w-4 h-4" />
            {t("agents.files_refresh")}
          </Button>
        </div>
      </div>

      <div className="bg-muted border border-border rounded-xl p-5 mb-4 shadow-sm">
        <div className="flex items-center gap-3 mb-3">
          <FolderOpen className="w-4 h-4" />
          <Input
            type="text"
            value={currentPathInput}            onChange={(e) => setCurrentPathInput(e.target.value)}
            onKeyDown={(e) => {              if (e.key === "Enter") {                navigateTo(currentPathInput);
              }            }}
                          className="flex-1 font-mono text-foreground"          />
          <Button variant="default" onClick={() => navigateTo(currentPathInput)}>
            <ArrowRight className="w-4 h-4" /> {t("agents.files_go")}
          </Button>
          <Button variant="secondary" onClick={goUp} aria-label={t("agents.files.go_up")}>
            <ArrowLeft className="w-4 h-4" /> {t("agents.files_up")}
          </Button>
        </div>        <div className="flex items-center gap-1.5 text-sm">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => navigateTo(rootPath)}
            className={`font-mono transition-colors ${
              currentPath === rootPath
                ? "text-foreground bg-primary/10"
                : "text-primary hover:bg-primary/5"
            }`}
          >            {rootPath}
          </Button>
          {pathParts.map((part, i) => (
            <span key={part} className="flex items-center gap-1.5">
              <span className="text-muted-foreground font-mono">{sep === "/" ? "/" : "\\"}</span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleBreadcrumbClick(i + 1)}
                className={`font-mono transition-colors hover:bg-primary/5 ${                  i === pathParts.length - 1 ? "text-foreground bg-primary/10" : "text-primary"
                }`}
              >
                {part}
              </Button>
            </span>
          ))}
        </div>

        <div className="flex items-center gap-2 mt-3 text-xs">          <span className="text-muted-foreground font-medium">{t("agents.files_quick")}</span>
          {quickPaths.map((qp) => (
            <Button
              variant="outline"
              size="sm"
              key={qp.path}
              onClick={() => navigateTo(qp.path)}
              className="text-primary hover:bg-primary/5 transition-colors font-medium"
            >
              {qp.label}
            </Button>
          ))}
        </div>
      </div>

      {showFind && (
        <Card className="p-5 mb-4">
          <div className="font-semibold text-sm text-foreground mb-3 flex items-center gap-2">
            <Search className="w-4 h-4" />
            {t("agents.files_search_files")}
          </div>
          <div className="flex gap-3">
            <Input
              type="text"
              value={findPattern}              onChange={(e) => setFindPattern(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleFind()}
              placeholder={t("agents.files_find_placeholder")}
            className="flex-1 font-mono text-foreground"
            />
            <Button variant="default" onClick={handleFind} className="bg-primary hover:bg-primary/90 text-primary-foreground">
              <Search className="w-4 h-4" /> {t("common.search")}
            </Button>
            {findResults.length > 0 && (
              <Button variant="secondary" onClick={() => { setFindResults([]); setFindPattern(""); }}>
                <X className="w-4 h-4" /> {t("agents.files_clear")}
              </Button>
            )}
          </div>
          {findResults.length > 0 && (
            <div className="mt-3 max-h-48 overflow-y-auto border border-border rounded-xl bg-muted/50">
              {findResults.map((result) => (
                <Button
                  variant="ghost"
                  size="sm"
                  key={result}
                  onClick={() => {
                    const parentDir = result.substring(0, result.lastIndexOf(sep)) || currentPath;                    const fileName = result.split(sep).pop() || "";
                    navigateTo(parentDir);
                    setSelectedFile(fileName);                  }}
                  className="w-full justify-start text-left px-4 py-2 text-xs font-mono text-muted-foreground hover:bg-primary/10 border-b border-border last:border-0 transition-colors gap-2"
                >
                  <File className="w-4 h-4" />
                  {result}
                </Button>
              ))}
            </div>
          )}
        </Card>
      )}

      <Dialog open={showDrives} onOpenChange={setShowDrives}>
        <DialogContent className="max-w-md">
          <DialogHeader className="bg-primary/10 border-b border-primary/20 -mx-6 -mt-6 px-6 py-5 rounded-t-2xl">
            <DialogTitle className="text-lg font-semibold text-foreground flex items-center gap-2">
              <HardDrive className="w-4 h-4" />
              {t("agents.files_available_drives")}
            </DialogTitle>
          </DialogHeader>
          <div className="max-h-64 overflow-y-auto">
            {drives.length === 0 ? (
              <p className="text-muted-foreground text-sm text-center py-4">{t("agents.files_no_drives")}</p>            ) : (
              <div className="space-y-2">
                {drives.map((drive) => (
                  <Button
                    variant="ghost"
                    size="sm"
                    key={drive.letter}                    onClick={() => {                      navigateTo(drive.letter);                      setShowDrives(false);
                    }}
                    className="w-full justify-start items-center gap-3 px-4 py-3 bg-muted hover:bg-primary/10 rounded-xl transition-colors"
                  >
                    <HardDrive className="w-4 h-4" />
                    <div className="text-left">                      <div className="text-sm font-medium text-foreground">
                        {drive.letter}{drive.label ? ` (${drive.label})` : ""}
                      </div>                      {drive.total > 0 && (
                        <div className="text-xs text-muted-foreground mt-0.5">
                          {t("agents.files_free_of", { free: formatSize(drive.free), total: formatSize(drive.total) })}                          <Progress value={Math.max(0, 100 - (drive.free / drive.total) * 100)} className="h-1.5 mt-1" />
                        </div>
                      )}
                    </div>
                  </Button>                ))}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>      {loading && (
        <div className="text-center py-8">
          <div className="inline-flex items-center gap-3">
            <Spinner size="sm" color="blue" />
            <p className="text-muted-foreground text-sm">{t("agents.files_loading")}</p>
          </div>
        </div>
      )}

      {!loading && entries.length === 0 && (        <Card className="text-center py-10">
          <FolderOpen className="w-4 h-4" />
          <p className="text-muted-foreground">{t("agents.files_empty")}</p>
        </Card>
      )}      {!loading && entries.length > 0 && (
        <Card className="overflow-hidden">
          <div className="overflow-x-auto">
            <Table className="w-full text-sm">
              <TableHeader>
                <TableRow className="border-b border-border bg-muted/50">
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-12">{t("common.type")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground">{t("agents.files_col_name")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-24">{t("agents.files_col_size")}</TableHead>
                  <TableHead className="text-left py-3 px-4 text-xs font-semibold text-muted-foreground w-40">{t("agents.files_col_modified")}</TableHead>
                  <TableHead className="text-right py-3 px-4 text-xs font-semibold text-muted-foreground w-44">{t("common.actions")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {entries.map((entry) => (
                  <TableRow
                    key={entry.name}
                    className={`border-b border-border last:border-0 hover:bg-primary/5 cursor-pointer transition-colors ${                      selectedFile === entry.name ? "bg-primary/10" : ""
                    }`}
                    onClick={() => handleFileClick(entry)}>
                    <TableCell className="py-2.5 px-4 text-center text-lg">
                      {getFileIcon(entry)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4">
                      <span className={`font-mono text-sm ${entry.is_dir ? "text-primary font-medium" : "text-foreground"}`}>
                        {entry.name}
                      </span>
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-sm text-muted-foreground font-mono">
                      {entry.is_dir ? "" : formatSize(entry.size)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-xs text-muted-foreground font-mono">
                      {formatTimestamp(entry.mod_time)}
                    </TableCell>
                    <TableCell className="py-2.5 px-4 text-right" onClick={(e) => e.stopPropagation()}>
                      {!entry.is_dir && (
                        <div className="flex items-center justify-end gap-1">
                          <Button variant="ghost" size="sm" onClick={() => void readFile(entry.name)} className="text-primary">
                            <Eye className="w-4 h-4" /> {t("agents.files_preview_btn")}
                          </Button>
                          <Button variant="ghost" size="sm" onClick={() => void downloadFile(entry.name)} className="text-blue-600 dark:text-blue-400">
                            <Download className="w-4 h-4" /> {t("agents.files_dl")}
                          </Button>
                          <Button variant="ghost" size="icon-sm" onClick={() => handleDelete(entry.name)} className="text-destructive" aria-label={t("common.delete")}>
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>        </Card>
      )}

      <Dialog open={showUpload} onOpenChange={(open) => { if (!uploading) setShowUpload(open); }}>
        <DialogContent className="max-w-md">
          <DialogHeader className="bg-emerald-500/10 border-b border-emerald-500/20 -mx-6 -mt-6 px-6 py-5 rounded-t-2xl">
            <DialogTitle className="text-lg font-semibold text-foreground flex items-center gap-2">
              <CloudUpload className="w-4 h-4" />
              {t("agents.files_upload_file")}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={handleUpload} className="space-y-4">
            <div>                <Label className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("agents.files_destination_path")}</Label>                <Input
                type="text"
                value={currentPath}
                readOnly                className="font-mono text-foreground"              />
            </div>
            <div>                <Label className="text-xs font-medium text-muted-foreground mb-1.5 block">{t("agents.files_select_file")}</Label>
              <input
                ref={fileInputRef}
                type="file"                className="w-full bg-card border border-border text-sm rounded-xl px-3 py-2.5 text-foreground"
                aria-label={t("agents.files.select_upload")}
              />
            </div>
            {uploading && (
              <div>
                <div className="flex justify-between text-xs text-muted-foreground mb-1">
                  <span>{t("agents.files_uploading")}</span>
                  <span>{uploadProgress}%</span>
                </div>
                <Progress value={uploadProgress} />
              </div>            )}
            <DialogFooter className="flex gap-3 pt-2 sm:justify-stretch">
              <Button
                type="button"
                variant="secondary"
                onClick={() => setShowUpload(false)}
                disabled={uploading}
                className="flex-1"
              >
                <X className="w-4 h-4" /> {t("common.cancel")}
              </Button>
              <Button
                type="submit"
                variant="default"
                disabled={uploading}
                className="flex-1"
              >
                        {uploading ? (                    <><Spinner size="sm" /> {t("agents.files_uploading")}</>
                ) : (
                  <><CloudUpload className="w-4 h-4" /> {t("agents.files_upload")}</>
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
      <Dialog open={showPreview && !!previewContent} onOpenChange={setShowPreview}>
        <DialogContent className="max-w-3xl max-h-[80vh] flex flex-col">
          <DialogHeader className={`px-6 py-5 ${              previewIsImage ? "bg-pink-500/10 border-b border-pink-500/20" : "bg-primary/10 border-b border-primary/20"
          } -mx-6 -mt-6 rounded-t-2xl`}>
            <DialogTitle className="text-lg font-semibold text-foreground flex items-center justify-between">
              <span className="flex items-center gap-2">
                {                  previewIsImage ? (
                    <><ImageIcon className="w-4 h-4" aria-hidden="true" /> {t("agents.files_image")} {selectedFile}</>
                  ) : (
                    <><FileText className="w-4 h-4" /> {t("agents.files_preview")} {selectedFile}</>
                  )
                }
              </span>
              <span className="flex items-center gap-2">                {!previewIsImage && (
                <Tooltip>
                  <TooltipTrigger render={<Button variant="ghost" size="icon-sm"
                    onClick={() => {
                        downloadText(previewContent, selectedFile || "preview.txt");
                    }}
                    aria-label={t("agents.files_download")}
                  >
                    <Download className="w-4 h-4" />
                  </Button>} />
                  <TooltipContent>{t("agents.files_download_content")}</TooltipContent>
                </Tooltip>
                )}
              </span>
            </DialogTitle>
          </DialogHeader>
          {previewIsImage ? (
            <div className="p-4 flex-1 flex items-center justify-center bg-muted/50 overflow-auto">                <img src={previewContent} alt={selectedFile || t("agents.files_preview_btn")} className="max-w-full max-h-[70vh] object-contain rounded-lg shadow-lg dark:shadow-black/30" loading="lazy" onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; }} />              </div>
          ) : (
            <div className="p-4 sm:p-5 overflow-auto flex-1">
              <pre className="text-sm font-mono whitespace-pre-wrap text-foreground bg-muted/50 rounded-xl p-4 border border-border">
                {previewContent}
              </pre>            </div>
          )}
        </DialogContent>
      </Dialog>
      {modal}
    </div>
  );
}
