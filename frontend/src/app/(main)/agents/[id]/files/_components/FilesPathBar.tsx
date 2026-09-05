"use client";

import { memo, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { ArrowLeft, ArrowRight, FolderOpen } from "lucide-react";
import { parentPath } from "./types";

type TKey = (key: string, params?: Record<string, string | number>) => string;

interface FilesPathBarProps {
  t: TKey;
  osType: "linux" | "windows";
  currentPath: string;
  currentPathInput: string;
  setCurrentPathInput: (v: string) => void;
  navigateTo: (path: string) => void;
  quickPaths: { label: string; path: string }[];
}

/** Path input + breadcrumb trail + quick-jump buttons. */
export default memo(function FilesPathBar({
  t, osType, currentPath, currentPathInput, setCurrentPathInput, navigateTo, quickPaths,
}: FilesPathBarProps) {
  const sep = osType === "linux" ? "/" : "\\";
  // Derive root from current path instead of hardcoding "C:\\" — browsing
  // D:\foo roots at D:\, and drive-root slices resolve to "C:\\" not "C:".
  const { rootPath, pathParts } = useMemo(() => {
    const driveMatch = currentPath.match(/^([A-Za-z]:)/);
    const root = osType === "linux" ? "/" : (driveMatch ? driveMatch[1] + "\\" : "C:\\");
    return { rootPath: root, pathParts: currentPath.split(/[\\/]/).filter(Boolean) };
  }, [currentPath, osType]);

  const goUp = () => navigateTo(parentPath(currentPath, osType));

  const handleBreadcrumbClick = (index: number) => {
    const parts = currentPath.split(/[\\/]/).filter(Boolean);
    if (index === 0) {
      navigateTo(rootPath);
    } else {
      const sliced = parts.slice(0, index);
      const joined = sliced.join(sep);
      const needsSlash = osType !== "linux" && sliced.length === 1 && !joined.endsWith("\\");
      navigateTo(osType === "linux" ? "/" + joined : (needsSlash ? joined + "\\" : joined));
    }
  };

  return (
    <div className="rounded-lg border border-border bg-muted/40 p-4">
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
  );
});
