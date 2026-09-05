"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ArrowDown, ArrowUp, Download, Eye, File, Folder, ImageIcon, Pencil, Trash2, X } from "lucide-react";
import { formatSize, formatTimestamp, isImageFile, type FileEntry } from "./types";
import type { FileSortKey } from "./useFilesView";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesTableProps {
  t: TKey;
  sortedEntries: FileEntry[];
  selected: Set<string>;
  allSelected: boolean;
  toggleSelect: (name: string) => void;
  selectAll: (on: boolean) => void;
  clearSelection: () => void;
  sortKey: FileSortKey;
  sortDir: 1 | -1;
  toggleSort: (key: FileSortKey) => void;
  selectedFile: string | null;
  batchBusy: boolean;
  pullName: string | null;
  onFileClick: (entry: FileEntry) => void;
  onPreview: (name: string) => void;
  onDownload: (name: string) => void;
  onRename: (name: string) => void;
  onDelete: (name: string) => void;
  onBatchPull: () => void;
  onBatchDelete: () => void;
  onCancelBatch: () => void;
  dragOver: boolean;
  onDragOver: () => void;
  onDragLeave: () => void;
  onDrop: (e: React.DragEvent) => void;
}

/** Multi-select bar + sortable directory listing. */
export default memo(function FilesTable(props: FilesTableProps) {
  const {
    t, sortedEntries, selected, allSelected, toggleSelect, selectAll, clearSelection,
    sortKey, sortDir, toggleSort, selectedFile, batchBusy, pullName,
    onFileClick, onPreview, onDownload, onRename, onDelete,
    onBatchPull, onBatchDelete, onCancelBatch,
    dragOver, onDragOver, onDragLeave, onDrop,
  } = props;

  const sortHead = (key: FileSortKey, labelKey: string, className?: string) => (
    <TableHead className={className}>
      <button type="button" onClick={() => toggleSort(key)} className="inline-flex items-center gap-1 hover:text-foreground">
        {t(labelKey)}
        {sortKey === key && (sortDir === 1 ? <ArrowUp className="size-3" /> : <ArrowDown className="size-3" />)}
      </button>
    </TableHead>
  );

  return (
    <>
      {selected.size > 0 && (
        <Card className="flex flex-wrap items-center gap-2 p-3">
          <span className="text-xs text-muted-foreground">{t("agents.files_selected", { n: selected.size })}</span>
          <Button variant="secondary" size="sm" disabled={batchBusy} onClick={onBatchPull}>
            <Download className="size-4" /> {t("agents.files_pull_selected")}
          </Button>
          <Button variant="destructive" size="sm" disabled={batchBusy} onClick={onBatchDelete}>
            <Trash2 className="size-4" /> {t("agents.files_delete_selected")}
          </Button>
          {batchBusy && (
            <Button variant="ghost" size="sm" onClick={onCancelBatch}>
              <X className="size-4" /> {t("common.cancel")}
            </Button>
          )}
          <Button variant="ghost" size="sm" disabled={batchBusy} onClick={clearSelection}>
            <X className="size-4" /> {t("agents.files_clear")}
          </Button>
        </Card>
      )}

      <Card
        className={`min-h-0 flex-1 overflow-hidden ${dragOver ? "ring-2 ring-primary" : ""}`}
        onDragOver={(e) => { e.preventDefault(); onDragOver(); }}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
      >
        {dragOver && (
          <div className="border-b border-dashed border-primary bg-primary/5 px-4 py-2 text-center text-xs text-primary">
            {t("agents.files_drop_hint")}
          </div>
        )}
        <Table>
          <TableHeader>
            <TableRow className="bg-muted/50">
              <TableHead className="w-10">
                <Checkbox
                  checked={allSelected}
                  onCheckedChange={(checked) => selectAll(checked === true)}
                  aria-label={t("agents.select_all")}
                />
              </TableHead>
              <TableHead className="w-10">{t("common.type")}</TableHead>
              {sortHead("name", "agents.files_col_name")}
              {sortHead("size", "agents.files_col_size", "w-24")}
              {sortHead("mod_time", "agents.files_col_modified", "w-40")}
              <TableHead className="w-64 text-right">{t("common.actions")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {sortedEntries.map((entry) => (
              <TableRow
                key={entry.name}
                className={`cursor-pointer ${selectedFile === entry.name ? "bg-primary/10" : ""}`}
                onClick={() => onFileClick(entry)}
              >
                <TableCell onClick={(e) => e.stopPropagation()}>
                  <Checkbox
                    checked={selected.has(entry.name)}
                    onCheckedChange={() => toggleSelect(entry.name)}
                    aria-label={entry.name}
                  />
                </TableCell>
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
                  <div className="flex items-center justify-end gap-1">
                    {!entry.is_dir && (
                      <>
                        <Button variant="ghost" size="sm" onClick={() => onPreview(entry.name)}>
                          <Eye className="size-4" /> {t("agents.files_preview_btn")}
                        </Button>
                        <Button variant="ghost" size="sm" disabled={pullName === entry.name} onClick={() => onDownload(entry.name)}>
                          <Download className="size-4" /> {t("agents.files_pull")}
                        </Button>
                      </>
                    )}
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => onRename(entry.name)}
                      aria-label={t("agents.files_rename")}
                    >
                      <Pencil className="size-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      onClick={() => onDelete(entry.name)}
                      className="text-destructive"
                      aria-label={t("common.delete")}
                    >
                      <Trash2 className="size-4" />
                    </Button>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </Card>
    </>
  );
});
