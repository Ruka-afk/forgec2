
import { useState } from "react";
import type { RepoItem } from "./types";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button, buttonVariants } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SearchInput } from "@/components/SearchInput";
import { EmptyState } from "@/components/ui/empty-state";
import { CheckCircle, Download, ExternalLink, Layers, Link, TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { safeHref } from "@/lib/safeUrl";
import { cn } from "@/lib/utils";

interface BOFRepoTabProps {
  repoItems: RepoItem[];
  loading: boolean;
  onImportUrl: (url: string, name?: string) => Promise<{ success: boolean; message: string }>;
}

function itemUrl(item: RepoItem): string | undefined {
  return safeHref(item.url || item.URL);
}

export default function BOFRepoTab({ repoItems, loading, onImportUrl }: BOFRepoTabProps) {
  const { t } = useI18n();
  const [importUrl, setImportUrl] = useState("");
  const [importName, setImportName] = useState("");
  const [importStatus, setImportStatus] = useState<{ loading: boolean; message: string; success: boolean } | null>(null);
  const [repoSearch, setRepoSearch] = useState("");

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

  const query = repoSearch.trim().toLowerCase();
  const filteredItems = query
    ? repoItems.filter((item) => {
        const haystack = `${item.name || ""} ${item.description || ""} ${item.author || ""}`.toLowerCase();
        return haystack.includes(query);
      })
    : repoItems;

  return (
    <div>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-6">
        <Card className="p-(--card-spacing) flex items-center gap-3">
          <IconBadge icon={Layers} color="primary" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{repoItems.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.curated_collections")}</div>
          </div>
        </Card>
        <Card className="p-(--card-spacing) flex items-center gap-3">
          <IconBadge icon={Link} color="info" size="xl" />
          <div>
            <div className="text-xl font-bold text-foreground">{query ? filteredItems.length : repoItems.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.found", { count: query ? filteredItems.length : repoItems.length })}</div>
          </div>
        </Card>
      </div>

      <Card className="p-(--card-spacing) mb-6">
        <div className="flex items-center gap-3 mb-4">
          <div className="size-8 bg-primary/10 dark:bg-primary/15 rounded-lg flex items-center justify-center text-primary">
            <Link className="size-4" />
          </div>
          <span className="text-sm font-semibold text-foreground">{t("bof.import_from_url")}</span>
        </div>
        <form onSubmit={handleImportUrl} className="flex flex-col sm:flex-row gap-3">
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
            className="sm:w-52 h-9 text-foreground"
          />
          <Button type="submit" size="lg" className="px-5 text-sm font-medium transition-colors">
            <Download className="size-4" />{t("bof.import")}
          </Button>
        </form>
        <p className="text-xs text-muted-foreground mt-2">{t("bof.import_filename_hint")}</p>
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
            {importStatus.loading ? <Spinner size="xs" /> : importStatus.success ? <CheckCircle className="size-4" /> : <TriangleAlert className="size-4" />}
            {importStatus.message}
          </div>
        )}
      </Card>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-6">
        <div className="sm:col-span-3">
          <SearchInput
            placeholder={t("bof.search")}
            label={t("bof.search")}
            value={repoSearch}
            onChange={setRepoSearch}
            className="w-full"
          />
        </div>
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <Spinner />
        </div>
      ) : filteredItems.length === 0 ? (
        <EmptyState
          icon={Layers}
          title={repoItems.length === 0 ? t("bof.no_collections") : t("bof.no_search_results")}
          message={repoItems.length === 0 ? t("bof.no_collections_hint") : t("bof.no_search_results_hint")}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {filteredItems.map((item, i) => {
            const href = itemUrl(item);
            return (
              <Card key={item.id || item.name || String(i)} className="p-(--card-spacing) hover:shadow-lg dark:hover:shadow-xl transition-shadow">
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="flex items-center gap-3 min-w-0">
                    <IconBadge icon={Layers} color="primary" size="lg" />
                    <div className="min-w-0">
                      <div className="text-sm font-semibold text-foreground font-mono truncate">{item.name || t("bof.unnamed")}</div>
                      {item.author && (
                        <div className="text-xs text-muted-foreground">{t("bof.by_author", { author: item.author })}</div>
                      )}
                    </div>
                  </div>
                </div>
                <p className="text-xs text-muted-foreground mb-3 line-clamp-2">{item.description || t("bof.no_description")}</p>
                {href && (
                  <a
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={cn(buttonVariants({ variant: "outline", size: "sm" }), "w-full")}
                  >
                    <ExternalLink className="size-4" />{t("bof.open_collection")}
                  </a>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
