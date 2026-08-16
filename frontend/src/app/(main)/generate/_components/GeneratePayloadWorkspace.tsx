"use client";

import { useCallback, useState, useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PageSpinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { BinaryPanel, UnixPanel, StagerPanel, PS1Panel, ShellcodePanel, DonutPanel } from "./BuildPanels";
import { canGenerateFromListener } from "./generate-gate";
import { defaultPayloadFormat, PAYLOAD_FORMATS, PAYLOAD_FORMAT_LABEL, type PayloadFormat } from "./generate-format";
import { ListenerCallbackStrip } from "./ListenerCallbackStrip";
import dynamic from "next/dynamic";
import { usePayloadGenerator } from "../hooks/usePayloadGenerator";
import type { PayloadKey } from "@/types/generate";
import { AppWindow, Cpu, Info, PackageOpen, X } from "lucide-react";
import { cn } from "@/lib/utils";

const ConnectionPanel = dynamic(() => import("./ConnectionPanel"), { ssr: false });
const OneLinerPanel = dynamic(() => import("./OneLinerPanel"), { ssr: false });
const QuickPresets = dynamic(() => import("./QuickPresets"), { ssr: false });
const BuildHistorySection = dynamic(() => import("./BuildHistorySection"), { ssr: false });

import { useWS } from "@/lib/wsContext";

const BANNER_DISMISS_KEY = "forgec2_gen_banner_dismissed";
const FORMAT_KEY = "forgec2_gen_format";

function SectionHeading({ icon, tint, title, desc, className }: { icon: ReactNode; tint: string; title: string; desc: string; className?: string }) {
  return (
    <div className={cn("flex items-center gap-x-3 mb-5", className)}>
      <div className={cn("w-10 h-10 rounded-lg ring-1 ring-border/50 flex items-center justify-center", tint)}>{icon}</div>
      <div>
        <div className="text-sm font-semibold text-foreground">{title}</div>
        <div className="text-xs text-muted-foreground">{desc}</div>
      </div>
    </div>
  );
}

export default function GeneratePayloadWorkspace() {
  const { t } = useI18n();
  const g = usePayloadGenerator();
  const [showBanner, setShowBanner] = useState(true);
  const [historyRefresh, setHistoryRefresh] = useState(0);
  const [format, setFormat] = useState<PayloadFormat>("exe");
  const [showAllFormats, setShowAllFormats] = useState(false);
  const [showDelivery, setShowDelivery] = useState(false);
  const prevBusyRef = useRef<Record<string, boolean>>({});

  useEffect(() => {
    try {
      const dismissed = localStorage.getItem(BANNER_DISMISS_KEY);
      if (dismissed === "true") setShowBanner(false);
      setFormat(defaultPayloadFormat(localStorage.getItem(FORMAT_KEY)));
    } catch { /* ignore */ }
  }, []);

  const pickFormat = (next: PayloadFormat) => {
    setFormat(next);
    try { localStorage.setItem(FORMAT_KEY, next); } catch { /* ignore */ }
  };

  const states = g.states;
  const setForms = g.setForms;

  // Builds finished elsewhere (other tab/operator/previously queued job)
  // refresh the recent-builds history section via the WS bus.
  const { subscribe } = useWS();
  useEffect(() => {
    const unsub = subscribe((msg) => {
      if (msg.type === "build_update" && (msg.status === "completed" || msg.status === "failed")) {
        setHistoryRefresh((n) => n + 1);
      }
    });
    return unsub;
  }, [subscribe]);

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

  const canGenerate = canGenerateFromListener(g.shared.listener_id);

  const formatPanel = (key: PayloadFormat) => {
    switch (key) {
      case "exe":
        return <BinaryPanel variant="exe" form={g.forms.exe} setForm={makeDispatch("exe")} busy={g.states.exe.busy} result={g.states.exe.result} onGenerate={g.handlerMap.exe} canGenerate={canGenerate} />;
      case "dll":
        return <BinaryPanel variant="dll" form={g.forms.dll} setForm={makeDispatch("dll")} busy={g.states.dll.busy} result={g.states.dll.result} onGenerate={g.handlerMap.dll} canGenerate={canGenerate} />;
      case "ps1":
        return <PS1Panel form={g.forms.ps1} setForm={makeDispatch("ps1")} busy={g.states.ps1.busy} result={g.states.ps1.result} code={g.extras.ps1?.code} originalLen={g.extras.ps1?.original_length} obfuscatedLen={g.extras.ps1?.obfuscated_len} onGenerate={g.handlerMap.ps1} canGenerate={canGenerate} />;
      case "linux":
        return <UnixPanel variant="linux" form={g.forms.linux} setForm={makeDispatch("linux")} busy={g.states.linux.busy} result={g.states.linux.result} onGenerate={g.handlerMap.linux} canGenerate={canGenerate} />;
      case "macos":
        return <UnixPanel variant="macos" form={g.forms.macos} setForm={makeDispatch("macos")} busy={g.states.macos.busy} result={g.states.macos.result} onGenerate={g.handlerMap.macos} canGenerate={canGenerate} />;
      case "oneliner":
        return (
          <OneLinerPanel
            form={g.forms.oneliner} setForm={makeDispatch("oneliner")}
            busy={g.states.oneliner.busy} result={g.states.oneliner.result}
            onelinerData={g.extras.oneliner?.data}
            listeners={g.listeners} getListenerInfo={g.getListenerInfo}
            onGenerate={g.handlerMap.oneliner}
            canGenerate={canGenerateFromListener(g.forms.oneliner.listener_id || g.shared.listener_id)}
          />
        );
    }
  };

  const currentListener = g.listeners.find((l) => String(l.id) === String(g.shared.listener_id));

  return (
    <div>
      <ListenerCallbackStrip
        listener={currentListener}
        callback={g.shared.c2_url}
        onCreate={g.handleCreateListener}
      />
      {showBanner && (
        <div className="mt-3 flex items-center gap-2 px-4 py-2 bg-warning/10 border border-warning/20 rounded-lg">
          <Info className="w-4 h-4" />
          <span className="flex-1 text-xs text-warning-foreground">{t("generate.banner_text")} <a href="https://go.dev/dl/" target="_blank" className="underline hover:text-warning-foreground transition-colors">{t("generate.banner_download")}</a></span>
          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={dismissBanner} className="text-warning hover:text-warning" aria-label={t("generate.dismiss")} />}>
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
            onProfileDeleted={g.deleteProfile}
            fileInputRef={g.fileInputRef}
            onProfileImport={g.handleProfileImport}
          />
        </div>

        {/* ── Main workspace ── */}
        <div className="min-w-0 space-y-8">
          <section className="animate-fade-slide-up">
            <SectionHeading
              icon={<AppWindow className="w-4 h-4" />}
              tint="bg-primary/10 text-primary"
              title={t("generate.agents_title")}
              desc={t("generate.build_one_desc")}
            />
            <div className="mb-4 flex flex-wrap gap-1.5">
              {PAYLOAD_FORMATS.map((key) => (
                <Button
                  key={key}
                  type="button"
                  size="xs"
                  variant={format === key ? "default" : "outline"}
                  onClick={() => pickFormat(key)}
                >
                  {t(PAYLOAD_FORMAT_LABEL[key])}
                </Button>
              ))}
            </div>
            {formatPanel(format)}
            <div className="mt-3">
              <Button type="button" variant="ghost" size="xs" onClick={() => setShowAllFormats((v) => !v)}>
                {showAllFormats ? t("generate.hide_all_formats") : t("generate.show_all_formats")}
              </Button>
            </div>
            {showAllFormats && (
              <div className="mt-3 grid grid-cols-1 gap-5 md:grid-cols-2">
                {PAYLOAD_FORMATS.filter((key) => key !== format).map((key) => (
                  <div key={key}>{formatPanel(key)}</div>
                ))}
              </div>
            )}
          </section>

          <section>
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowDelivery((v) => !v)}>
              <PackageOpen className="size-4" />
              {showDelivery ? t("generate.hide_delivery") : t("generate.show_delivery")}
            </Button>
            {showDelivery && (
              <div className="mt-4 space-y-6">
                <div>
                  <SectionHeading
                    icon={<PackageOpen className="w-4 h-4" />}
                    tint="bg-chart-6/violet text-chart-6"
                    title={t("generate.artifact_kit")}
                    desc={t("generate.artifact_kit_desc")}
                  />
                  <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
                    <StagerPanel variant="windows" form={g.forms.stager} setForm={makeDispatch("stager")} busy={g.states.stager.busy} result={g.states.stager.result} onGenerate={g.handlerMap.stager} canGenerate={canGenerate} />
                    <StagerPanel variant="linux" form={g.forms.stager_linux} setForm={makeDispatch("stager_linux")} busy={g.states.stager_linux.busy} result={g.states.stager_linux.result} onGenerate={g.handlerMap.stager_linux} canGenerate={canGenerate} />
                  </div>
                </div>
                <div>
                  <SectionHeading
                    icon={<Cpu className="w-4 h-4" />}
                    tint="bg-warning/10 text-warning"
                    title={t("generate.shellcode_donut")}
                    desc={t("generate.shellcode_donut_desc")}
                  />
                  <div className="grid grid-cols-1 gap-5 lg:grid-cols-2">
                    <ShellcodePanel form={g.forms.shellcode} setForm={makeDispatch("shellcode")} busy={g.states.shellcode.busy} result={g.states.shellcode.result} onGenerate={g.handlerMap.shellcode} canGenerate={canGenerate} />
                    <DonutPanel form={g.forms.donut} setForm={makeDispatch("donut")} busy={g.states.donut.busy} result={g.states.donut.result} onGenerate={g.handlerMap.donut} fileRef={g.donutFileRef} canGenerate={canGenerate} />
                  </div>
                </div>
              </div>
            )}
          </section>

          <QuickPresets onApply={g.applyPreset} />
          <BuildHistorySection refreshKey={historyRefresh} />

          <div className="pt-4 text-center text-xs text-muted-foreground border-t border-border/60">
            {t("generate.footer_text")}
            <span className="block mt-1 text-warning">{t("generate.footer_warning")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
