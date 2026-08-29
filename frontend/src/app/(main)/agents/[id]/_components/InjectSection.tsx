"use client";

import { memo, useRef, useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Banner } from "@/components/ui/banner";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Syringe, AlertTriangle, Upload } from "lucide-react";

interface InjectSectionProps {
  agentId: string;
  online: boolean;
  osType?: string;
}

const WINDOWS_TECHNIQUES = [
  { value: "createremotethread", label: "CreateRemoteThread" },
  { value: "ntcreatethreadex", label: "NtCreateThreadEx (direct syscall)" },
  { value: "ntcreatethreadex_indirect", label: "NtCreateThreadEx (indirect syscall)" },
  { value: "apc", label: "QueueUserAPC" },
  { value: "earlybird", label: "Early Bird (suspended + APC)" },
  { value: "threadless", label: "Threadless (SetThreadContext RIP)" },
  { value: "syscall", label: "Hell's Gate direct syscall" },
  { value: "indirect", label: "Indirect syscall (ntdll gadget)" },
  { value: "hollow", label: "Process hollowing" },
  { value: "hijack", label: "Thread hijacking" },
  { value: "atom", label: "Atom bombing" },
  { value: "txf", label: "Transacted hollowing (TxF)" },
  { value: "stomp", label: "Module stomping" },
];

const LINUX_TECHNIQUES = [
  { value: "ptrace", label: "Ptrace POKEDATA" },
  { value: "mem", label: "/proc/pid/mem write" },
  { value: "process_vm_writev", label: "process_vm_writev" },
  { value: "ld_preload", label: "LD_PRELOAD" },
];

const DARWIN_TECHNIQUES = [
  { value: "ptrace", label: "Ptrace attach + exception" },
  { value: "task_for_pid", label: "Mach VM via task_for_pid" },
];

function techniquesFor(os?: string) {
  const o = (os || "").toLowerCase();
  if (o.includes("linux")) return LINUX_TECHNIQUES;
  if (o.includes("darwin") || o.includes("mac")) return DARWIN_TECHNIQUES;
  return WINDOWS_TECHNIQUES;
}

/**
 * InjectSection — queue a shellcode injection task against a target PID.
 * The shellcode is base64-encoded in the browser and sent as task data; the
 * agent executes it with the selected technique (see task_injection.go).
 */
export default memo(function InjectSection({ agentId, online, osType }: InjectSectionProps) {
  const { t } = useI18n();
  const [pid, setPid] = useState("");
  const [technique, setTechnique] = useState("");
  const [shellcodeB64, setShellcodeB64] = useState("");
  const [shellcodeName, setShellcodeName] = useState("");
  const [sending, setSending] = useState(false);
  const [confirmStep, setConfirmStep] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const techniques = techniquesFor(osType);

  const handleFile = (file: File | undefined | null) => {
    if (!file) return;
    if (file.size > 3 * 1024 * 1024) {
      toast.error(t("agents.inject_too_large"));
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const buf = reader.result as ArrayBuffer;
      const bytes = new Uint8Array(buf);
      let binary = "";
      const CHUNK = 0x8000;
      for (let i = 0; i < bytes.length; i += CHUNK) {
        binary += String.fromCharCode(...bytes.subarray(i, i + CHUNK));
      }
      setShellcodeB64(btoa(binary));
      setShellcodeName(`${file.name} (${(file.size / 1024).toFixed(1)} KB)`);
    };
    reader.onerror = () => toast.error(t("agents.inject_read_failed"));
    reader.readAsArrayBuffer(file);
  };

  const handleSubmit = async () => {
    // pid must be a positive integer: it is posted as a raw string and a
    // garbage value ("abc", "-1") went straight to the agent dispatcher.
    if (!/^\d+$/.test(pid.trim()) || Number(pid) <= 0) return;
    if (!technique || !shellcodeB64) return;
    if (!confirmStep) {
      setConfirmStep(true);
      return;
    }
    setSending(true);
    try {
      // The existing POST /agents/:id/inject endpoint takes form-encoded
      // fields: pid, tech, shellcode (base64).
      await api.post(paths.agents.inject(agentId), {
        pid: pid.trim(),
        tech: technique,
        shellcode: shellcodeB64,
      });
      toast.success(t("agents.inject_queued"));
      // Reset the form so a double-click cannot re-fire the same payload.
      setPid("");
      setTechnique("");
      setShellcodeB64("");
      setShellcodeName("");
      setConfirmStep(false);
      if (fileRef.current) fileRef.current.value = "";
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("agents.inject_failed"));
    } finally {
      setSending(false);
    }
  };

  return (
    <Card className="mb-4 gap-0">
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-sm font-semibold text-foreground"><Syringe className="size-3.5" />{t("agents.inject_title")}</h3>
      </div>
      <div className="p-3 space-y-3">
        <p className="text-xs text-muted-foreground">{t("agents.inject_desc")}</p>
        {confirmStep && (
          <Banner tone="destructive" icon={<AlertTriangle className="size-3.5" />} className="text-xs">
            {t("agents.inject_confirm_msg")}
          </Banner>
        )}
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          <div>
            <Label className="text-xs text-muted-foreground">{t("agents.inject_pid")}</Label>
            <Input type="number" min={1} value={pid} onChange={(e) => setPid(e.target.value)} placeholder="4820" className="mt-1 h-9 font-mono" />
          </div>
          <div>
            <Label className="text-xs text-muted-foreground">{t("agents.inject_technique")}</Label>
            <Select value={technique} onValueChange={(v) => { if (v) { setTechnique(v); setConfirmStep(false); } }}>
              <SelectTrigger className="mt-1"><SelectValue placeholder={t("agents.inject_select_tech")} /></SelectTrigger>
              <SelectContent>
                {techniques.map((tech) => (
                  <SelectItem key={tech.value} value={tech.value}>{tech.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div>
          <Label className="text-xs text-muted-foreground">{t("agents.inject_shellcode")}</Label>
          <div className="mt-1 flex items-center gap-2">
            <input
              ref={fileRef}
              type="file"
              accept=".bin,.raw,.dll,.so,.exe"
              className="hidden"
              onChange={(e) => handleFile(e.target.files?.[0])}
            />
            <Button variant="outline" size="sm" onClick={() => fileRef.current?.click()} disabled={!online}>
              <Upload className="size-4" /> {t("agents.inject_pick_file")}
            </Button>
            <span className="text-xs text-muted-foreground truncate">{shellcodeName || t("agents.inject_no_file")}</span>
          </div>
        </div>

        <Button
          onClick={handleSubmit}
          disabled={!online || sending || !pid || !technique || !shellcodeB64}
          variant={confirmStep ? "destructive" : "default"}
          className="w-full"
        >
          <Syringe className="size-4" />
          {sending ? t("agents.inject_sending") : confirmStep ? t("agents.evasion_confirm") : t("agents.inject_execute")}
        </Button>
      </div>
    </Card>
  );
});
