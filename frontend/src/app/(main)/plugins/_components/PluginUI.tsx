"use client";

import { useState } from "react";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { ArrowUp, CheckCircle, Clock, Download, Info, Link, Play, Puzzle, Star, Trash2 } from "lucide-react";
import type { Plugin, Review } from "./types";
import { PLUGIN_CATEGORIES } from "./categories";
export function ReviewsModal({ plugin, reviews, open, onOpenChange, onPost }: { plugin: Plugin; reviews: Review[]; open: boolean; onOpenChange: (open: boolean) => void; onPost: (id: string, rating: number, content: string) => void }) {
  const { t } = useI18n();
  const pid = plugin.id || "";
  const [rating, setRating] = useState(5);
  const [content, setContent] = useState("");

  const submit = () => {
    onPost(pid, rating, content);
    setContent("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md max-h-[80vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{t("plugins.reviews_title")} {plugin.name}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 mb-4">
          <div className="flex items-center gap-1 mb-2">
            {[1, 2, 3, 4, 5].map((s) => (
              <Button key={s} variant="ghost" size="icon-xs" onClick={() => setRating(s)} className="text-lg" aria-label={`${s} star`}>
                <Star className={`w-4 h-4 ${s <= rating ? "text-warning fill-warning" : "text-muted-foreground"}`} />
              </Button>
            ))}
          </div>
          <Textarea aria-label={t("plugins.review_ph")} name="textarea-7" value={content} onChange={(e) => setContent(e.target.value)} placeholder={t("plugins.review_ph")} className="min-h-[5rem] resize-none" />
          <Button onClick={submit} className="w-full">{t("plugins.submit_review")}</Button>
        </div>
        <div className="space-y-3">
          {reviews.length === 0 && <p className="text-sm text-muted-foreground text-center py-4">{t("plugins.no_reviews")}</p>}
          {reviews.map((r) => (
            <div key={r.id} className="bg-muted rounded-lg p-3">
              <div className="flex items-center justify-between mb-1">
                <span className="text-xs font-medium text-muted-foreground">{r.user || t("plugins.anonymous")}</span>
                <div className="flex items-center gap-0.5">
                  {[1, 2, 3, 4, 5].map((s) => (
                    <Star key={s} className={`w-2.5 h-2.5 ${s <= (r.rating || 0) ? "text-warning fill-warning" : "text-muted-foreground"}`} />
                  ))}
                </div>
              </div>
              <p className="text-xs text-muted-foreground">{r.content}</p>
            </div>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  );
}

export function PluginCard({ plugin, actionState, onInstall, onUninstall, onDelete, onToggle, onDetail, onExecute, onExport, onUpdate, onReviews, onRating }: {
  plugin: Plugin;
  actionState?: string;
  onInstall: (id: string) => void;
  onUninstall: (id: string) => void;
  onDelete: (id: string) => void;
  onToggle: (id: string, enabled: boolean) => void;
  onDetail: () => void;
  onExecute: () => void;
  onExport: () => void;
  onUpdate: () => void;
  onReviews: () => void;
  onRating: (r: number) => void;
}) {
  const { t } = useI18n();
  const id = plugin.id || "";
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || "";
  const author = plugin.author || "-";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const downloads = plugin.downloads || 0;
  const [hoverRating, setHoverRating] = useState(0);
  const catInfo = PLUGIN_CATEGORIES.find((c) => c.key === cat);

  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-all group">
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-3">
<IconBadge icon={Puzzle} color="primary" size="xl" className="dark:bg-primary/20" />
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-foreground truncate cursor-pointer hover:text-primary dark:hover:text-primary transition-colors" onClick={onDetail}>{name}</h3>
            <p className="text-xs text-muted-foreground">v{version} &middot; {author}</p>
          </div>
        </div>
        {updateAvail && (
          <Button size="xs" onClick={(e) => { e.stopPropagation(); onUpdate(); }} className="shrink-0 px-2 py-0.5 bg-warning/15 text-warning text-(--fs-micro-sm) font-medium rounded-lg hover:bg-warning/15 transition-colors" title={t("plugins.update_available")}>
            <ArrowUp className="w-4 h-4" />{t("plugins.update")}
          </Button>
        )}
      </div>

      <p className="text-xs text-muted-foreground mb-3 line-clamp-2 leading-relaxed">{desc || t("plugins.no_desc")}</p>

      <div className="flex items-center gap-2 mb-3 flex-wrap">
        {catInfo && (
          <span className={`text-(--fs-micro-sm) px-2 py-0.5 rounded-lg ${catInfo.color}`}>
            <span className="mr-1">{catInfo.icon}</span>{t(catInfo.labelKey)}
          </span>
        )}
      </div>

      <div className="flex items-center gap-0.5 mb-1">
        {[1, 2, 3, 4, 5].map((s) => (
          <Button key={s} variant="ghost" size="icon-xs" onClick={() => onRating(s)} onMouseEnter={() => setHoverRating(s)} onMouseLeave={() => setHoverRating(0)} className="p-0.5" aria-label={`${s} star`}>
            <Star className={`w-2.5 h-2.5 transition-colors ${s <= (hoverRating || rating) ? "text-warning fill-warning" : "text-muted-foreground"}`} />
          </Button>
        ))}
        <span className="text-(--fs-micro-sm) text-muted-foreground ml-1">{(hoverRating || rating).toFixed(1)}</span>
        <Button variant="ghost" size="xs" onClick={onReviews} className="text-(--fs-micro-sm) text-primary ml-1 hover:underline">{t("plugins.reviews")}</Button>
      </div>
      <div className="text-(--fs-micro-sm) text-muted-foreground mb-3">{downloads.toLocaleString()} {t("plugins.downloads")}</div>

      {deps.length > 0 && (
        <div className="mb-3 text-(--fs-micro-sm) text-muted-foreground">
          <Link className="w-4 h-4" />{t("plugins.deps")} {deps.join(", ")}
        </div>
      )}

        <div className="flex items-center justify-between pt-3 border-t border-border">
        {installed ? (
          <div className="flex items-center gap-2">
            <Switch checked={enabled} onCheckedChange={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} />
            <span className="text-(--fs-micro-sm) text-muted-foreground">{enabled ? t("plugins.enabled") : t("plugins.disabled")}</span>
          </div>
        ) : (
          <span className="text-(--fs-micro-sm) text-muted-foreground">{t("plugins.not_installed")}</span>
        )}
        <div className="flex items-center gap-1">
          <Button variant="ghost" size="icon-xs" onClick={onExecute} disabled={!installed} className="text-muted-foreground hover:text-success hover:bg-success/15 disabled:opacity-30" aria-label={t("plugins.execute")}><Play className="w-4 h-4" /></Button>
          <Button variant="ghost" size="icon-xs" onClick={onExport} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-primary/20" aria-label={t("plugins.export")}><Download className="w-4 h-4" /></Button>
          {installed ? (
            <>
              <Button size="xs" onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-(--fs-micro-sm) font-medium text-warning bg-warning/15 hover:bg-warning/15 transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <Spinner /> : t("plugins.uninstall")}</Button>
              <Button variant="ghost" size="icon-xs" onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-50" aria-label={t("plugins.delete")}><Trash2 className="w-4 h-4" /></Button>
            </>
          ) : (
            <Button size="xs" onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-2.5 py-1 text-(--fs-micro-sm) font-medium text-primary bg-primary/10 dark:bg-primary/20 hover:bg-primary/10 dark:hover:bg-chart-3/20 transition-colors disabled:opacity-50">{actionState === "installing" ? <><Spinner className="mr-1" />...</> : t("plugins.install")}</Button>
          )}
          <Button variant="ghost" size="icon-xs" onClick={onDetail} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-chart-3/20" aria-label={t("plugins.details")}><Info className="w-4 h-4" /></Button>
        </div>
      </div>
    </Card>
  );
}

export function PluginListItem({ plugin, actionState, onInstall, onUninstall, onDelete, onToggle, onDetail, onExecute, onExport, onUpdate, onReviews, onRating }: {
  plugin: Plugin;
  actionState?: string;
  onInstall: (id: string) => void;
  onUninstall: (id: string) => void;
  onDelete: (id: string) => void;
  onToggle: (id: string, enabled: boolean) => void;
  onDetail: () => void;
  onExecute: () => void;
  onExport: () => void;
  onUpdate: () => void;
  onReviews: () => void;
  onRating: (r: number) => void;
}) {
  const { t } = useI18n();
  const id = plugin.id || "";
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || "";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const catInfo = PLUGIN_CATEGORIES.find((c) => c.key === cat);

  return (
    <Card className="p-4 hover:shadow-lg dark:hover:shadow-black/30 transition-all">
      <div className="flex items-center gap-4">
<IconBadge icon={Puzzle} color="primary" size="xl" className="dark:bg-primary/20" />
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <h3 className="text-sm font-semibold text-foreground truncate cursor-pointer hover:text-primary dark:hover:text-primary transition-colors" onClick={onDetail}>{name}</h3>
            <span className="text-(--fs-micro-sm) text-muted-foreground">v{version}</span>
            {catInfo && <span className={`text-(--fs-micro-sm) px-1.5 py-0.5 rounded ${catInfo.color}`}>{t(catInfo.labelKey)}</span>}
            {updateAvail && <Button size="xs" onClick={onUpdate} className="text-(--fs-micro-sm) px-1.5 py-0.5 rounded bg-warning/15 text-warning hover:bg-warning/20"><ArrowUp className="w-4 h-4" />{t("plugins.update")}</Button>}
          </div>
          <p className="text-xs text-muted-foreground truncate">{desc || t("plugins.no_desc_short")}</p>
        </div>
        <div className="flex items-center gap-0.5 shrink-0">
          {[1, 2, 3, 4, 5].map((s) => <Button key={s} variant="ghost" size="icon-xs" onClick={() => onRating(s)} aria-label={`${s} star`}><Star className={`w-2.5 h-2.5 hover:text-warning transition-colors ${s <= rating ? "text-warning fill-warning" : "text-muted-foreground"}`} /></Button>)}
        <Button variant="ghost" size="xs" onClick={onReviews} className="text-(--fs-micro-sm) text-primary ml-1 hover:underline">{t("plugins.reviews")}</Button>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <Button variant="ghost" size="icon-xs" onClick={onExecute} disabled={!installed} className="text-muted-foreground hover:text-success hover:bg-success/15 disabled:opacity-30" aria-label={t("plugins.execute")}><Play className="w-4 h-4" /></Button>
          <Button variant="ghost" size="icon-xs" onClick={onExport} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-primary/20" aria-label={t("plugins.export")}><Download className="w-4 h-4" /></Button>
          {installed ? (
            <>
              <Switch checked={enabled} onCheckedChange={() => onToggle(id, !enabled)} disabled={actionState === "toggling"} className="shrink-0" />
              <Button size="xs" onClick={() => onUninstall(id)} disabled={actionState === "uninstalling"} className="px-2 py-1 text-(--fs-micro-sm) font-medium text-warning bg-warning/15 hover:bg-warning/15 transition-colors disabled:opacity-50">{actionState === "uninstalling" ? <Spinner /> : t("plugins.uninstall")}</Button>
              <Button variant="ghost" size="icon-xs" onClick={() => onDelete(id)} disabled={actionState === "deleting"} className="text-muted-foreground hover:text-destructive hover:bg-destructive/10 disabled:opacity-50" aria-label={t("plugins.delete")}><Trash2 className="w-4 h-4" /></Button>
            </>
          ) : (
            <Button onClick={() => onInstall(id)} disabled={actionState === "installing"} className="px-2.5 py-1 text-(--fs-micro-sm) font-medium text-primary bg-primary/10 dark:bg-primary/20 hover:bg-primary/10 dark:hover:bg-chart-3/20 transition-colors disabled:opacity-50">{actionState === "installing" ? <><Spinner className="mr-1" />{t("plugins.installing")}</> : t("plugins.install")}</Button>
          )}
          <Button variant="ghost" size="icon-xs" onClick={onDetail} className="text-muted-foreground hover:text-primary hover:bg-primary/10 dark:hover:bg-chart-3/20" aria-label={t("plugins.details")}><Info className="w-4 h-4" /></Button>
        </div>
      </div>
      {deps.length > 0 && (
        <div className="mt-2 ml-14 text-(--fs-micro-sm) text-muted-foreground">
          <Link className="w-4 h-4" />{t("plugins.dependencies")} {deps.join(", ")}
        </div>
      )}
    </Card>
  );
}

export function PluginDetailModal({ plugin, open, onOpenChange }: {
  plugin: Plugin;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const { t } = useI18n();
  const name = plugin.name || t("plugins.unknown");
  const version = plugin.version || "1.0.0";
  const desc = plugin.description || t("plugins.no_desc");
  const author = plugin.author || "-";
  const cat = plugin.category || "";
  const enabled = plugin.Enabled !== undefined ? plugin.Enabled : plugin.enabled;
  const rating = plugin.Rating !== undefined ? plugin.Rating : plugin.rating || 0;
  const deps = plugin.dependencies || [];
  const installed = plugin.Installed !== undefined ? plugin.Installed : plugin.installed;
  const updateAvail = plugin.UpdateAvailable !== undefined ? plugin.UpdateAvailable : plugin.update_available;
  const readme = plugin.readme || "";
  const downloads = plugin.downloads || 0;
  const lastUpdated = plugin.last_updated || "-";

  const catInfo = PLUGIN_CATEGORIES.find((c) => c.key === cat);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[85vh] overflow-hidden flex flex-col p-0">
        <DialogHeader className="px-6 py-4 border-b border-border">
          <div className="flex items-center gap-3">
<IconBadge icon={Puzzle} color="primary" size="xl" className="dark:bg-primary/20" />
            <div>
              <DialogTitle>{name}</DialogTitle>
              <p className="text-xs text-muted-foreground">v{version} &middot; {author}</p>
            </div>
          </div>
        </DialogHeader>

        <div className="overflow-y-auto flex-1 px-6 py-4 space-y-5">
          <div className="flex items-center gap-3 flex-wrap">
            {catInfo && (
              <span className={`text-xs px-2.5 py-1 rounded-lg ${catInfo.color}`}>
                <span className="mr-1">{catInfo.icon}</span>{t(catInfo.labelKey)}
              </span>
            )}
            {installed ? (
              <Badge variant={enabled ? "success" : "secondary"} className="text-xs">
                {enabled ? <CheckCircle className="w-3 h-3 mr-1" /> : <span className="w-2 h-2 rounded-full bg-current inline-block mr-1" />}{enabled ? t("plugins.enabled") : t("plugins.disabled")}
              </Badge>
            ) : (
              <Badge variant="secondary" className="text-xs">{t("plugins.not_installed")}</Badge>
            )}
            {updateAvail && (
              <Badge variant="warning" className="text-xs">
                <ArrowUp className="w-4 h-4" />{t("plugins.update_available")}
              </Badge>
            )}
          </div>

          <div className="flex items-center gap-4">
            <div className="flex items-center gap-0.5">
              {[1, 2, 3, 4, 5].map((s) => (
                <Star key={s} className={`w-3 h-3 ${s <= rating ? "text-warning fill-warning" : "text-muted-foreground"}`} />
              ))}
              <span className="text-xs text-muted-foreground ml-1">{rating.toFixed(1)}</span>
            </div>
            <span className="text-xs text-muted-foreground"><Download className="w-4 h-4" />{downloads.toLocaleString()} {t("plugins.downloads")}</span>
            <span className="text-xs text-muted-foreground"><Clock className="w-4 h-4" />{t("plugins.updated")}: {lastUpdated}</span>
          </div>

          <div>
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.description")}</h3>
            <p className="text-sm text-muted-foreground leading-relaxed">{desc}</p>
          </div>

          {deps.length > 0 && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.dependencies")}</h3>
              <div className="flex flex-wrap gap-2">
                {deps.map((d) => (
                  <Badge key={d} variant="outline" className="font-mono">{d}</Badge>
                ))}
              </div>
            </div>
          )}

          {readme && (
            <div>
              <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">{t("plugins.readme")}</h3>
              <div className="bg-muted border border-border rounded-lg p-4 text-xs text-muted-foreground whitespace-pre-wrap max-h-60 overflow-y-auto">{readme}</div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
