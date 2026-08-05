import type { ReactNode } from "react";

import { Listener, OneLinerForm, OneLinerType, OneLinerData } from "./types";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { CopyButton } from "@/components/ui/copy-button";
import { CheckCircle2, Terminal, Zap } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { FieldLabel, PayloadCard } from "./PayloadCard";
import { BuildResult, BuildStatusBadge } from "./BuildResult";

export default function OneLinerPanel({
  form, setForm, busy, result, onelinerData, listeners, getListenerInfo, onGenerate, onCopy,
}: {
  form: OneLinerForm;
  setForm: React.Dispatch<React.SetStateAction<OneLinerForm>>;
  busy: boolean;
  result: ReactNode;
  onelinerData?: OneLinerData;
  listeners: Listener[];
  getListenerInfo: (id: string) => { scheme: string; host: string; port: string | number; type: string; name: string } | null;
  onGenerate: () => void;
  onCopy: (text: string) => void;
}) {
  const { t } = useI18n();
  return (
    <PayloadCard
      icon={<Terminal className="w-5 h-5" />}
      tint="bg-rose-500/10 text-rose-600 dark:text-rose-400"
      title={t("generate.oneliner_title")}
      subtitle={t("generate.oneliner_subtitle")}
      badge={<BuildStatusBadge busy={busy} result={result === "success" ? "OK" : result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 rounded-xl font-medium flex items-center justify-center gap-x-2 bg-primary hover:bg-primary/90 text-primary-foreground disabled:opacity-50">
            {busy ? <><Spinner size="xs" /> {t("generate.panel.generating")}</> : <><Zap className="w-4 h-4" /> {t("generate.oneliner_generate")}</>}
          </Button>
          <BuildResult busy={busy} result={result === "success" ? null : result} />
        </>
      }
    >
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <div>
          <FieldLabel>{t("generate.payload_type")}</FieldLabel>
          <Select value={form.payload_type} onValueChange={(val) => val != null && setForm({ ...form, payload_type: val })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="exe">{t("generate.panel.exe_title")}</SelectItem>
              <SelectItem value="ps1">{t("generate.panel.ps1_title")}</SelectItem>
              <SelectItem value="linux">{t("generate.panel.elf_title")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <FieldLabel>{t("generate.oneliner_listener")}</FieldLabel>
          <Select value={form.listener_id} onValueChange={(val) => {
            if (val == null) return;
            const info = getListenerInfo(val);
            let c2url = "";
            let protocol = "http";
            if (info) {
              c2url = `${info.scheme}://${info.host}:${info.port}`;
              if (info.scheme === "tcp" || info.scheme === "tls") protocol = "tcp";
              else if (info.scheme === "dns" || info.type === "dns") protocol = "dns";
            }
            setForm({ ...form, listener_id: val, c2_url: c2url, protocol });
          }}>
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t("generate.oneliner_select")} />
            </SelectTrigger>
            <SelectContent>
              {listeners.map((l, i) => {
                const id = String(l.id ?? i);
                const name = l.name || t("generate.unknown_listener");
                const scheme = l.scheme || l.type || "http";
                const host = l.host || "";
                const port = l.port || "";
                return <SelectItem key={id} value={id}>{name} ({scheme}://{host}:{port})</SelectItem>;
              })}
            </SelectContent>
          </Select>
        </div>
        <div>
          <FieldLabel>{t("generate.heartbeat_sec")}</FieldLabel>
          <Input aria-label={t("generate.oneliner_heartbeat_aria")} name="input-2" type="number" value={form.beacon_time} onChange={(e) => setForm({ ...form, beacon_time: e.target.value })} />
        </div>
        <div>
          <FieldLabel>{t("generate.jitter_pct")}</FieldLabel>
          <Input aria-label={t("generate.oneliner_jitter_aria")} name="input-3" type="number" value={form.jitter} onChange={(e) => setForm({ ...form, jitter: e.target.value })} />
        </div>
      </div>
      <div className="flex items-center gap-x-4">
        <div className="flex items-center gap-x-2">
          <Checkbox aria-label={t("generate.panel.skip_tls_aria")} checked={form.skip_tls} onCheckedChange={(v) => setForm({ ...form, skip_tls: v === true })} id="skip-tls-oneliner" />
          <Label htmlFor="skip-tls-oneliner" className="text-sm text-foreground">{t("generate.panel.skip_tls_short")}</Label>
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox aria-label={t("generate.panel.persist_aria")} checked={form.persist} onCheckedChange={(v) => setForm({ ...form, persist: v === true })} id="persist-oneliner" />
          <Label htmlFor="persist-oneliner" className="text-sm text-muted-foreground">{t("generate.panel.persist")}</Label>
        </div>
      </div>

      {result === "success" && onelinerData ? (
        <div className="mt-2">
          <div className="mb-3 flex items-center gap-x-2 text-xs text-emerald-600">
            <CheckCircle2 className="h-4 w-4" />
            {t("generate.oneliner_download_url")} <code className="rounded bg-muted px-2 py-0.5 text-xs">{onelinerData.download_url}</code> {t("generate.oneliner_valid")}
          </div>
          <div className="space-y-2">
            {onelinerData.types.map((item: OneLinerType, idx: number) => (
              <div key={idx} className="rounded-xl border border-border p-3 transition-colors hover:border-rose-300 dark:hover:border-rose-800">
                <div className="mb-1.5 flex items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="grid h-5 w-5 shrink-0 place-items-center rounded-md bg-rose-500/10 text-(--fs-micro-sm) font-semibold text-rose-600 dark:text-rose-400">{idx + 1}</span>
                    <span className="truncate text-sm font-medium text-foreground">{item.name}</span>
                    <span className="truncate text-(--fs-micro-sm) text-muted-foreground">{item.desc}</span>
                  </div>
                  <CopyButton text={item.command} label={t("generate.oneliner_copy")} size="xs" />
                </div>
                <code className="block rounded-xl bg-muted p-2 font-mono text-(--fs-xs-sm) leading-relaxed whitespace-pre-wrap break-all text-foreground select-all">{item.command}</code>
              </div>
            ))}
          </div>
        </div>
      ) : null}
    </PayloadCard>
  );
}
