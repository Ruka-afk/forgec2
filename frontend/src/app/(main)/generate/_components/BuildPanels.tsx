import React from "react";
import type { ReactNode } from "react";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Alert, AlertDescription } from "@/components/ui/alert";
import type { BinaryForm, UnixForm, PS1Form, StagerForm, ShellcodeForm, DonutForm, BinaryVariant, UnixVariant, StagerVariant } from "@/types/generate";
import { CheckCircle, Copy, Download, Info, Link, Wand2 } from "lucide-react";

function PanelHeader({ bg, icon, title, subtitle }: { bg: string; icon: string; title: string; subtitle: string }) {
  return (
    <div className="flex items-center gap-x-3 mb-4 pb-4 border-b border-border">
      <div className={`w-11 h-11 ${bg} rounded-xl flex items-center justify-center text-2xl`}>{icon}</div>
      <div>
        <div className="font-semibold text-base text-foreground">{title}</div>
        <div className="text-xs text-muted-foreground">{subtitle}</div>
      </div>
    </div>
  );
}

function ResultDisplay({ result }: { result: ReactNode }) {
  if (!result) return null;
  return (
    <Alert variant="destructive" className="mt-3">
      <AlertDescription>
        <pre className="whitespace-pre-wrap font-mono">{result}</pre>
      </AlertDescription>
    </Alert>
  );
}

// ─── BinaryPanel (exe / dll) ───────────────────────────────────

interface BinaryPanelProps {
  variant: BinaryVariant;
  form: BinaryForm;
  setForm: React.Dispatch<React.SetStateAction<BinaryForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
}

const VARIANT_CONFIG: Record<BinaryVariant, { bg: string; icon: string; title: string; subtitle: string; btnColor: string; btnLabel: string; showP2P: boolean }> = {
  exe: { bg: "bg-amber-500/10", icon: "\u{1F5A5}\uFE0F", title: "Windows EXE", subtitle: "Native Go payload", btnColor: "bg-warning hover:bg-warning/80 text-white", btnLabel: "Generate EXE", showP2P: true },
  dll: { bg: "bg-destructive/10", icon: "🧩", title: "Windows DLL", subtitle: "rundll32 / regsvr32 / LoadLibrary", btnColor: "bg-destructive hover:bg-destructive/90", btnLabel: "Generate DLL", showP2P: false },
};

