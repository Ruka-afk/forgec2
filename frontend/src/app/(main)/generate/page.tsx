"use client";

import { useCallback, useState, useEffect, useRef } from "react";
import { PageHeader, PageSpinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import SharedSettings from "./_components/SharedSettings";
import { BinaryPanel, UnixPanel, StagerPanel, PS1Panel, ShellcodePanel, DonutPanel } from "./_components/BuildPanels";
import OneLinerPanel from "./_components/OneLinerPanel";
import QuickPresets from "./_components/QuickPresets";
import BuildHistorySection from "./_components/BuildHistorySection";
import { usePayloadGenerator } from "./hooks/usePayloadGenerator";
import type { PayloadKey } from "@/types/generate";
import { ChevronRight, Cpu, Info, PackageOpen, X } from "lucide-react";

const BANNER_DISMISS_KEY = "forgec2_gen_banner_dismissed";

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

  useEffect(() => {
    const prev = prevBusyRef.current;
    let anyCompleted = false;
    for (const key of Object.keys(g.states)) {
      if (prev[key] && !g.states[key as keyof typeof g.states].busy) {
        anyCompleted = true;
      }
    }
    if (anyCompleted) setHistoryRefresh((n) => n + 1);
    const next: Record<string, boolean> = {};
    for (const key of Object.keys(g.states)) {
      next[key] = g.states[key as keyof typeof g.states].busy;
    }
    prevBusyRef.current = next;
  }, [g.states]);

  const dismissBanner = () => {
    setShowBanner(false);
    try { localStorage.setItem(BANNER_DISMISS_KEY, "true"); } catch { /* ignore */ }
  };

  const makeDispatch = useCallback(<K extends PayloadKey>(key: K) => {
    return (valueOrFn: unknown) => {
      g.setForms((prev) => {
        const next = typeof valueOrFn === "function" ? valueOrFn(prev[key]) : valueOrFn;
        return { ...prev, [key]: next };
      });
    };
  }, [g.setForms]);

  if (g.loading) return <PageSpinner />;

  return (
    <div className="max-w-[80rem] mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("generate.title")} subtitle={t("generate.subtitle")} />

      {showBanner && (
        <div className="mt-3 flex items-center gap-2 px-4 py-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-700 rounded-xl">
          <Info className="w-4 h-4" />
          <span className="flex-1 text-xs text-amber-700 dark:text-amber-300">{t("generate.banner_text")} <a href="https://go.dev/dl/" target="_blank" className="underline hover:text-amber-800 dark:hover:text-amber-200 transition-colors">{t("generate.banner_download")}</a></span>
          <Button variant="ghost" size="icon-sm" onClick={dismissBanner} className="text-amber-500 hover:text-amber-700 dark:hover:text-amber-200" title={t("generate.dismiss")} aria-label={t("generate.dismiss")}>
            <X className="w-4 h-4" />
          </Button>
        </div>
      )}

      <div className="flex items-center gap-3 mt-5 mb-3 text-xs">
        <div className="flex items-center gap-1.5">
          <span className="w-5 h-5 rounded-full bg-primary text-primary-foreground flex items-center justify-center text-[10px] font-semibold">1</span>
          <span className="text-foreground font-medium">{t("generate.step_config")}</span>
        </div>
        <ChevronRight className="w-4 h-4" />
        <div className="flex items-center gap-1.5">
          <span className="w-5 h-5 rounded-full bg-muted-foreground text-white flex items-center justify-center text-[10px] font-semibold">2</span>
          <span className="text-muted-foreground">{t("generate.step_payload")}</span>
        </div>
      </div>

      <SharedSettings
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
      />

      <input aria-label="Import profile JSON file" name="profile-import" ref={g.fileInputRef} type="file" accept=".json,application/json" className="hidden" onChange={g.handleProfileImport} />

      {/* Primary Payloads */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-5 gap-5">
        <div className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "0ms" }}><BinaryPanel variant="exe" form={g.forms.exe} setForm={makeDispatch("exe")} busy={g.states.exe.busy} result={g.states.exe.result} onGenerate={g.handlerMap.exe} /></div>
        <div className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "40ms" }}><BinaryPanel variant="dll" form={g.forms.dll} setForm={makeDispatch("dll")} busy={g.states.dll.busy} result={g.states.dll.result} onGenerate={g.handlerMap.dll} /></div>
        <div className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "80ms" }}><PS1Panel form={g.forms.ps1} setForm={makeDispatch("ps1")} busy={g.states.ps1.busy} result={g.states.ps1.result} code={g.extras.ps1?.code} originalLen={g.extras.ps1?.original_length} obfuscatedLen={g.extras.ps1?.obfuscated_len} onGenerate={g.handlerMap.ps1} onCopy={g.copyToClipboard} /></div>
        <div className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "120ms" }}><UnixPanel variant="linux" form={g.forms.linux} setForm={makeDispatch("linux")} busy={g.states.linux.busy} result={g.states.linux.result} onGenerate={g.handlerMap.linux} /></div>
        <div className="opacity-0 animate-[fadeSlideUp_0.35s_cubic-bezier(0.16,1,0.3,1)_forwards]" style={{ animationDelay: "160ms" }}><UnixPanel variant="macos" form={g.forms.macos} setForm={makeDispatch("macos")} busy={g.states.macos.busy} result={g.states.macos.result} onGenerate={g.handlerMap.macos} /></div>
      </div>

      {/* Artifact Kit */}
      <div className="mt-8">
        <div className="flex items-center gap-x-3 mb-5 animate-fade-slide-up">
          <div className="w-10 h-10 bg-indigo-100 dark:bg-indigo-900/30 rounded-xl flex items-center justify-center"><PackageOpen className="w-4 h-4" /></div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("generate.artifact_kit")}</div>
            <div className="text-xs text-muted-foreground">{t("generate.artifact_kit_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <StagerPanel variant="windows" form={g.forms.stager} setForm={makeDispatch("stager")} busy={g.states.stager.busy} result={g.states.stager.result} onGenerate={g.handlerMap.stager} />
          <StagerPanel variant="linux" form={g.forms.stager_linux} setForm={makeDispatch("stager_linux")} busy={g.states.stager_linux.busy} result={g.states.stager_linux.result} onGenerate={g.handlerMap.stager_linux} />
        </div>
      </div>

      {/* Shellcode / Donut */}
      <div className="mt-8">
        <div className="flex items-center gap-x-3 mb-5 animate-fade-slide-up">
          <div className="w-10 h-10 bg-yellow-100 dark:bg-yellow-900/30 rounded-xl flex items-center justify-center"><Cpu className="w-4 h-4" /></div>
          <div>
            <div className="text-sm font-semibold text-foreground">{t("generate.shellcode_donut")}</div>
            <div className="text-xs text-muted-foreground">{t("generate.shellcode_donut_desc")}</div>
          </div>
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
          <ShellcodePanel form={g.forms.shellcode} setForm={makeDispatch("shellcode")} busy={g.states.shellcode.busy} result={g.states.shellcode.result} onGenerate={g.handlerMap.shellcode} />
          <DonutPanel form={g.forms.donut} setForm={makeDispatch("donut")} busy={g.states.donut.busy} result={g.states.donut.result} onGenerate={g.handlerMap.donut} fileRef={g.donutFileRef} />
        </div>
      </div>

      {/* One-Liner */}
      <OneLinerPanel
        form={g.forms.oneliner} setForm={makeDispatch("oneliner")}
        busy={g.states.oneliner.busy} result={g.states.oneliner.result}
        onelinerData={g.extras.oneliner?.data}
        listeners={g.listeners} getListenerInfo={g.getListenerInfo}
        onGenerate={g.handlerMap.oneliner} onCopy={g.copyToClipboard}
      />

      <QuickPresets onApply={g.applyPreset} />
      <BuildHistorySection refreshKey={historyRefresh} />

      <div className="mt-6 text-center text-xs text-muted-foreground">
        {t("generate.footer_text")}
        <span className="block mt-1 text-amber-600 dark:text-amber-400">{t("generate.footer_warning")}</span>
      </div>
    </div>
  );
}
