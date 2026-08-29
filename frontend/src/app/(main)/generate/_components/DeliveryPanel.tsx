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
        <Button type="button" onClick={onGenerate} disabled={busy} className="w-full">
          {busy ? t("generate.panel.generating") : t("generate.delivery_build")}
        </Button>
      }
    >
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div>
          <Label>{t("generate.delivery_format")}</Label>
          <Select value={format} onValueChange={(v) => v && setFormat(v)}>
            <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
            <SelectContent>
              <SelectItem value="html">{t("generate.delivery_html")}</SelectItem>
              <SelectItem value="url">{t("generate.delivery_url")}</SelectItem>
              <SelectItem value="lnk">{t("generate.delivery_lnk")}</SelectItem>
              <SelectItem value="iso">{t("generate.delivery_iso")}</SelectItem>
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
      </div>
    </PayloadCard>
  );
}