export const BinaryPanel = React.memo(function BinaryPanel({ variant, form, setForm, busy, result, onGenerate }: BinaryPanelProps) {
  const cfg = VARIANT_CONFIG[variant];
  const id = `binary-${variant}`;
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg={cfg.bg} icon={cfg.icon} title={cfg.title} subtitle={cfg.subtitle} />
      <div className="space-y-3">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Filename</span>
          <Input aria-label="Output filename" name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
        </div>
        {cfg.showP2P && (
          <div className="mt-2 pt-2 border-t border-border">
            <details className="group">
              <summary className="text-xs text-muted-foreground cursor-pointer hover:text-indigo-600 transition-colors select-none"><Link className="w-4 h-4" /> P2P / DNS Config (opt)</summary>
              <div className="mt-2 space-y-2">
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Mode</span>
                  <Select value={form.p2p_mode} onValueChange={(val) => val != null && setForm({ ...form, p2p_mode: val })}>
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Direct (HTTP/TCP)" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="">Direct (HTTP/TCP)</SelectItem>
                      <SelectItem value="parent">P2P Parent</SelectItem>
                      <SelectItem value="child">P2P Child</SelectItem>
                      <SelectItem value="dns">DNS Tunnel</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {form.p2p_mode === "child" && (
                  <div>
                    <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Parent Address</span>
                    <Input aria-label="tcp://192.168.1.100:4444" name={`${id}-parent`} type="text" placeholder="tcp://192.168.1.100:4444" value={form.p2p_parent} onChange={(e) => setForm({ ...form, p2p_parent: e.target.value })} className="font-mono text-xs" />
                  </div>
                )}
                {form.p2p_mode === "parent" && (
                  <div>
                    <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Listen Address</span>
                    <Input aria-label="TCP: :4444 / SMB: pipe_name" name={`${id}-listen`} type="text" placeholder="TCP: :4444 / SMB: pipe_name" value={form.p2p_listen_addr} onChange={(e) => setForm({ ...form, p2p_listen_addr: e.target.value })} className="font-mono text-xs" />
                  </div>
                )}
                {form.p2p_mode === "dns" && (
                  <div className="space-y-2">
                    <div>
                      <span className="block text-xs font-semibold text-muted-foreground mb-1.5">DNS Domain</span>
                      <Input aria-label="c2.example.com" name={`${id}-dns-domain`} type="text" placeholder="c2.example.com" value={form.dns_domain} onChange={(e) => setForm({ ...form, dns_domain: e.target.value })} className="font-mono text-xs" />
                    </div>
                    <div>
                      <span className="block text-xs font-semibold text-muted-foreground mb-1.5">DNS Server</span>
                      <Input aria-label="192.168.1.100" name={`${id}-dns-server`} type="text" placeholder="192.168.1.100" value={form.dns_server} onChange={(e) => setForm({ ...form, dns_server: e.target.value })} className="font-mono text-xs" />
                    </div>
                  </div>
                )}
              </div>
            </details>
          </div>
        )}
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Architecture</span>
          <Select value={form.arch} onValueChange={(val) => val != null && setForm({ ...form, arch: val })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="amd64">x64 (amd64)</SelectItem>
              <SelectItem value="arm64">ARM64</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id={`${id}-persist`} aria-label="Enable persistence" checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
          <Label htmlFor={`${id}-persist`} className="text-sm text-foreground">Persist</Label>
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id={`${id}-skip-tls`} aria-label="Skip TLS verification" checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
          <Label htmlFor={`${id}-skip-tls`} className="text-sm text-muted-foreground">Skip TLS Verify</Label>
        </div>
        <div className="flex items-start gap-x-2">
          <Checkbox id={`${id}-evasion`} aria-label="EDR Evasion" checked={form.evasion} onCheckedChange={(checked) => setForm({ ...form, evasion: checked === true })} />
          <Label htmlFor={`${id}-evasion`} className="text-sm text-muted-foreground">
            EDR Evasion (random sleep)
            <span className="block text-(--font-size-micro-sm) text-muted-foreground font-normal">Set FORGEC2_EVASION=1 at runtime</span>
          </Label>
        </div>
        {variant === "exe" && (
          <div className="flex items-start gap-x-2">
            <Checkbox id={`${id}-obfuscate`} aria-label="Obfuscate" checked={form.obfuscate} onCheckedChange={(checked) => setForm({ ...form, obfuscate: checked === true })} />
            <Label htmlFor={`${id}-obfuscate`} className="text-sm text-muted-foreground">
              Obfuscate (garble)
              <span className="block text-(--font-size-micro-sm) text-muted-foreground font-normal">Strip symbols + build ID, hide literals</span>
            </Label>
          </div>
        )}
        <div>
          <span className="block text-sm text-muted-foreground mb-1">Domain Front (CDN host)</span>
          <Input aria-label="e.g. cdn.cloudflare.com" name={`${id}-domain-front`} value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder="e.g. cdn.cloudflare.com" className="text-sm" />
        </div>
        <div className="mt-2 pt-2 border-t border-border">
          <details className="group">
            <summary className="text-xs text-muted-foreground cursor-pointer hover:text-indigo-600 transition-colors select-none">Working Hours (opt)</summary>
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">Start (HH:MM)</span>
                  <Input aria-label="Working start time" type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })} className="text-xs" />
                </div>
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">End (HH:MM)</span>
                  <Input aria-label="Working end time" type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })} className="text-xs" />
                </div>
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">Timezone</span>
                  <Input aria-label="Timezone" placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono text-xs" />
                </div>
              </div>
            </div>
          </details>
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className={`w-full h-10 ${cfg.btnColor} disabled:opacity-50 transition-colors text-destructive-foreground font-medium rounded-xl flex items-center justify-center gap-x-2`}>
          {busy ? <><Spinner /> Generating...</> : <><Download className="w-4 h-4" /> {cfg.btnLabel}</>}
        </Button>
      </div>
      <ResultDisplay result={result} />
    </Card>
  );
});

// ─── UnixPanel (linux / macos) ─────────────────────────────────

interface UnixPanelProps {
  variant: UnixVariant;
  form: UnixForm;
  setForm: React.Dispatch<React.SetStateAction<UnixForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
}

const UNIX_CONFIG: Record<UnixVariant, { bg: string; icon: string; title: string; subtitle: string; btnColor: string; btnLabel: string }> = {
  linux: { bg: "bg-emerald-100 dark:bg-emerald-900/30", icon: "🐧", title: "Linux ELF", subtitle: "Native Go payload / amd64", btnColor: "bg-emerald-600 hover:bg-emerald-700", btnLabel: "Generate ELF" },
  macos: { bg: "bg-primary/10", icon: "🍏", title: "macOS Binary", subtitle: "Native Go payload / amd64", btnColor: "bg-primary hover:bg-primary/90 text-primary-foreground", btnLabel: "Generate macOS" },
};

