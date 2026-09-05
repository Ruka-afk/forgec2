"use client";

import { useParams } from "next/navigation";
import { Banner } from "@/components/ui/banner";
import { PageContainer } from "@/components/ui/page-container";
import { TriangleAlert } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useRemoteDesktop } from "./_components/useRemoteDesktop";
import RdpToolbar from "./_components/RdpToolbar";
import RdpStage from "./_components/RdpStage";
import RdpStatusCards from "./_components/RdpStatusCards";

export default function RemoteDesktopPage() {
  const { t } = useI18n();
  const params = useParams();
  const id = params.id as string;

  const rdp = useRemoteDesktop(id);

  return (
    <PageContainer className="h-full gap-3 px-4 py-3 sm:px-6">
      <Banner tone="warning" icon={<TriangleAlert className="size-4" />} className="items-start">
        <div className="font-semibold">{t("agents.rdp_experimental_title")}</div>
        <div className="text-xs text-muted-foreground">{t("agents.rdp_experimental_desc")}</div>
      </Banner>
      <div className="flex min-h-0 flex-1 flex-col">
        <RdpToolbar
          t={t}
          agentId={id}
          nativeWidth={rdp.nativeWidth}
          nativeHeight={rdp.nativeHeight}
          resolutions={rdp.RESOLUTIONS}
          resolution={rdp.resolution}
          setResolution={rdp.setResolution}
          monitoring={rdp.monitoring}
          isFullscreen={rdp.isFullscreen}
          onToggleFullscreen={() => void rdp.toggleFullscreen()}
          versionBlocked={rdp.versionBlocked}
          onStart={() => void rdp.startMonitoring()}
          onStop={() => void rdp.stopMonitoring()}
        />

        <div className="flex-1 min-h-0 flex flex-col">
          <RdpStage
            t={t}
            status={rdp.status}
            monitoring={rdp.monitoring}
            lastUpdate={rdp.lastUpdate}
            wsLive={rdp.wsLive}
            screenData={rdp.screenData}
            nativeWidth={rdp.nativeWidth}
            nativeHeight={rdp.nativeHeight}
            mouseX={rdp.mouseX}
            mouseY={rdp.mouseY}
            showCursor={rdp.showCursor}
            containerRef={rdp.containerRef}
            imgRef={rdp.imgRef}
            onImageLoad={(w, h) => { rdp.setNativeWidth(w); rdp.setNativeHeight(h); }}
            onClick={(e) => void rdp.handleClick(e)}
            onMouseMove={rdp.handleMouseMove}
            onMouseLeave={rdp.hideCursor}
          />

          <RdpStatusCards t={t} monitoring={rdp.monitoring} />
        </div>
      </div>
    </PageContainer>
  );
}
