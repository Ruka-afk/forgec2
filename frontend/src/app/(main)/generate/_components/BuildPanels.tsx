import React from "react";
import type { ReactNode } from "react";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { CopyButton } from "@/components/ui/copy-button";
import type { BinaryForm, UnixForm, PS1Form, StagerForm, ShellcodeForm, DonutForm, BinaryVariant, UnixVariant, StagerVariant } from "@/types/generate";
import { AppWindow, Apple, Binary, CheckCircle2, Disc, Download, HardDrive, Info, Package, PackageOpen, Puzzle, Terminal, Wand2 } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { AdvancedSection, FieldLabel, PayloadCard } from "./PayloadCard";
import { BuildResult, BuildStatusBadge } from "./BuildResult";

const BTN_CLASS = "w-full h-10 rounded-lg font-medium flex items-center justify-center gap-x-2 accent-gradient text-primary-foreground shadow-md shadow-primary/25 hover:shadow-lg hover:shadow-primary/35 hover:brightness-110 active:scale-[0.98] disabled:opacity-50";

// ─── BinaryPanel (exe / dll) ───────────────────────────────────

interface BinaryPanelProps {
  variant: BinaryVariant;
  form: BinaryForm;
  setForm: React.Dispatch<React.SetStateAction<BinaryForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
  canGenerate?: boolean;
}

const VARIANT_CONFIG: Record<BinaryVariant, { tint: string; icon: ReactNode; titleKey: string; subtitleKey: string; btnKey: string; showP2P: boolean }> = {
  exe: { tint: "bg-warning/10 text-warning", icon: <AppWindow className="w-5 h-5" />, titleKey: "generate.panel.exe_title", subtitleKey: "generate.panel.exe_subtitle", btnKey: "generate.panel.generate_exe", showP2P: true },
  dll: { tint: "bg-destructive/10 text-destructive", icon: <Puzzle className="w-5 h-5" />, titleKey: "generate.panel.dll_title", subtitleKey: "generate.panel.dll_subtitle", btnKey: "generate.panel.generate_dll", showP2P: false },
};