export const UnixPanel = React.memo(function UnixPanel({ variant, form, setForm, busy, result, onGenerate }: UnixPanelProps) {
  const cfg = UNIX_CONFIG[variant];
  const id = `unix-${variant}`;
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg={cfg.bg} icon={cfg.icon} title={cfg.title} subtitle={cfg.subtitle} />
      <div className="space-y-3">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Filename</span>
          <Input aria-label="Output filename" name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id={`${id}-persist`} aria-label="Enable persistence" checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
          <Label htmlFor={`${id}-persist`} className="text-sm text-foreground">Persist</Label>
          <Checkbox id={`${id}-skip-tls`} aria-label="Skip TLS verification" checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} className="ml-3" />
          <Label htmlFor={`${id}-skip-tls`} className="text-sm text-muted-foreground">Skip TLS</Label>
          <Checkbox id={`${id}-obfuscate`} aria-label="Obfuscate" checked={form.obfuscate} onCheckedChange={(checked) => setForm({ ...form, obfuscate: checked === true })} className="ml-3" />
          <Label htmlFor={`${id}-obfuscate`} className="text-sm text-muted-foreground">Obfuscate</Label>
        </div>
        <div>
          <span className="block text-sm text-muted-foreground mb-1">Domain Front (CDN host)</span>
          <Input aria-label="e.g. cdn.cloudflare.com" name={`${id}-domain-front`} value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder="e.g. cdn.cloudflare.com" className="text-sm" />
        </div>
        <div className="mt-2 pt-2 border-t border-border">
          <details className="group">
            <summary className="text-xs text-muted-foreground cursor-pointer hover:text-indigo-600 transition-colors select-none">Working Hours (opt)</summary>
            <div className="mt-2 space-y-2">
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">Start (HH:MM)</span>
                  <Input aria-label="Working start time" type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })} className="text-xs" />
                </div>
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">End (HH:MM)</span>
                  <Input aria-label="Working end time" type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })} className="text-xs" />
                </div>
                <div>
                  <span className="block text-xs font-semibold text-muted-foreground mb-1">Timezone</span>
                  <Input aria-label="Timezone" placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono text-xs" />
                </div>
              </div>
            </div>
          </details>
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className={`w-full h-10 ${cfg.btnColor} disabled:opacity-50 text-white font-medium rounded-xl flex items-center justify-center gap-x-2`}>
          {busy ? <><Spinner /> Generating...</> : <><Download className="w-4 h-4" /> {cfg.btnLabel}</>}
        </Button>
      </div>
      <ResultDisplay result={result} />
    </Card>
  );
});

// ─── StagerPanel (windows / linux) ─────────────────────────────

interface StagerPanelProps {
  variant: StagerVariant;
  form: StagerForm;
  setForm: React.Dispatch<React.SetStateAction<StagerForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
}

const STAGER_CONFIG: Record<StagerVariant, { bg: string; title: string; subtitle: string; btnColor: string }> = {
  windows: { bg: "bg-primary/10", title: "Windows Stager", subtitle: "XOR loader + remote implant", btnColor: "bg-primary hover:bg-primary/90 text-primary-foreground" },
  linux: { bg: "bg-teal-100 dark:bg-teal-900/30", title: "Linux Stager", subtitle: "XOR loader + remote implant", btnColor: "bg-teal-600 hover:bg-teal-700" },
};

export const StagerPanel = React.memo(function StagerPanel({ variant, form, setForm, busy, result, onGenerate }: StagerPanelProps) {
  const cfg = STAGER_CONFIG[variant];
  const id = `stager-${variant}`;
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg={cfg.bg} icon="📦" title={cfg.title} subtitle={cfg.subtitle} />
      <div className="space-y-4">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Output Filename</span>
          <Input aria-label="Output filename" name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id={`${id}-skip-tls`} aria-label="Skip TLS verification" checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
          <Label htmlFor={`${id}-skip-tls`} className="text-sm text-foreground">Skip TLS Verify</Label>
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className={`w-full h-10 ${cfg.btnColor} disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2`}>
          {busy ? <><Spinner /> Generating...</> : <><Download className="w-4 h-4" /> Generate Loader</>}
        </Button>
      </div>
      <ResultDisplay result={result} />
    </Card>
  );
});

// ─── ShellcodePanel ────────────────────────────────────────────

export const ShellcodePanel = React.memo(function ShellcodePanel({
  form, setForm, busy, result, onGenerate,
}: {
  form: ShellcodeForm;
  setForm: React.Dispatch<React.SetStateAction<ShellcodeForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
}) {
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg="bg-amber-100 dark:bg-amber-900/30" icon="💻" title="Raw Shellcode" subtitle="WinExec + PowerShell" />
      <div className="space-y-4">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Command</span>
          <Input aria-label="Shellcode command" name="shellcode-cmd" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Filename</span>
          <Input aria-label="Output filename" name="shellcode-filename" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-warning hover:bg-warning/80 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><Spinner /> Generating...</> : <><Download className="w-4 h-4" /> Generate Shellcode</>}
        </Button>
      </div>
      <ResultDisplay result={result} />
    </Card>
  );
});

// ─── DonutPanel ────────────────────────────────────────────────

