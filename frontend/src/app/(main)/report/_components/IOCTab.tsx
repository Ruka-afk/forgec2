"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { formatTime } from "@/lib/utils";
import { useI18n } from "@/lib/i18n";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/ui/spinner";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import { Download, FileJson, FileSpreadsheet } from "lucide-react";

interface IocEntry {
  type: string;
  value: string;
  count: number;
  first_seen: string;
  last_seen: string;
}

const IOC_TYPES = ["", "ipv4", "domain", "url", "md5", "sha1", "sha256"] as const;

/**
 * IOCTab — indicators of compromise extracted from task results and recon
 * data, with one-click STIX 2.1 / CSV export for handover.
 */
export default function IOCTab() {
  const { t } = useI18n();
  const [entries, setEntries] = useState<IocEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState("30");
  const [typeFilter, setTypeFilter] = useState("");
  const [downloading, setDownloading] = useState<"stix2" | "csv" | null>(null);
  const loadGenRef = useRef(0);

  const load = useCallback(() => {
    setLoading(true);
    const q = new URLSearchParams({ days });
    if (typeFilter) q.set("type", typeFilter);
    // Generation guard: rapid filter toggles must not let the slower older
    // request overwrite newer results.
    const gen = ++loadGenRef.current;
    api.get<{ iocs?: IocEntry[]; tasks_scanned?: number }>(paths.report.iocs(q.toString()))
      .then((d) => {
        if (gen !== loadGenRef.current) return;
        setEntries(d.iocs || []);
      })
      .catch(() => {
        if (gen !== loadGenRef.current) return;
        setEntries([]);
      })
      .finally(() => {
        if (gen === loadGenRef.current) setLoading(false);
      });
  }, [days, typeFilter]);

  useEffect(() => { load(); }, [load]);

  const download = async (format: "stix2" | "csv") => {
    if (downloading) return;
    setDownloading(format);
    try {
      const fallback = `forgec2-iocs-${days}d.${format === "stix2" ? "json" : "csv"}`;
      const { blob, filename } = await api.downloadGet(paths.report.iocExport(format, Number(days)), fallback);
      downloadBlob(blob, filename);
    } catch {
      toast.error(t("report.toast.download_failed"));
    } finally {
      setDownloading(null);
    }
  };

  return (
    <Card className="p-(--card-spacing)">
      <div className="flex items-center justify-between gap-3 flex-wrap mb-4">
        <h2 className="text-lg font-semibold text-foreground">{t("report.ioc_title")}</h2>
        <div className="flex items-center gap-2 flex-wrap">
          <Select value={days} onValueChange={(v) => v && setDays(v)}>
            <SelectTrigger className="w-28" aria-label={t("report.ioc_range")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="7">{t("report.ioc_days", { n: "7" })}</SelectItem>
              <SelectItem value="30">{t("report.ioc_days", { n: "30" })}</SelectItem>
              <SelectItem value="90">{t("report.ioc_days", { n: "90" })}</SelectItem>
            </SelectContent>
          </Select>
          <Select value={typeFilter || "all"} onValueChange={(v) => setTypeFilter(v === "all" ? "" : (v ?? ""))}>
            <SelectTrigger className="w-32" aria-label={t("report.ioc_type")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("builds.filter_all")}</SelectItem>
              {IOC_TYPES.filter((x) => x !== "").map((ty) => (
                <SelectItem key={ty} value={ty}>{ty}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" onClick={() => void download("stix2")} disabled={downloading !== null} className="gap-1.5">
            {downloading === "stix2" ? <Spinner size="xs" /> : <FileJson className="size-4" />} STIX 2.1
          </Button>
          <Button variant="outline" size="sm" onClick={() => void download("csv")} disabled={downloading !== null} className="gap-1.5">
            {downloading === "csv" ? <Spinner size="xs" /> : <FileSpreadsheet className="size-4" />} CSV
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="py-10 text-center"><Spinner /></div>
      ) : entries.length === 0 ? (
        <p className="text-xs text-muted-foreground text-center py-8">{t("report.ioc_empty")}</p>
      ) : (
        <div className="overflow-x-auto max-h-[480px] overflow-y-auto rounded-lg border border-border divide-y divide-border">
          {entries.map((e, i) => (
            <div key={`${e.type}-${e.value}-${i}`} className="px-3 py-2 flex items-center gap-3 hover:bg-muted/40 transition-colors">
              <Badge variant="outline" className="font-mono text-(--fs-micro) shrink-0 w-16 justify-center">{e.type}</Badge>
              <code className="text-xs font-mono text-foreground truncate flex-1 select-all">{e.value}</code>
              <span className="text-(--fs-micro-sm) text-muted-foreground shrink-0">×{e.count}</span>
              <span className="text-(--fs-micro-sm) text-muted-foreground shrink-0 hidden md:block w-28 text-right">{formatTime(e.last_seen)}</span>
            </div>
          ))}
        </div>
      )}

      <p className="text-(--fs-micro-sm) text-muted-foreground mt-2 flex items-center gap-1.5">
        <Download className="size-3.5" />{t("report.ioc_hint")}
      </p>
    </Card>
  );
}
