"use client";

import { useCallback, useState, useEffect, useRef } from "react";
import { useSearchParams } from "react-router-dom";
import type { ReactNode } from "react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { PageSpinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { BinaryPanel, UnixPanel, StagerPanel, PS1Panel, ShellcodePanel, DonutPanel } from "./BuildPanels";
import { canGenerateFromListener, canGeneratePayload } from "./generate-gate";
import { defaultPayloadFormat, PAYLOAD_FORMATS, PAYLOAD_FORMAT_LABEL, type PayloadFormat } from "./generate-format";
import { parseGenerateQuery } from "./generate-query";
import { ListenerCallbackStrip } from "./ListenerCallbackStrip";
import dynamic from "next/dynamic";
import { usePayloadGenerator } from "../hooks/usePayloadGenerator";
import type { PayloadKey } from "@/types/generate";
import { AppWindow, Cpu, Info, PackageOpen, X } from "lucide-react";
import { cn } from "@/lib/utils";

const ConnectionPanel = dynamic(() => import("./ConnectionPanel"), { ssr: false });
const OneLinerPanel = dynamic(() => import("./OneLinerPanel"), { ssr: false });
const DeliveryPanel = dynamic(() => import("./DeliveryPanel"), { ssr: false });
const QuickPresets = dynamic(() => import("./QuickPresets"), { ssr: false });
const BuildHistorySection = dynamic(() => import("./BuildHistorySection"), { ssr: false });

import { useWS } from "@/lib/wsContext";

const BANNER_DISMISS_KEY = "forgec2_gen_banner_dismissed";
const FORMAT_KEY = "forgec2_gen_format";

function SectionHeading({ icon, tint, title, desc, className }: { icon: ReactNode; tint: string; title: string; desc: string; className?: string }) {
  return (
    <div className={cn("mb-3 flex items-center gap-x-2.5", className)}>
      <div className={cn("grid size-8 place-items-center rounded-lg shadow-sm ring-1 ring-border/30", tint)}>{icon}</div>
      <div className="min-w-0">
        <div className="truncate text-sm font-semibold tracking-tight text-foreground">{title}</div>
        <div className="truncate text-xs leading-4 text-muted-foreground">{desc}</div>
      </div>
    </div>
  );
}

const FORMAT_META: Record<PayloadFormat, { icon: ReactNode; desc: string; iconTint: string; activeRing: string }> = {
  exe: { icon: <AppWindow className="size-4" />, desc: "GUI implant · exe", iconTint: "bg-warning/10 text-warning", activeRing: "border-warning/30 bg-warning/5" },
  dll: { icon: <PackageOpen className="size-4" />, desc: "sideload · dll", iconTint: "bg-destructive/10 text-destructive", activeRing: "border-destructive/30 bg-destructive/5" },
  ps1: { icon: <Cpu className="size-4" />, desc: "ps1 · one-liner", iconTint: "bg-info/10 text-info", activeRing: "border-info/30 bg-info/5" },
  linux: { icon: <Cpu className="size-4" />, desc: "elf · amd64/arm", iconTint: "bg-success/10 text-success", activeRing: "border-success/30 bg-success/5" },
  macos: { icon: <AppWindow className="size-4" />, desc: "mach-o · mac", iconTint: "bg-chart-6/violet text-chart-6", activeRing: "border-chart-6/30 bg-chart-6/5" },
  oneliner: { icon: <PackageOpen className="size-4" />, desc: "curl · wget · ps", iconTint: "bg-muted text-muted-foreground", activeRing: "border-primary/30 bg-primary/5" },
};