export const DonutPanel = React.memo(function DonutPanel({
  form, setForm, busy, result, onGenerate, fileRef,
}: {
  form: DonutForm;
  setForm: React.Dispatch<React.SetStateAction<DonutForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
  fileRef: React.RefObject<HTMLInputElement | null>;
}) {
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg="bg-amber-500/10" icon="🍩" title="Donut Loader" subtitle="Native .NET to PIC shellcode" />
      <div className="space-y-4">
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">.NET Assembly (EXE/DLL)</span>
          <input aria-label="Upload .NET assembly file" name="donut-assembly" ref={fileRef} type="file" accept=".exe,.dll" onChange={(e) => setForm({ ...form, assembly: e.target.files?.[0] || null })} className="w-full text-sm file:mr-3 file:py-2 file:px-4 file:rounded-xl file:border-0 file:text-sm file:font-semibold file:bg-indigo-50 file:text-indigo-700 hover:file:bg-indigo-100 dark:file:bg-indigo-900/30 dark:file:text-indigo-300 dark:hover:file:bg-indigo-800/40" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Arch</span>
          <Select value={form.arch} onValueChange={(val) => val != null && setForm({ ...form, arch: val })}>
            <SelectTrigger className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="amd64">x64 (amd64)</SelectItem>
              <SelectItem value="x86">x86</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Class (opt)</span>
          <Input aria-label="Leave blank for Main" name="donut-class" type="text" placeholder="Leave blank for Main" value={form.class} onChange={(e) => setForm({ ...form, class: e.target.value })} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Method (opt)</span>
          <Input aria-label="Main" name="donut-method" type="text" placeholder="Main" value={form.method} onChange={(e) => setForm({ ...form, method: e.target.value })} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Args (opt)</span>
          <Input aria-label="Arguments" name="donut-args" type="text" placeholder="" value={form.args} onChange={(e) => setForm({ ...form, args: e.target.value })} className="font-mono text-xs" />
        </div>
        <div>
          <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Output Filename</span>
          <Input aria-label="Output filename" name="donut-filename" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-warning hover:bg-warning/80 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><Spinner /> Generating...</> : <><Download className="w-4 h-4" /> Generate Donut</>}
        </Button>
      </div>
      <ResultDisplay result={result} />
    </Card>
  );
});

// ─── PS1Panel ──────────────────────────────────────────────────

export const PS1Panel = React.memo(function PS1Panel({
  form, setForm, busy, result, code, originalLen, obfuscatedLen, onGenerate, onCopy,
}: {
  form: PS1Form;
  setForm: React.Dispatch<React.SetStateAction<PS1Form>>;
  busy: boolean;
  result: ReactNode;
  code?: string;
  originalLen?: number;
  obfuscatedLen?: number;
  onGenerate: () => void;
  onCopy: (text: string) => void;
}) {
  return (
    <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
      <PanelHeader bg="bg-primary/10" icon="📜" title="PowerShell Script" subtitle="Run in memory / fileless" />
      <div className="space-y-3">
        <div>
          <Label htmlFor="ps1-filename" className="text-sm text-muted-foreground mb-1 block">Filename</Label>
          <Input id="ps1-filename" placeholder="agent.ps1" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} className="h-9 bg-background border-border" />
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id="ps1-persist" aria-label="Persist" checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
          <Label htmlFor="ps1-persist" className="text-sm text-foreground">Persist</Label>
        </div>
        <div className="flex items-center gap-x-2">
          <Checkbox id="ps1-skip-tls" aria-label="Skip TLS verification" checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
          <Label htmlFor="ps1-skip-tls" className="text-sm text-muted-foreground">Skip TLS Verify</Label>
        </div>
        <Button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-primary hover:bg-primary/90 disabled:opacity-50 text-primary-foreground font-medium rounded-xl flex items-center justify-center gap-x-2">
          {busy ? <><Spinner /> Generating...</> : <><Wand2 className="w-4 h-4" /> Generate PS1</>}
        </Button>
      </div>
      {code ? (
        <div className="mt-3">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs text-emerald-600 font-medium">
              <CheckCircle className="w-4 h-4" /> Generated: {originalLen} B / Obfuscated: {obfuscatedLen} B
            </span>
            <Button variant="outline" size="xs" onClick={() => onCopy(code)}>
              <Copy className="w-4 h-4" /> Copy
            </Button>
          </div>
          <Textarea aria-label="Generated PS1 output" name="ps1-output" readOnly value={code} className="h-48 bg-background text-emerald-400 font-mono text-xs p-3 border-border resize-none" />
          <div className="mt-1 text-xs text-muted-foreground"><Info className="w-4 h-4" /> Paste directly into PowerShell</div>
        </div>
      ) : result ? (
        <ResultDisplay result={result} />
      ) : null}
    </Card>
  );
});