export const BinaryPanel = React.memo(function BinaryPanel({ variant, form, setForm, busy, result, onGenerate, canGenerate = true }: BinaryPanelProps) {
  const { t } = useI18n();
  const cfg = VARIANT_CONFIG[variant];
  const id = `binary-${variant}`;
  return (
    <PayloadCard
      icon={cfg.icon}
      tint={cfg.tint}
      title={t(cfg.titleKey)}
      subtitle={t(cfg.subtitleKey)}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Download className="w-4 h-4" /> {t(cfg.btnKey)}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.filename")}</FieldLabel>
        <Input aria-label={t("generate.panel.output_filename")} name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
      {cfg.showP2P && (
        <AdvancedSection title={t("generate.panel.p2p_config")}>
          <div>
            <FieldLabel>{t("generate.panel.mode")}</FieldLabel>
            <Select value={form.p2p_mode} onValueChange={(val) => val != null && setForm({ ...form, p2p_mode: val })}>
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t("generate.panel.direct")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{t("generate.panel.direct")}</SelectItem>
                <SelectItem value="parent">{t("generate.panel.p2p_parent")}</SelectItem>
                <SelectItem value="child">{t("generate.panel.p2p_child")}</SelectItem>
                <SelectItem value="dns">{t("generate.panel.dns_tunnel")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {form.p2p_mode === "child" && (
            <div>
              <FieldLabel>{t("generate.panel.parent_address")}</FieldLabel>
              <Input aria-label="tcp://192.168.1.100:4444" name={`${id}-parent`} type="text" placeholder="tcp://192.168.1.100:4444" value={form.p2p_parent} onChange={(e) => setForm({ ...form, p2p_parent: e.target.value })} className="font-mono text-xs" />
            </div>
          )}
          {form.p2p_mode === "parent" && (
            <div>
              <FieldLabel>{t("generate.panel.listen_address")}</FieldLabel>
              <Input aria-label="TCP: :4444 / SMB: pipe_name" name={`${id}-listen`} type="text" placeholder="TCP: :4444 / SMB: pipe_name" value={form.p2p_listen_addr} onChange={(e) => setForm({ ...form, p2p_listen_addr: e.target.value })} className="font-mono text-xs" />
            </div>
          )}
          {form.p2p_mode === "dns" && (
            <>
              <div>
                <FieldLabel>{t("generate.panel.dns_domain")}</FieldLabel>
                <Input aria-label="c2.example.com" name={`${id}-dns-domain`} type="text" placeholder="c2.example.com" value={form.dns_domain} onChange={(e) => setForm({ ...form, dns_domain: e.target.value })} className="font-mono text-xs" />
              </div>
              <div>
                <FieldLabel>{t("generate.panel.dns_server")}</FieldLabel>
                <Input aria-label="192.168.1.100" name={`${id}-dns-server`} type="text" placeholder="192.168.1.100" value={form.dns_server} onChange={(e) => setForm({ ...form, dns_server: e.target.value })} className="font-mono text-xs" />
              </div>
            </>
          )}
        </AdvancedSection>
      )}
      <div>
        <FieldLabel>{t("generate.panel.architecture")}</FieldLabel>
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
        <Checkbox id={`${id}-persist`} aria-label={t("generate.panel.persist_aria")} checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
        <Label htmlFor={`${id}-persist`} className="text-sm text-foreground">{t("generate.panel.persist")}</Label>
      </div>
      <div className="flex items-center gap-x-2">
        <Checkbox id={`${id}-skip-tls`} aria-label={t("generate.panel.skip_tls_aria")} checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
        <Label htmlFor={`${id}-skip-tls`} className="text-sm text-muted-foreground">{t("generate.panel.skip_tls")}</Label>
      </div>
      <div className="flex items-start gap-x-2">
        <Checkbox id={`${id}-evasion`} aria-label={t("generate.panel.edr_evasion_aria")} checked={form.evasion} onCheckedChange={(checked) => setForm({ ...form, evasion: checked === true })} />
        <Label htmlFor={`${id}-evasion`} className="text-sm text-muted-foreground">
          {t("generate.panel.edr_evasion")}
          <span className="block text-(--fs-micro-sm) text-muted-foreground font-normal">{t("generate.panel.edr_evasion_hint")}</span>
        </Label>
      </div>
      <div className="flex items-start gap-x-2">
        <Checkbox id={`${id}-ghost-mode`} aria-label={t("generate.panel.ghost_mode_aria")} checked={form.ghost_mode} onCheckedChange={(checked) => setForm({ ...form, ghost_mode: checked === true })} />
        <Label htmlFor={`${id}-ghost-mode`} className="text-sm text-muted-foreground">
          {t("generate.panel.ghost_mode")}
          <span className="block text-(--fs-micro-sm) text-muted-foreground font-normal">{t("generate.panel.ghost_mode_hint")}</span>
        </Label>
      </div>
      {variant === "exe" && (
        <div className="flex items-start gap-x-2">
          <Checkbox id={`${id}-obfuscate`} aria-label={t("generate.panel.obfuscate_aria")} checked={form.obfuscate} onCheckedChange={(checked) => setForm({ ...form, obfuscate: checked === true })} />
          <Label htmlFor={`${id}-obfuscate`} className="text-sm text-muted-foreground">
            {t("generate.panel.obfuscate")}
            <span className="block text-(--fs-micro-sm) text-muted-foreground font-normal">{t("generate.panel.obfuscate_hint")}</span>
          </Label>
        </div>
      )}
      <div>
        <FieldLabel>{t("generate.panel.domain_front")}</FieldLabel>
        <Input aria-label={t("generate.panel.domain_front_aria")} name={`${id}-domain-front`} value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder={t("generate.panel.domain_front_aria")} />
      </div>
      <AdvancedSection title={t("generate.panel.working_hours")}>
        <div className="grid grid-cols-3 gap-2">
          <div>
            <FieldLabel>{t("generate.panel.start_hhmm")}</FieldLabel>
            <Input aria-label={t("generate.panel.start_aria")} type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })} />
          </div>
          <div>
            <FieldLabel>{t("generate.panel.end_hhmm")}</FieldLabel>
            <Input aria-label={t("generate.panel.end_aria")} type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })} />
          </div>
          <div>
            <FieldLabel>{t("generate.panel.timezone")}</FieldLabel>
            <Input aria-label={t("generate.panel.timezone_aria")} placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono text-xs" />
          </div>
        </div>
      </AdvancedSection>
    </PayloadCard>
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
  canGenerate?: boolean;
}

