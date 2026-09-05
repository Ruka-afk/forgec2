"use client";

import { memo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { EmptyState } from "@/components/ui/empty-state";
import { Spinner } from "@/components/ui/spinner";
import { SafeImg } from "@/components/ui/safe-img";
import { toast } from "sonner";
import { Camera, Download, Mic, Trash2 } from "lucide-react";
import { api, pollTask } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { downloadBlob } from "@/lib/download";
import { useI18n } from "@/lib/i18n";
import { base64ToBytes } from "./task-collect";
import CollectCard from "./CollectCard";

interface WebcamMicSectionProps {
  agentId: string;
  online: boolean;
}

interface Shot {
  b64: string;
  time: string;
  device: string;
}

interface Clip {
  b64: string;
  time: string;
  seconds: number;
}

async function runCapture(agentId: string, action: "webcam" | "mic", param: string): Promise<string> {
  const data = (await api.postJson(paths.toolkit.action(agentId), { action, param, shell: "" })) as {
    task_id?: number;
  };
  const taskId = data.task_id;
  if (!taskId) throw new Error("dispatch failed: no task id");
  const st = await pollTask(agentId, taskId, { timeoutMs: 180_000 });
  if (st.status === "failed") throw new Error(st.error || "capture failed");
  const task = (await api.get(paths.agents.task(agentId, String(taskId)))) as {
    result?: string;
    data?: { result?: string };
  };
  return task.result ?? task.data?.result ?? "";
}

export default memo(function WebcamMicSection({ agentId, online }: WebcamMicSectionProps) {
  const { t } = useI18n();
  const [device, setDevice] = useState("");
  const [seconds, setSeconds] = useState("10");
  const [capturing, setCapturing] = useState<"webcam" | "mic" | null>(null);
  const [shots, setShots] = useState<Shot[]>([]);
  const [clips, setClips] = useState<Clip[]>([]);

  const handleWebcam = async () => {
    if (!agentId || capturing) return;
    setCapturing("webcam");
    try {
      const b64 = (await runCapture(agentId, "webcam", device.trim())).replace(/\s+/g, "");
      if (!b64) throw new Error(t("agents.media_empty"));
      setShots((prev) => [{ b64, time: new Date().toLocaleString(), device: device.trim() || "default" }, ...prev]);
      toast.success(t("agents.media_captured"));
    } catch (e) {
      if ((e as Error).name !== "AbortError") {
        toast.error(e instanceof Error ? e.message : t("agents.media_failed"));
      }
    } finally {
      setCapturing(null);
    }
  };

  const handleMic = async () => {
    if (!agentId || capturing) return;
    const secs = Math.max(1, Math.min(300, parseInt(seconds, 10) || 10));
    // Duration only: the agent reads the first field as seconds, so a
    // multi-word device hint would corrupt device selection upstream.
    const param = String(secs);
    setCapturing("mic");
    try {
      const b64 = (await runCapture(agentId, "mic", param)).replace(/\s+/g, "");
      if (!b64) throw new Error(t("agents.media_empty"));
      setClips((prev) => [{ b64, time: new Date().toLocaleString(), seconds: secs }, ...prev]);
      toast.success(t("agents.media_captured"));
    } catch (e) {
      if ((e as Error).name !== "AbortError") {
        toast.error(e instanceof Error ? e.message : t("agents.media_failed"));
      }
    } finally {
      setCapturing(null);
    }
  };

  const downloadShot = (s: Shot, i: number) => {
    downloadBlob(new Blob([base64ToBytes(s.b64)], { type: "image/jpeg" }), `webcam_${Date.now()}_${i}.jpg`);
  };

  const downloadClip = (c: Clip, i: number) => {
    downloadBlob(new Blob([base64ToBytes(c.b64)], { type: "audio/wav" }), `mic_${c.seconds}s_${Date.now()}_${i}.wav`);
  };

  const busy = capturing !== null;

  const empty = shots.length === 0 && clips.length === 0;

  const gallery = (
    <div className="space-y-3">
            {shots.length > 0 && (
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3">
                {shots.map((s, i) => (
                  <div key={`${s.time}-${i}`} className="group relative overflow-hidden rounded-lg border border-border">
                    <SafeImg
                      src={`data:image/jpeg;base64,${s.b64}`}
                      alt={`${t("agents.media_webcam")} ${s.time}`}
                      className="aspect-video w-full object-cover"
                      loading="lazy"
                    />
                    <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-black/60 px-1.5 py-1 opacity-0 transition-opacity group-hover:opacity-100">
                      <span className="truncate font-mono text-xs text-white">{s.time}</span>
                      <span className="flex gap-1">
                        <Button variant="ghost" size="icon-xs" className="text-white hover:bg-white/20" onClick={() => downloadShot(s, i)} aria-label={t("agents.files_download")}>
                          <Download className="size-3.5" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon-xs"
                          className="text-white hover:bg-white/20"
                          onClick={() => setShots((prev) => prev.filter((_, j) => j !== i))}
                          aria-label={t("common.delete")}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
            {clips.map((c, i) => (
              <div key={`${c.time}-${i}`} className="flex flex-wrap items-center gap-2 rounded-lg border border-border bg-muted/50 px-2.5 py-2">
                <Mic className="size-4 shrink-0 text-muted-foreground" />
                <span className="font-mono text-xs text-muted-foreground">
                  {c.time} · {c.seconds}s
                </span>
                <audio controls preload="none" src={`data:audio/wav;base64,${c.b64}`} className="h-8 min-w-0 flex-1" />
                <Button variant="ghost" size="icon-xs" onClick={() => downloadClip(c, i)} aria-label={t("agents.files_download")}>
                  <Download className="size-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon-xs"
                  className="text-destructive"
                  onClick={() => setClips((prev) => prev.filter((_, j) => j !== i))}
                  aria-label={t("common.delete")}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            ))}
    </div>
  );

  return (
    <CollectCard
      title={t("agents.media_title")}
      icon={<Camera className="size-3.5" />}
      emptyIcon={Camera}
      emptyTitle={t("agents.media_empty_title")}
      emptyHint={t("agents.media_empty_hint")}
      result={null}
      resultOverride={
        empty ? (
          <EmptyState icon={Camera} title={t("agents.media_empty_title")} message={t("agents.media_empty_hint")} />
        ) : (
          gallery
        )
      }
    >
      <div className="grid grid-cols-1 gap-2 sm:grid-cols-[1fr_5rem_auto_auto]">
        <div>
          <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.media_device")}</Label>
          <Input
            value={device}
            onChange={(e) => setDevice(e.target.value)}
            placeholder={t("agents.media_device_ph")}
            className="h-8 font-mono text-xs"
          />
        </div>
        <div>
          <Label className="mb-1 block text-xs text-muted-foreground">{t("agents.media_seconds")}</Label>
          <Input
            value={seconds}
            onChange={(e) => setSeconds(e.target.value.replace(/[^0-9]/g, "").slice(0, 3))}
            className="h-8 font-mono text-xs"
            inputMode="numeric"
          />
        </div>
        <div className="flex items-end gap-2">
          <Button size="sm" disabled={!online || busy} onClick={() => void handleWebcam()}>
            {capturing === "webcam" ? <Spinner size="xs" /> : <Camera className="size-4" />}
            {t("agents.media_webcam")}
          </Button>
          <Button size="sm" variant="outline" disabled={!online || busy} onClick={() => void handleMic()}>
            {capturing === "mic" ? <Spinner size="xs" /> : <Mic className="size-4" />}
            {t("agents.media_mic")}
          </Button>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{t("agents.media_hint")}</p>
    </CollectCard>
  );
});
