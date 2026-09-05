"use client";

import { memo, type RefObject } from "react";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { SafeImg } from "@/components/ui/safe-img";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { Spinner } from "@/components/ui/spinner";
import { CloudUpload, Download, FileText, FolderPlus, HardDrive, ImageIcon, Pencil, Usb, X } from "lucide-react";
import { downloadText } from "@/lib/download";
import { safeImageSrc } from "@/lib/safeUrl";
import { joinPath } from "./types";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesDialogsProps {
  t: TKey;
  osType: "linux" | "windows";
  currentPath: string;
  // usb / drives output
  showUsb: boolean;
  setShowUsb: (v: boolean) => void;
  usbOutput: string;
  showDrives: boolean;
  setShowDrives: (v: boolean) => void;
  driveOutput: string;
  // upload
  showUpload: boolean;
  setShowUpload: (v: boolean) => void;
  uploading: boolean;
  uploadProgress: number;
  uploadName: string;
  setUploadName: (v: string) => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
  onUploadSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
  cancelUpload: () => void;
  // mkdir
  showMkdir: boolean;
  setShowMkdir: (v: boolean) => void;
  mkdirName: string;
  setMkdirName: (v: string) => void;
  onMkdirSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
  // rename
  renameTarget: string | null;
  setRenameTarget: (v: string | null) => void;
  renameName: string;
  setRenameName: (v: string) => void;
  onRenameSubmit: (e: React.FormEvent<HTMLFormElement>) => void;
  // preview
  showPreview: boolean;
  setShowPreview: (v: boolean) => void;
  previewContent: string;
  previewIsImage: boolean;
  selectedFile: string | null;
}

/** All file-browser dialogs: usb/drives output, upload, mkdir, rename, preview. */
export default memo(function FilesDialogs(props: FilesDialogsProps) {
  const {
    t, osType, currentPath,
    showUsb, setShowUsb, usbOutput,
    showDrives, setShowDrives, driveOutput,
    showUpload, setShowUpload, uploading, uploadProgress, uploadName, setUploadName,
    fileInputRef, onUploadSubmit, cancelUpload,
    showMkdir, setShowMkdir, mkdirName, setMkdirName, onMkdirSubmit,
    renameTarget, setRenameTarget, renameName, setRenameName, onRenameSubmit,
    showPreview, setShowPreview, previewContent, previewIsImage, selectedFile,
  } = props;

  return (
    <>
      <Dialog open={showUsb} onOpenChange={setShowUsb}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Usb className="size-4" />
              {t("agents.files_usb_output")}
            </DialogTitle>
          </DialogHeader>
          {usbOutput ? (
            <pre className="max-h-72 overflow-auto whitespace-pre-wrap rounded-lg bg-muted/50 p-3 font-mono text-xs">
              {usbOutput}
            </pre>
          ) : (
            <p className="py-4 text-center text-sm text-muted-foreground">{t("agents.files_usb_failed")}</p>
          )}
        </DialogContent>
      </Dialog>

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

      <Dialog open={showUpload} onOpenChange={(open) => { if (!uploading) setShowUpload(open); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <CloudUpload className="size-4" />
              {t("agents.files_upload_file")}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={onUploadSubmit} className="space-y-4">
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
                multiple
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
              {uploading ? (
                <Button type="button" variant="secondary" onClick={cancelUpload} className="flex-1">
                  <X className="size-4" /> {t("agents.files_cancel_upload")}
                </Button>
              ) : (
                <Button type="button" variant="secondary" onClick={() => setShowUpload(false)} className="flex-1">
                  <X className="size-4" /> {t("common.cancel")}
                </Button>
              )}
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

      <Dialog open={showMkdir} onOpenChange={setShowMkdir}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <FolderPlus className="size-4" />
              {t("agents.files_mkdir")}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={onMkdirSubmit} className="space-y-4">
            <div>
              <Label className="mb-1.5 block text-xs text-muted-foreground">{t("agents.files_mkdir_placeholder")}</Label>
              <Input
                value={mkdirName}
                onChange={(e) => setMkdirName(e.target.value)}
                placeholder={t("agents.files_mkdir_placeholder")}
                className="font-mono"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setShowMkdir(false)} className="flex-1">
                <X className="size-4" /> {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={!mkdirName.trim()} className="flex-1">
                <FolderPlus className="size-4" /> {t("agents.files_mkdir")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      <Dialog open={renameTarget !== null} onOpenChange={(open) => { if (!open) setRenameTarget(null); }}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Pencil className="size-4" />
              {t("agents.files_rename")}: {renameTarget}
            </DialogTitle>
          </DialogHeader>
          <form onSubmit={onRenameSubmit} className="space-y-4">
            <div>
              <Label className="mb-1.5 block text-xs text-muted-foreground">{t("agents.files_rename_placeholder")}</Label>
              <Input
                value={renameName}
                onChange={(e) => setRenameName(e.target.value)}
                placeholder={t("agents.files_rename_placeholder")}
                className="font-mono"
                autoFocus
              />
            </div>
            <DialogFooter>
              <Button type="button" variant="secondary" onClick={() => setRenameTarget(null)} className="flex-1">
                <X className="size-4" /> {t("common.cancel")}
              </Button>
              <Button type="submit" disabled={!renameName.trim()} className="flex-1">
                <Pencil className="size-4" /> {t("agents.files_rename")}
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
              <SafeImg
                src={safeImageSrc(previewContent)}
                alt={selectedFile || t("agents.files_preview_btn")}
                className="max-h-[70vh] max-w-full rounded-lg object-contain"
                loading="lazy"
              />
            </div>
          ) : (
            <pre className="flex-1 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-muted/50 p-4 font-mono text-sm">
              {previewContent}
            </pre>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
});