const UNIX_CONFIG: Record<UnixVariant, { tint: string; icon: ReactNode; titleKey: string; subtitleKey: string; btnKey: string }> = {
  linux: { tint: "bg-success/10 text-success", icon: <HardDrive className="w-5 h-5" />, titleKey: "generate.panel.elf_title", subtitleKey: "generate.panel.unix_subtitle", btnKey: "generate.panel.generate_elf" },
  macos: { tint: "bg-primary/10 text-primary", icon: <Apple className="w-5 h-5" />, titleKey: "generate.panel.macos_title", subtitleKey: "generate.panel.unix_subtitle", btnKey: "generate.panel.generate_macos" },
};

export const UnixPanel = React.memo(function UnixPanel({ variant, form, setForm, busy, result, onGenerate, canGenerate = true }: UnixPanelProps) {
  const { t } = useI18n();
  const cfg = UNIX_CONFIG[variant];
  const id = `unix-${variant}`;
  return (
    <PayloadCard
      icon={cfg.icon}
      tint={cfg.tint}
      title={t(cfg.titleKey)}
      subtitle={t(cfg.subtitleKey)}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Download className="w-4 h-4" /> {t(cfg.btnKey)}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.filename")}</FieldLabel>
        <Input aria-label={t("generate.panel.output_filename")} name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
        <Checkbox id={`${id}-persist`} aria-label={t("generate.panel.persist_aria")} checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
        <Label htmlFor={`${id}-persist`} className="text-sm text-foreground">{t("generate.panel.persist")}</Label>
        <Checkbox id={`${id}-skip-tls`} aria-label={t("generate.panel.skip_tls_aria")} checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} className="ml-3" />
        <Label htmlFor={`${id}-skip-tls`} className="text-sm text-muted-foreground">{t("generate.panel.skip_tls_short")}</Label>
        <Checkbox id={`${id}-obfuscate`} aria-label={t("generate.panel.obfuscate_aria")} checked={form.obfuscate} onCheckedChange={(checked) => setForm({ ...form, obfuscate: checked === true })} className="ml-3" />
        <Label htmlFor={`${id}-obfuscate`} className="text-sm text-muted-foreground">{t("generate.panel.obfuscate_aria")}</Label>
      </div>
      <div>
        <FieldLabel>{t("generate.panel.domain_front")}</FieldLabel>
        <Input aria-label={t("generate.panel.domain_front_aria")} name={`${id}-domain-front`} value={form.domain_front} onChange={(e) => setForm({ ...form, domain_front: e.target.value })} placeholder={t("generate.panel.domain_front_aria")} />
      </div>
      <AdvancedSection title={t("generate.panel.working_hours")}>
        <div className="grid grid-cols-3 gap-2">
          <div>
            <FieldLabel>{t("generate.panel.start_hhmm")}</FieldLabel>
            <Input aria-label={t("generate.panel.start_aria")} type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })} />
          </div>
          <div>
            <FieldLabel>{t("generate.panel.end_hhmm")}</FieldLabel>
            <Input aria-label={t("generate.panel.end_aria")} type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })} />
          </div>
          <div>
            <FieldLabel>{t("generate.panel.timezone")}</FieldLabel>
            <Input aria-label={t("generate.panel.timezone_aria")} placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono text-xs" />
          </div>
        </div>
      </AdvancedSection>
    </PayloadCard>
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
  canGenerate?: boolean;
}

const STAGER_CONFIG: Record<StagerVariant, { tint: string; icon: ReactNode; titleKey: string; subtitleKey: string }> = {
  windows: { tint: "bg-chart-6/violet text-chart-6", icon: <PackageOpen className="w-5 h-5" />, titleKey: "generate.panel.stager_win_title", subtitleKey: "generate.panel.stager_subtitle" },
  linux: { tint: "bg-chart-2/10 text-chart-2 dark:text-chart-2", icon: <Package className="w-5 h-5" />, titleKey: "generate.panel.stager_linux_title", subtitleKey: "generate.panel.stager_subtitle" },
};

