"use client";

import { useState } from "react";
import type { RepoItem } from "./types";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Box, Check, CheckCircle, Download, DownloadCloud, Layers, Link, Star, TriangleAlert } from "lucide-react";

interface BOFRepoTabProps {
  repoItems: RepoItem[];
  loading: boolean;
  onImport: (item: RepoItem) => Promise<{ success: boolean; message: string }>;
  onImportUrl: (url: string, name?: string) => Promise<{ success: boolean; message: string }>;
  onRate: (itemId: string, rating: number) => void;
}

function renderStars(rating: number | undefined, interactive: boolean, onRate?: (rating: number) => void) {
  const r = rating ?? 0;
  return (
    <div className="flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((star) => (
        <Button
          variant="ghost"
          size="icon-xs"
          key={star}
          onClick={() => interactive && onRate?.(star)}
          className={`${interactive ? "cursor-pointer hover:scale-110 transition-transform" : "cursor-default"} text-xs ${star <= r ? "text-amber-500" : "text-muted-foreground"}`}
          aria-label="Rate"
        >
          <Star className="w-4 h-4" />
        </Button>
      ))}
    </div>
  );
}

export default function BOFRepoTab({ repoItems, loading, onImport, onImportUrl, onRate }: BOFRepoTabProps) {
  const [importUrl, setImportUrl] = useState("");
  const [importName, setImportName] = useState("");
  const [importStatus, setImportStatus] = useState<{ loading: boolean; message: string; success: boolean } | null>(null);
  const [repoSearch, setRepoSearch] = useState("");
  const [filterCategory, setFilterCategory] = useState("all");
  const [filterArch, setFilterArch] = useState("all");
  const [sortBy, setSortBy] = useState<"stars" | "name">("stars");

  const handleImportUrl = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!importUrl.trim()) return;
    setImportStatus({ loading: true, message: "Importing BOF from URL...", success: false });
    const result = await onImportUrl(importUrl, importName || undefined);
    setImportStatus({ loading: false, message: result.message, success: result.success });
    if (result.success) {
      setImportUrl("");
      setImportName("");
    }
  };

  const handleImportFromRepo = async (item: RepoItem) => {
    setImportStatus({ loading: true, message: `Importing ${item.name}...`, success: false });
    const result = await onImport(item);
    setImportStatus({ loading: false, message: result.message, success: result.success });
  };

  const filteredItems = repoItems.filter((item) => {
    const name = (item.name || "").toLowerCase();
    const desc = (item.description || "").toLowerCase();
    const author = (item.author || "").toLowerCase();
    const query = repoSearch.toLowerCase();
    const matchSearch = !query || name.includes(query) || desc.includes(query) || author.includes(query);
    const matchCategory = filterCategory === "all" || (item.category) === filterCategory;
    const matchArch = filterArch === "all" || (item.architecture) === filterArch;
    return matchSearch && matchCategory && matchArch;
  }).sort((a, b) => {
    if (sortBy === "stars") return (b.stars ?? 0) - (a.stars ?? 0);
    if (sortBy === "name") return (a.name ?? "").localeCompare(b.name ?? "");
    return 0;
  });

  return (
    <div>
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-primary/10 rounded-xl flex items-center justify-center">
            <Layers className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.length}</div>
            <div className="text-xs text-muted-foreground">Community BOFs</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/20 rounded-xl flex items-center justify-center">
            <Download className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.filter((i) => i.imported).length}</div>
            <div className="text-xs text-muted-foreground">Imported</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/20 rounded-xl flex items-center justify-center">
            <Star className="w-4 h-4" />
          </div>
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.filter((i) => (i.rating ?? 0) >= 4).length}</div>
            <div className="text-xs text-muted-foreground">Highly Rated</div>
          </div>
        </Card>
      </div>

      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-8 h-8 bg-indigo-100 dark:bg-indigo-900/20 rounded-lg flex items-center justify-center text-indigo-600 dark:text-indigo-400">
            <Link className="w-4 h-4" />
          </div>
          <span className="text-sm font-semibold text-foreground">Import from URL</span>
        </div>
        <form onSubmit={handleImportUrl} className="flex gap-3">
          <Input
            placeholder="https://..."
            required
            pattern="https?://.*"
            value={importUrl}
            onChange={(e) => setImportUrl(e.target.value)}
            className="flex-1 h-10 text-foreground"
          />
          <Input
            placeholder="BOF Name (optional)"
            value={importName}
            onChange={(e) => setImportName(e.target.value)}
            className="w-52 h-10 text-foreground"
          />
          <Button type="submit" className="px-5 h-10 rounded-xl text-sm font-medium transition-colors">
            <Download className="w-4 h-4" />Import
          </Button>
        </form>
        {importStatus && (
          <div
            className={`mt-3 p-3 rounded-xl text-xs flex items-center gap-2 ${
              importStatus.loading
                ? "bg-primary/10 text-primary"
                : importStatus.success
                  ? "bg-emerald-50 text-emerald-600 dark:bg-emerald-900/20 dark:text-emerald-400"
                  : "bg-destructive/10 text-destructive"
            }`}
          >
            {importStatus.loading ? <Spinner size="xs" /> : importStatus.success ? <CheckCircle className="w-4 h-4" /> : <TriangleAlert className="w-4 h-4" />}
            {importStatus.message}
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Search</span>
            <Input
              placeholder="Search BOFs..."
              value={repoSearch}
              onChange={(e) => setRepoSearch(e.target.value)}
              className="w-full h-9 text-foreground"
            />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Category</span>
          <Select value={filterCategory} onValueChange={(v) => setFilterCategory(v ?? "")}>
            <SelectTrigger className="w-full h-9 dark:text-foreground">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Categories</SelectItem>
              {Array.from(new Set(repoItems.map((i) => i.category || "Uncategorized"))).map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Architecture</span>
          <Select value={filterArch} onValueChange={(v) => setFilterArch(v ?? "")}>
            <SelectTrigger className="w-full h-9 dark:text-foreground">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Architectures</SelectItem>
              {Array.from(new Set(repoItems.map((i) => i.architecture || "x64"))).map((a) => (
                <SelectItem key={a} value={a}>
                  {a}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="flex items-center justify-between mb-4">
        <span className="text-sm text-muted-foreground">{filteredItems.length} BOFs found</span>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">Sort by:</span>
          <Select value={sortBy} onValueChange={(v) => setSortBy(v as "stars" | "name")}>
            <SelectTrigger className="h-8 bg-card text-xs text-foreground focus-visible:ring-3">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="stars">Popularity</SelectItem>
              <SelectItem value="name">Name</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>

      {loading ? (
        <div className="text-center py-12 text-muted-foreground">
          <Spinner />
        </div>
      ) : repoItems.length === 0 ? (
        <div className="text-center py-12 text-muted-foreground">
          <Spinner />
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredItems.map((item, i) => {
            const itemId = item.id || String(i);
            const imported = item.imported;
            return (
              <Card key={itemId} className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
                <div className="flex items-start justify-between mb-3">
                  <div className="flex items-center gap-3">
                    <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${imported ? "bg-emerald-100 dark:bg-emerald-900/20 text-emerald-600 dark:text-emerald-400" : "bg-primary/10 text-primary"}`}>
                      <Box className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-sm font-semibold text-foreground font-mono">{item.name || "Unnamed"}</div>
                      <div className="text-xs text-muted-foreground">
                        by {item.author || "Unknown"}
                        {item.category ? ` · ${item.category}` : ""}
                      </div>
                    </div>
                  </div>
                  <Badge variant={imported ? "success" : "outline"}>
                    {imported ? "Imported" : "Available"}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground mb-3 line-clamp-2">{item.description || "No description"}</p>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    {renderStars(item.rating, true, (rating) => onRate(itemId, rating))}
                    <span className="flex items-center gap-1">
                      <Star className="w-4 h-4" />
                      {item.reviews ?? 0}
                    </span>
                    <span className="flex items-center gap-1">
                      <Download className="w-4 h-4" />
                      {item.downloads ?? 0}
                    </span>
                    <Badge variant="secondary" className="font-mono">{item.architecture || "x64"}</Badge>
                  </div>
                  <Button onClick={() => handleImportFromRepo(item)} disabled={!!imported} size="sm">
                    {imported ? <Check className="w-4 h-4 mr-1" /> : <DownloadCloud className="w-4 h-4 mr-1" />}
                    {imported ? "Imported" : "Import"}
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}

