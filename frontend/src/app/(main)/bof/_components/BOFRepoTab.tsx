"use client";

import { useState } from "react";
import type { RepoItem } from "./types";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/framework/SearchInput";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Box, Check, CheckCircle, Download, DownloadCloud, Layers, Link, Star, TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface BOFRepoTabProps {
  repoItems: RepoItem[];
  loading: boolean;
  onImport: (item: RepoItem) => Promise<{ success: boolean; message: string }>;
  onImportUrl: (url: string, name?: string) => Promise<{ success: boolean; message: string }>;
  onRate: (itemId: string, rating: number) => void;
}

function renderStars(rating: number | undefined, interactive: boolean, onRate?: (rating: number) => void, rateLabel = "Rate") {
  const r = rating ?? 0;
  return (
    <div className="flex items-center gap-0.5">
      {[1, 2, 3, 4, 5].map((star) => (
        <Button
          variant="ghost"
          size="icon-xs"
          key={star}
          onClick={() => interactive && onRate?.(star)}
          className={`${interactive ? "cursor-pointer hover:scale-110 transition-transform" : "cursor-default"} text-xs ${star <= r ? "text-warning" : "text-muted-foreground"}`}
          aria-label={`${rateLabel} ${star}`}
        >
          <Star className="w-4 h-4" />
        </Button>
      ))}
    </div>
  );
}

export default function BOFRepoTab({ repoItems, loading, onImport, onImportUrl, onRate }: BOFRepoTabProps) {
  const { t } = useI18n();
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
    setImportStatus({ loading: true, message: t("bof.importing"), success: false });
    const result = await onImportUrl(importUrl, importName || undefined);
    setImportStatus({ loading: false, message: result.message, success: result.success });
    if (result.success) {
      setImportUrl("");
      setImportName("");
    }
  };

  const handleImportFromRepo = async (item: RepoItem) => {
    setImportStatus({ loading: true, message: t("bof.importing_name", { name: item.name || t("bof.unnamed") }), success: false });
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
          <IconBadge icon={Layers} color="primary" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.community_bofs")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <IconBadge icon={Download} color="success" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.filter((i) => i.imported).length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_imported")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <IconBadge icon={Star} color="warning" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.filter((i) => (i.rating ?? 0) >= 4).length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.highly_rated")}</div>
          </div>
        </Card>
      </div>

      <Card className="p-4 sm:p-5 mb-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="w-8 h-8 bg-primary/10 dark:bg-primary/15 rounded-lg flex items-center justify-center text-primary">
            <Link className="w-4 h-4" />
          </div>
          <span className="text-sm font-semibold text-foreground">{t("bof.import_from_url")}</span>
        </div>
        <form onSubmit={handleImportUrl} className="flex gap-3">
          <Input
            aria-label={t("bof.url")}
            placeholder={t("bof.repo_url_ph")}
            required
            pattern="https?://.*"
            value={importUrl}
            onChange={(e) => setImportUrl(e.target.value)}
            className="flex-1 h-9 text-foreground"
          />
          <Input
            aria-label={t("bof.bof_name")}
            placeholder={t("bof.bof_name_optional")}
            value={importName}
            onChange={(e) => setImportName(e.target.value)}
            className="w-52 h-9 text-foreground"
          />
          <Button type="submit" size="lg" className="px-5 text-sm font-medium transition-colors">
            <Download className="w-4 h-4" />{t("bof.import")}
          </Button>
        </form>
        {importStatus && (
          <div
            className={`mt-3 p-3 rounded-lg text-xs flex items-center gap-2 ${
              importStatus.loading
                ? "bg-primary/10 text-primary"
                : importStatus.success
                  ? "bg-success/15 text-success"
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
          <SearchInput
            placeholder={t("bof.search")}
            label={t("bof.search")}
            value={repoSearch}
            onChange={setRepoSearch}
            className="w-full"
          />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.category")}</span>
          <Select value={filterCategory} onValueChange={(v) => setFilterCategory(v ?? "")}>
            <SelectTrigger className="w-full dark:text-foreground">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("bof.all_categories")}</SelectItem>
              {Array.from(new Set(repoItems.map((i) => i.category || t("bof.uncategorized")))).map((c) => (
                <SelectItem key={c} value={c}>
                  {c}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">{t("bof.architecture")}</span>
          <Select value={filterArch} onValueChange={(v) => setFilterArch(v ?? "")}>
            <SelectTrigger className="w-full dark:text-foreground">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("bof.all_architectures")}</SelectItem>
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
        <span className="text-sm text-muted-foreground">{t("bof.found", { count: filteredItems.length })}</span>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">{t("bof.sort_by")}</span>
          <Select value={sortBy} onValueChange={(v) => setSortBy(v as "stars" | "name")}>
            <SelectTrigger className="h-8 bg-card text-xs text-foreground focus-visible:ring-3">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="stars">{t("bof.sort_popularity")}</SelectItem>
              <SelectItem value="name">{t("bof.sort_name")}</SelectItem>
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
<IconBadge icon={Box} color={imported ? "success" : "primary"} size="lg" />
                    <div>
                      <div className="text-sm font-semibold text-foreground font-mono">{item.name || t("bof.unnamed")}</div>
                      <div className="text-xs text-muted-foreground">
                        {t("bof.by_author", { author: item.author || t("bof.unknown") })}
                        {item.category ? ` · ${item.category}` : ""}
                      </div>
                    </div>
                  </div>
                  <Badge variant={imported ? "success" : "outline"}>
                    {imported ? t("bof.imported") : t("bof.available")}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground mb-3 line-clamp-2">{item.description || t("bof.no_description")}</p>
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4 text-xs text-muted-foreground">
                    {renderStars(item.rating, true, (rating) => onRate(itemId, rating), t("bof.rate"))}
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
                    {imported ? t("bof.imported") : t("bof.import")}
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

