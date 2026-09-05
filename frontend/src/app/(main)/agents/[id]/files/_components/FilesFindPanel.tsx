"use client";

import { memo } from "react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { File, Search, X } from "lucide-react";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesFindPanelProps {
  t: TKey;
  sep: string;
  currentPath: string;
  findPattern: string;
  setFindPattern: (v: string) => void;
  findResults: string[];
  setFindResults: (v: string[]) => void;
  huntDownload: boolean;
  setHuntDownload: (v: boolean | ((p: boolean) => boolean)) => void;
  findFiles: () => void;
  huntFiles: () => void;
  navigateTo: (path: string) => void;
  setSelectedFile: (name: string | null) => void;
}

/** Filename search + deep-hunt panel with clickable results. */
export default memo(function FilesFindPanel({
  t, sep, currentPath, findPattern, setFindPattern, findResults, setFindResults,
  huntDownload, setHuntDownload, findFiles, huntFiles, navigateTo, setSelectedFile,
}: FilesFindPanelProps) {
  return (
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
        <Button variant="secondary" onClick={() => void huntFiles()}>
          <Search className="size-4" /> {t("agents.files_hunt")}
        </Button>
        {findResults.length > 0 && (
          <Button variant="secondary" onClick={() => { setFindResults([]); setFindPattern(""); }}>
            <X className="size-4" /> {t("agents.files_clear")}
          </Button>
        )}
      </div>
      <p className="mt-2 text-xs text-muted-foreground">{t("agents.files_hunt_hint")}</p>
      <Button
        type="button"
        variant={huntDownload ? "secondary" : "outline"}
        size="sm"
        className="mt-2"
        onClick={() => setHuntDownload((v) => !v)}
      >
        {t("agents.files_hunt_download")}
      </Button>
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
  );
});