export const StagerPanel = React.memo(function StagerPanel({ variant, form, setForm, busy, result, onGenerate, canGenerate = true }: StagerPanelProps) {
  const { t } = useI18n();
  const cfg = STAGER_CONFIG[variant];
  const id = `stager-${variant}`;
  return (
    <PayloadCard
      icon={cfg.icon}
      tint={cfg.tint}
      title={t(cfg.titleKey)}
      subtitle={t(cfg.subtitleKey)}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Download className="w-4 h-4" /> {t("generate.panel.generate_loader")}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.output_filename")}</FieldLabel>
        <Input aria-label={t("generate.panel.output_filename")} name={`${id}-filename`} value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
      <div className="flex items-center gap-x-2">
        <Checkbox id={`${id}-skip-tls`} aria-label={t("generate.panel.skip_tls_aria")} checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
        <Label htmlFor={`${id}-skip-tls`} className="text-sm text-foreground">{t("generate.panel.skip_tls")}</Label>
      </div>
    </PayloadCard>
  );
});

// ─── ShellcodePanel ────────────────────────────────────────────

export const ShellcodePanel = React.memo(function ShellcodePanel({
  form, setForm, busy, result, onGenerate, canGenerate = true,
}: {
  form: ShellcodeForm;
  setForm: React.Dispatch<React.SetStateAction<ShellcodeForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
  canGenerate?: boolean;
}) {
  const { t } = useI18n();
  return (
    <PayloadCard
      icon={<Binary className="w-5 h-5" />}
      tint="bg-chart-2/10 text-info"
      title={t("generate.panel.shellcode_title")}
      subtitle={t("generate.panel.shellcode_subtitle")}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Download className="w-4 h-4" /> {t("generate.panel.generate_shellcode")}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.command")}</FieldLabel>
        <Input aria-label={t("generate.panel.command")} name="shellcode-cmd" value={form.command} onChange={(e) => setForm({ ...form, command: e.target.value })} className="font-mono text-xs" />
      </div>
      <div>
        <FieldLabel>{t("generate.panel.filename")}</FieldLabel>
        <Input aria-label={t("generate.panel.output_filename")} name="shellcode-filename" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
    </PayloadCard>
  );
});

// ─── DonutPanel ────────────────────────────────────────────────

export const DonutPanel = React.memo(function DonutPanel({
  form, setForm, busy, result, onGenerate, fileRef, canGenerate = true,
}: {
  form: DonutForm;
  setForm: React.Dispatch<React.SetStateAction<DonutForm>>;
  busy: boolean;
  result: ReactNode;
  onGenerate: () => void;
  fileRef: React.RefObject<HTMLInputElement | null>;
  canGenerate?: boolean;
}) {
  const { t } = useI18n();
  return (
    <PayloadCard
      icon={<Disc className="w-5 h-5" />}
      tint="bg-warning/10 text-warning"
      title={t("generate.panel.donut_title")}
      subtitle={t("generate.panel.donut_subtitle")}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Download className="w-4 h-4" /> {t("generate.panel.generate_donut")}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.dotnet_assembly")}</FieldLabel>
        <Input aria-label={t("generate.panel.upload_assembly_aria")} name="donut-assembly" ref={fileRef} type="file" accept=".exe,.dll" onChange={(e) => setForm({ ...form, assembly: e.target.files?.[0] || null })} />
      </div>
      <div>
        <FieldLabel>{t("generate.panel.arch")}</FieldLabel>
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
        <FieldLabel>{t("generate.panel.class_opt")}</FieldLabel>
        <Input aria-label={t("generate.panel.blank_for_main")} name="donut-class" type="text" placeholder={t("generate.panel.blank_for_main")} value={form.class} onChange={(e) => setForm({ ...form, class: e.target.value })} className="font-mono text-xs" />
      </div>
      <div>
        <FieldLabel>{t("generate.panel.method_opt")}</FieldLabel>
        <Input aria-label={t("generate.panel.main_aria")} name="donut-method" type="text" placeholder={t("generate.panel.main_aria")} value={form.method} onChange={(e) => setForm({ ...form, method: e.target.value })} className="font-mono text-xs" />
      </div>
      <div>
        <FieldLabel>{t("generate.panel.args_opt")}</FieldLabel>
        <Input aria-label={t("generate.panel.args_opt")} name="donut-args" type="text" placeholder="" value={form.args} onChange={(e) => setForm({ ...form, args: e.target.value })} className="font-mono text-xs" />
      </div>
      <div>
        <FieldLabel>{t("generate.panel.output_filename")}</FieldLabel>
        <Input aria-label={t("generate.panel.output_filename")} name="donut-filename" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
    </PayloadCard>
  );
});

