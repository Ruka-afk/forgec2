"use client";

import { useCallback, useState, useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PageHeader, PageSpinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { BinaryPanel, UnixPanel, StagerPanel, PS1Panel, ShellcodePanel, DonutPanel } from "./_components/BuildPanels";
import dynamic from "next/dynamic";
import { usePayloadGenerator } from "./hooks/usePayloadGenerator";
import type { PayloadKey } from "@/types/generate";
import { AppWindow, Cpu, Info, PackageOpen, X } from "lucide-react";
import { cn } from "@/lib/utils";

const ConnectionPanel = dynamic(() => import("./_components/ConnectionPanel"), { ssr: false });
const OneLinerPanel = dynamic(() => import("./_components/OneLinerPanel"), { ssr: false });
const QuickPresets = dynamic(() => import("./_components/QuickPresets"), { ssr: false });
const BuildHistorySection = dynamic(() => import("./_components/BuildHistorySection"), { ssr: false });

const BANNER_DISMISS_KEY = "forgec2_gen_banner_dismissed";

function SectionHeading({ icon, tint, title, desc, className }: { icon: ReactNode; tint: string; title: string; desc: string; className?: string }) {
  return (
    <div className={cn("flex items-center gap-x-3 mb-5", className)}>
      <div className={cn("w-10 h-10 rounded-xl ring-1 ring-border/50 flex items-center justify-center", tint)}>{icon}</div>
      <div>
        <div className="text-sm font-semibold text-foreground">{title}</div>
        <div className="text-xs text-muted-foreground">{desc}</div>
      </div>
    </div>
  );
}

export default function GeneratePage() {
  const { t } = useI18n();
  const g = usePayloadGenerator();
  const [showBanner, setShowBanner] = useState(true);
  const [historyRefresh, setHistoryRefresh] = useState(0);
  const prevBusyRef = useRef<Record<string, boolean>>({});

  useEffect(() => {
    try {
      const dismissed = localStorage.getItem(BANNER_DISMISS_KEY);
      if (dismissed === "true") setShowBanner(false);
    } catch { /* ignore */ }
  }, []);

  const states = g.states;
  const setForms = g.setForms;

  useEffect(() => {
    const prev = prevBusyRef.current;
    let anyCompleted = false;
    for (const key of Object.keys(states)) {
      if (prev[key] && !states[key as keyof typeof states].busy) {
        anyCompleted = true;
      }
    }
    if (anyCompleted) setHistoryRefresh((n) => n + 1);
    const next: Record<string, boolean> = {};
    for (const key of Object.keys(states)) {
      next[key] = states[key as keyof typeof states].busy;
    }
    prevBusyRef.current = next;
  }, [states]);

  const dismissBanner = () => {
    setShowBanner(false);
    try { localStorage.setItem(BANNER_DISMISS_KEY, "true"); } catch { /* ignore */ }
  };

  const makeDispatch = useCallback(<K extends PayloadKey>(key: K) => {
    return (valueOrFn: unknown) => {
      setForms((prev) => {
        const next = typeof valueOrFn === "function" ? valueOrFn(prev[key]) : valueOrFn;
        return { ...prev, [key]: next };
      });
    };
  }, [setForms]);

  if (g.loading) return <PageSpinner />;

  const agentCards = (
    <>
      <div className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "0ms" }}><BinaryPanel variant="exe" form={g.forms.exe} setForm={makeDispatch("exe")} busy={g.states.exe.busy} result={g.states.exe.result} onGenerate={g.handlerMap.exe} /></div>
      <div className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "40ms" }}><BinaryPanel variant="dll" form={g.forms.dll} setForm={makeDispatch("dll")} busy={g.states.dll.busy} result={g.states.dll.result} onGenerate={g.handlerMap.dll} /></div>
      <div className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "80ms" }}><PS1Panel form={g.forms.ps1} setForm={makeDispatch("ps1")} busy={g.states.ps1.busy} result={g.states.ps1.result} code={g.extras.ps1?.code} originalLen={g.extras.ps1?.original_length} obfuscatedLen={g.extras.ps1?.obfuscated_len} onGenerate={g.handlerMap.ps1} /></div>
      <div className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "120ms" }}><UnixPanel variant="linux" form={g.forms.linux} setForm={makeDispatch("linux")} busy={g.states.linux.busy} result={g.states.linux.result} onGenerate={g.handlerMap.linux} /></div>
      <div className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "160ms" }}><UnixPanel variant="macos" form={g.forms.macos} setForm={makeDispatch("macos")} busy={g.states.macos.busy} result={g.states.macos.result} onGenerate={g.handlerMap.macos} /></div>
    </>
  );

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("generate.title")} subtitle={t("generate.subtitle")} />

      {showBanner && (
        <div className="mt-3 flex items-center gap-2 px-4 py-2 bg-warning/10 border border-warning/20 rounded-xl">
          <Info className="w-4 h-4" />
          <span className="flex-1 text-xs text-warning-foreground">{t("generate.banner_text")} <a href="https://go.dev/dl/" target="_blank" className="underline hover:text-amber-800 dark:hover:text-amber-200 transition-colors">{t("generate.banner_download")}</a></span>
          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={dismissBanner} className="text-amber-500 hover:text-amber-700 dark:hover:text-amber-200" aria-label={t("generate.dismiss")} />}>
            <X className="w-4 h-4" />
            </TooltipTrigger>
            <TooltipContent>{t("generate.dismiss")}</TooltipContent>
          </Tooltip>
        </div>
      )}

      <div className="mt-5 grid grid-cols-1 lg:grid-cols-[320px_minmax(0,1fr)] gap-5 items-start">
        {/* ── Left rail: connection (sticky) ── */}
        <div className="min-w-0 lg:sticky lg:top-24 lg:max-h-[calc(100vh-8rem)] lg:overflow-y-auto">
          <ConnectionPanel
            listeners={g.listeners}
            shared={g.shared}
            profilePresets={g.profilePresets}
            profileLocked={g.profileLocked}
            showListenerModal={g.showListenerModal}
            listenerForm={g.listenerForm}
            setShared={g.setShared}
            changeProfile={g.changeProfile}
            handleCreateListener={g.handleCreateListener}
            submitListener={g.submitListener}
            setShowListenerModal={g.setShowListenerModal}
            setListenerForm={g.setListenerForm}
            onProfileDeleted={g.deleteProfile}
            fileInputRef={g.fileInputRef}
            onProfileImport={g.handleProfileImport}
          />
        </div>

        {/* ── Main workspace ── */}
        <div className="min-w-0 space-y-8">
          {/* Agent Binaries */}
          <section className="animate-fade-slide-up">
            <SectionHeading
              icon={<AppWindow className="w-4 h-4" />}
              tint="bg-primary/10 text-primary"
              title={t("generate.agents_title")}
              desc={t("generate.agents_desc")}
            />
            <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
              {agentCards}
            </div>
          </section>

          {/* Artifact Kit */}
          <section className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "60ms" }}>
            <SectionHeading
              icon={<PackageOpen className="w-4 h-4" />}
              tint="bg-violet-500/10 text-violet-600 dark:text-violet-400"
              title={t("generate.artifact_kit")}
              desc={t("generate.artifact_kit_desc")}
            />
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <StagerPanel variant="windows" form={g.forms.stager} setForm={makeDispatch("stager")} busy={g.states.stager.busy} result={g.states.stager.result} onGenerate={g.handlerMap.stager} />
              <StagerPanel variant="linux" form={g.forms.stager_linux} setForm={makeDispatch("stager_linux")} busy={g.states.stager_linux.busy} result={g.states.stager_linux.result} onGenerate={g.handlerMap.stager_linux} />
            </div>
          </section>

          {/* Shellcode / Donut */}
          <section className="opacity-0 animate-fade-slide-up" style={{ animationDelay: "120ms" }}>
            <SectionHeading
              icon={<Cpu className="w-4 h-4" />}
              tint="bg-amber-500/10 text-amber-600 dark:text-amber-400"
              title={t("generate.shellcode_donut")}
              desc={t("generate.shellcode_donut_desc")}
            />
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
              <ShellcodePanel form={g.forms.shellcode} setForm={makeDispatch("shellcode")} busy={g.states.shellcode.busy} result={g.states.shellcode.result} onGenerate={g.handlerMap.shellcode} />
              <DonutPanel form={g.forms.donut} setForm={makeDispatch("donut")} busy={g.states.donut.busy} result={g.states.donut.result} onGenerate={g.handlerMap.donut} fileRef={g.donutFileRef} />
            </div>
          </section>

          {/* One-Liner */}
          <OneLinerPanel
            form={g.forms.oneliner} setForm={makeDispatch("oneliner")}
            busy={g.states.oneliner.busy} result={g.states.oneliner.result}
            onelinerData={g.extras.oneliner?.data}
            listeners={g.listeners} getListenerInfo={g.getListenerInfo}
            onGenerate={g.handlerMap.oneliner}
          />

          <QuickPresets onApply={g.applyPreset} />
          <BuildHistorySection refreshKey={historyRefresh} />

          <div className="pt-4 text-center text-xs text-muted-foreground border-t border-border/60">
            {t("generate.footer_text")}
            <span className="block mt-1 text-amber-600 dark:text-amber-400">{t("generate.footer_warning")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
