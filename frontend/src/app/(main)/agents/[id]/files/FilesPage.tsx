"use client";

import { useRef, useState } from "react";
import { useParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { PageContainer } from "@/components/ui/page-container";
import { Progress } from "@/components/ui/progress";
import { Spinner } from "@/components/ui/spinner";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { FolderOpen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { joinPath, type FileEntry } from "./_components/types";
import { transferPercent } from "./_components/file-task";
import { useAgentFiles } from "./_components/useAgentFiles";
import { useFilesView } from "./_components/useFilesView";
import FilesToolbar from "./_components/FilesToolbar";
import FilesPathBar from "./_components/FilesPathBar";
import FilesFindPanel from "./_components/FilesFindPanel";
import FilesTable from "./_components/FilesTable";
import FilesDialogs from "./_components/FilesDialogs";

export default function FilesPage() {
  const { t } = useI18n();
  const urlParams = useParams<{ id: string }>();
  const id = Array.isArray(urlParams?.id) ? urlParams.id[0] : urlParams?.id || "";
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [showUpload, setShowUpload] = useState(false);
  const [uploadName, setUploadName] = useState("");
  const [showFind, setShowFind] = useState(false);
  const [showMkdir, setShowMkdir] = useState(false);
  const [mkdirName, setMkdirName] = useState("");
  const [renameTarget, setRenameTarget] = useState<string | null>(null);
  const [renameName, setRenameName] = useState("");
  const [dragOver, setDragOver] = useState(false);
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
    huntDownload,
    setHuntDownload,
    usbOutput,
    showUsb,
    setShowUsb,
    osType,
    quickPaths,
    loadDirectory,
    navigateTo,
    downloadFile,
    readFile,
    deleteFile,
    deleteFiles,
    pullFiles,
    batchBusy,
    batchProgress,
    cancelBatch,
    mkdir,
    renameFile,
    uploadFile,
    cancelUpload,
    loadDrives,
    loadUsb,
    findFiles,
    huntFiles,
  } = useAgentFiles(id);

  const {
    selected, setSelected,
    sortKey, sortDir, toggleSelect, toggleSort,
    sortedEntries, allSelected,
  } = useFilesView(entries, currentPath);

  const sep = osType === "linux" ? "/" : "\\";

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

  const handleBatchDelete = async () => {
    const names = [...selected];
    if (names.length === 0) return;
    if (!(await confirm({ message: t("agents.files_confirm_delete") }))) return;
    await deleteFiles(names);
    setSelected(new Set());
  };

  const handleBatchPull = async () => {
    const names = [...selected];
    if (names.length === 0) return;
    await pullFiles(names);
    setSelected(new Set());
  };

  const handleUpload = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const files = fileInputRef.current?.files;
    if (!files?.length) return;
    // Upload every picked file; the input resets so the same file can retry.
    [...files].forEach((file) => uploadFile(file));
    if (fileInputRef.current) fileInputRef.current.value = "";
    setUploadName("");
    setShowUpload(false);
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const files = e.dataTransfer?.files;
    if (!files?.length) return;
    [...files].forEach((file) => uploadFile(file));
  };

  return (
    <PageContainer className="h-full gap-3 px-4 py-3 sm:px-6">
      <FilesToolbar
        t={t}
        agentId={id}
        onToggleFind={() => setShowFind((v) => !v)}
        onUpload={() => setShowUpload(true)}
        onDrives={() => void loadDrives()}
        onMkdir={() => { setMkdirName(""); setShowMkdir(true); }}
        onUsb={() => void loadUsb()}
        onRefresh={() => void loadDirectory(currentPath)}
      />

      <p className="text-xs text-muted-foreground">{t("agents.files_channel_hint")}</p>

      <FilesPathBar
        t={t}
        osType={osType}
        currentPath={currentPath}
        currentPathInput={currentPathInput}
        setCurrentPathInput={setCurrentPathInput}
        navigateTo={navigateTo}
        quickPaths={quickPaths}
      />

      {showFind && (
        <FilesFindPanel
          t={t}
          sep={sep}
          currentPath={currentPath}
          findPattern={findPattern}
          setFindPattern={setFindPattern}
          findResults={findResults}
          setFindResults={setFindResults}
          huntDownload={huntDownload}
          setHuntDownload={setHuntDownload}
          findFiles={() => void findFiles()}
          huntFiles={() => void huntFiles()}
          navigateTo={navigateTo}
          setSelectedFile={setSelectedFile}
        />
      )}

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

      {uploading && !showUpload && (
        <Card className="p-3">
          <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
            <span>{t("agents.files_uploading")} {uploadProgress}%</span>
            <Button variant="ghost" size="sm" onClick={cancelUpload}>
              <X className="size-4" /> {t("common.cancel")}
            </Button>
          </div>
          <Progress value={uploadProgress} />
        </Card>
      )}

      {batchBusy && batchProgress && (
        <Card className="p-3">
          <div className="mb-1 flex items-center justify-between text-xs text-muted-foreground">
            <span className="font-mono">{t("agents.files_selected", { n: `${batchProgress.done}/${batchProgress.total}` })} — {batchProgress.current}</span>
            <Button variant="ghost" size="sm" onClick={cancelBatch}>
              <X className="size-4" /> {t("common.cancel")}
            </Button>
          </div>
          <Progress value={Math.round((batchProgress.done / Math.max(1, batchProgress.total)) * 100)} />
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
        <FilesTable
          t={t}
          sortedEntries={sortedEntries}
          selected={selected}
          allSelected={allSelected}
          toggleSelect={toggleSelect}
          selectAll={(on) => setSelected(on ? new Set(entries.map((e) => e.name)) : new Set())}
          clearSelection={() => setSelected(new Set())}
          sortKey={sortKey}
          sortDir={sortDir}
          toggleSort={toggleSort}
          selectedFile={selectedFile}
          batchBusy={batchBusy}
          pullName={pullName}
          onFileClick={handleFileClick}
          onPreview={(name) => void readFile(name)}
          onDownload={(name) => void downloadFile(name)}
          onRename={(name) => { setRenameTarget(name); setRenameName(name); }}
          onDelete={(name) => void handleDelete(name)}
          onBatchPull={() => void handleBatchPull()}
          onBatchDelete={() => void handleBatchDelete()}
          onCancelBatch={cancelBatch}
          dragOver={dragOver}
          onDragOver={() => setDragOver(true)}
          onDragLeave={() => setDragOver(false)}
          onDrop={handleDrop}
        />
      )}

      <FilesDialogs
        t={t}
        osType={osType}
        currentPath={currentPath}
        showUsb={showUsb}
        setShowUsb={setShowUsb}
        usbOutput={usbOutput}
        showDrives={showDrives}
        setShowDrives={setShowDrives}
        driveOutput={driveOutput}
        showUpload={showUpload}
        setShowUpload={setShowUpload}
        uploading={uploading}
        uploadProgress={uploadProgress}
        uploadName={uploadName}
        setUploadName={setUploadName}
        fileInputRef={fileInputRef}
        onUploadSubmit={handleUpload}
        cancelUpload={cancelUpload}
        showMkdir={showMkdir}
        setShowMkdir={setShowMkdir}
        mkdirName={mkdirName}
        setMkdirName={setMkdirName}
        onMkdirSubmit={(e) => {
          e.preventDefault();
          if (!mkdirName.trim()) return;
          void mkdir(mkdirName).finally(() => setShowMkdir(false));
        }}
        renameTarget={renameTarget}
        setRenameTarget={setRenameTarget}
        renameName={renameName}
        setRenameName={setRenameName}
        onRenameSubmit={(e) => {
          e.preventDefault();
          if (renameTarget && renameName.trim()) {
            const target = renameTarget;
            const name = renameName;
            setRenameTarget(null);
            void renameFile(target, name);
          }
        }}
        showPreview={showPreview}
        setShowPreview={setShowPreview}
        previewContent={previewContent}
        previewIsImage={previewIsImage}
        selectedFile={selectedFile}
      />
      {modal}
    </PageContainer>
  );
}