// ─── PS1Panel ──────────────────────────────────────────────────

export const PS1Panel = React.memo(function PS1Panel({
  form, setForm, busy, result, code, originalLen, obfuscatedLen, onGenerate, canGenerate = true,
}: {
  form: PS1Form;
  setForm: React.Dispatch<React.SetStateAction<PS1Form>>;
  busy: boolean;
  result: ReactNode;
  code?: string;
  originalLen?: number;
  obfuscatedLen?: number;
  onGenerate: () => void;
  canGenerate?: boolean;
}) {
  const { t } = useI18n();
  return (
    <PayloadCard
      icon={<Terminal className="w-5 h-5" />}
      tint="bg-primary/10 text-primary"
      title={t("generate.panel.ps1_title")}
      subtitle={t("generate.panel.ps1_subtitle")}
      badge={<BuildStatusBadge busy={busy} result={result} />}
      footer={
        <>
          <Button type="button" onClick={onGenerate} disabled={busy || !canGenerate} title={!canGenerate ? t("generate.toast.select_listener_first") : undefined} className={BTN_CLASS}>
            {busy ? <><Spinner /> {t("generate.panel.generating")}</> : <><Wand2 className="w-4 h-4" /> {t("generate.panel.generate_ps1")}</>}
          </Button>
          <BuildResult busy={busy} result={result} />
        </>
      }
    >
      <div>
        <FieldLabel>{t("generate.panel.filename")}</FieldLabel>
        <Input id="ps1-filename" placeholder="agent.ps1" value={form.filename} onChange={(e) => setForm({ ...form, filename: e.target.value })} />
      </div>
      <div className="flex items-center gap-x-2">
        <Checkbox id="ps1-persist" aria-label={t("generate.panel.persist")} checked={form.persist} onCheckedChange={(checked) => setForm({ ...form, persist: checked === true })} />
        <Label htmlFor="ps1-persist" className="text-sm text-foreground">{t("generate.panel.persist")}</Label>
      </div>
      <div className="flex items-center gap-x-2">
        <Checkbox id="ps1-skip-tls" aria-label={t("generate.panel.skip_tls_aria")} checked={form.skip_tls} onCheckedChange={(checked) => setForm({ ...form, skip_tls: checked === true })} />
        <Label htmlFor="ps1-skip-tls" className="text-sm text-muted-foreground">{t("generate.panel.skip_tls")}</Label>
      </div>
      {code ? (
        <div className="mt-1">
          <div className="mb-2 flex items-center justify-between">
            <span className="flex items-center gap-x-1.5 text-xs font-medium text-success">
              <CheckCircle2 className="h-4 w-4" /> {t("generate.panel.generated_sizes", { original: originalLen ?? 0, obfuscated: obfuscatedLen ?? 0 })}
            </span>
            <CopyButton text={code} label={t("generate.panel.copy")} size="xs" />
          </div>
          <Textarea aria-label={t("generate.panel.ps1_title")} name="ps1-output" readOnly value={code} className="h-48 resize-none bg-background p-3 font-mono text-xs text-chart-1 border-border" />
          <div className="mt-1 flex items-center gap-x-1.5 text-xs text-muted-foreground"><Info className="h-4 w-4" /> {t("generate.panel.paste_ps")}</div>
        </div>
      ) : null}
    </PayloadCard>
  );
});