export default function GeneratePayloadWorkspace() {
  const { t } = useI18n();
  const [searchParams] = useSearchParams();
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
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    try {
      const q = parseGenerateQuery(searchParams.toString());
      const next = q.format || defaultPayloadFormat(localStorage.getItem(FORMAT_KEY));
      setFormat(next);
      if (q.format) {
        try { localStorage.setItem(FORMAT_KEY, q.format); } catch { /* ignore */ }
      }
    } catch { /* ignore */ }
  }, [searchParams]);

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

  const currentListener = g.listeners.find((l) => String(l.id) === String(g.shared.listener_id));
  const canGenerate = canGeneratePayload({
    listenerId: g.shared.listener_id,
    listenerScheme: currentListener?.scheme || currentListener?.type,
    beaconTransport: g.shared.beacon_transport,
    failover: g.shared.failover,
  });

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

  return (
    <div className="mx-auto w-full max-w-[1440px]">
      <ListenerCallbackStrip
        listener={currentListener}
        callback={g.shared.c2_url}
        onCreate={g.handleCreateListener}
      />
      {showBanner && (
        <div className="mt-2 flex items-center gap-2.5 rounded-lg border border-warning/25 bg-gradient-to-r from-warning/15 via-warning/10 to-transparent px-3 py-2 shadow-sm">
          <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-warning/15 text-warning ring-1 ring-warning/20">
            <Info className="size-4" />
          </div>
          <span className="flex-1 text-xs leading-5 text-warning-foreground">{t("generate.banner_text")} <a href="https://go.dev/dl/" target="_blank" rel="noopener noreferrer" className="font-medium underline decoration-warning/40 underline-offset-2 transition-colors hover:text-warning">{t("generate.banner_download")}</a></span>
          <Tooltip>
            <TooltipTrigger render={<Button variant="ghost" size="icon-sm" onClick={dismissBanner} className="shrink-0 text-warning hover:bg-warning/10 hover:text-warning" aria-label={t("generate.dismiss")} />}>
            <X className="size-4" />
            </TooltipTrigger>
            <TooltipContent>{t("generate.dismiss")}</TooltipContent>
          </Tooltip>
        </div>
      )}

      <div className="mt-3 grid grid-cols-1 items-start gap-4 lg:grid-cols-[300px_minmax(0,1fr)]">
        {/* ── 左栏：连接信息 ── */}
        <div className="min-w-0 lg:sticky lg:top-4 lg:max-h-[calc(100vh-6rem)] lg:overflow-y-auto lg:pr-1 lg:pb-2 [scrollbar-width:thin]">
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

        {/* ── 右栏：生成载荷 ── */}
        <div className="min-w-0 space-y-4">
          <section className="animate-fade-slide-up">
            <SectionHeading
              icon={<AppWindow className="size-4" />}
              tint="bg-primary/10 text-primary"
              title={t("generate.agents_title")}
              desc={t("generate.build_one_desc")}
            />
            <div className="mb-3 grid grid-cols-3 gap-1.5 sm:grid-cols-6" role="tablist" aria-label={t("generate.agents_title")}>
              {PAYLOAD_FORMATS.map((key) => {
                const meta = FORMAT_META[key];
                const active = format === key;
                return (
                  <button
                    key={key}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    onClick={() => pickFormat(key)}
                    className={cn(
                      "group relative flex items-center gap-2 rounded-lg border px-2 py-2 text-left outline-none transition-all duration-150 hover:shadow-sm focus-visible:ring-2 focus-visible:ring-ring active:scale-[0.98]",
                      active
                        ? cn("shadow-sm ring-1", meta.activeRing)
                        : "border-border/60 bg-card hover:border-primary/20 hover:bg-muted/40"
                    )}
                  >
                    <div className={cn("grid size-7 shrink-0 place-items-center rounded-md ring-1 ring-border/30 transition-transform duration-200 group-hover:scale-105", meta.iconTint)}>
                      {meta.icon}
                    </div>
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-xs font-semibold leading-4 text-foreground">{t(PAYLOAD_FORMAT_LABEL[key])}</div>
                      <div className="truncate text-[11px] leading-3 text-muted-foreground">{meta.desc}</div>
                    </div>
                    {active && <div className="absolute right-1.5 top-1.5 size-1.5 rounded-full bg-primary shadow-sm" aria-hidden="true" />}
                  </button>
                );
              })}
            </div>
            <div className="animate-fade-slide-up" key={format}>{formatPanel(format)}</div>
            <div className="mt-2.5 flex justify-center">
              <Button type="button" variant="outline" size="xs" onClick={() => setShowAllFormats((v) => !v)} className="h-7 rounded-full px-3 text-xs shadow-sm">
                {showAllFormats ? t("generate.hide_all_formats") : t("generate.show_all_formats")}
              </Button>
            </div>
            {showAllFormats && (
              <div className="mt-3 grid grid-cols-1 gap-4 md:grid-cols-2">
                {PAYLOAD_FORMATS.filter((key) => key !== format).map((key) => (
                  <div key={key} className="animate-fade-slide-up">{formatPanel(key)}</div>
                ))}
              </div>
            )}
          </section>

          <section className="rounded-xl border border-border/60 bg-muted/20 p-3">
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowDelivery((v) => !v)} className="h-9 w-full justify-start rounded-lg px-2.5 py-1.5 hover:bg-card">
              <span className="grid size-8 place-items-center rounded-lg bg-chart-6/10 text-chart-6 ring-1 ring-border/40">
                <PackageOpen className="size-4" />
              </span>
              <span className="flex-1 text-left text-sm font-medium">{showDelivery ? t("generate.hide_delivery") : t("generate.show_delivery")}</span>
            </Button>
            {showDelivery && (
              <div className="mt-3 animate-fade-slide-up space-y-4">
                <div>
                  <SectionHeading
                    icon={<PackageOpen className="size-4" />}
                    tint="bg-chart-6/violet text-chart-6"
                    title={t("generate.artifact_kit")}
                    desc={t("generate.artifact_kit_desc")}
                  />
                  <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <StagerPanel variant="windows" form={g.forms.stager} setForm={makeDispatch("stager")} busy={g.states.stager.busy} result={g.states.stager.result} onGenerate={g.handlerMap.stager} canGenerate={canGenerate} />
                    <StagerPanel variant="linux" form={g.forms.stager_linux} setForm={makeDispatch("stager_linux")} busy={g.states.stager_linux.busy} result={g.states.stager_linux.result} onGenerate={g.handlerMap.stager_linux} canGenerate={canGenerate} />
                  </div>
                </div>
                <div>
                  <SectionHeading
                    icon={<Cpu className="size-4" />}
                    tint="bg-warning/10 text-warning"
                    title={t("generate.shellcode_donut")}
                    desc={t("generate.shellcode_donut_desc")}
                  />
                  <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                    <ShellcodePanel form={g.forms.shellcode} setForm={makeDispatch("shellcode")} busy={g.states.shellcode.busy} result={g.states.shellcode.result} onGenerate={g.handlerMap.shellcode} canGenerate={canGenerate} />
                    <DonutPanel form={g.forms.donut} setForm={makeDispatch("donut")} busy={g.states.donut.busy} result={g.states.donut.result} onGenerate={g.handlerMap.donut} fileRef={g.donutFileRef} canGenerate={canGenerate} />
                  </div>
                </div>
                <DeliveryPanel />
              </div>
            )}
          </section>

          <QuickPresets onApply={g.applyPreset} />
          <BuildHistorySection refreshKey={historyRefresh} />

          <div className="rounded-xl border border-border/60 bg-card px-4 py-3 text-center text-xs leading-5 text-muted-foreground">
            {t("generate.footer_text")}
            <span className="mt-1 block font-medium text-warning">{t("generate.footer_warning")}</span>
          </div>
        </div>
      </div>
    </div>
  );
}
