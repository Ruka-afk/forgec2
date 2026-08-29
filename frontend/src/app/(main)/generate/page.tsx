"use client";

import dynamic from "next/dynamic";
import { useI18n } from "@/lib/i18n";
import { useUrlState } from "@/lib/hooks/useUrlState";
import { PageContainer } from "@/components/ui/page-container";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { GENERATE_TABS, type GenerateTab } from "./_components/generate-tabs";

const GeneratePayloadWorkspace = dynamic(
  () => import("./_components/GeneratePayloadWorkspace"),
  { ssr: false },
);
const PackerPageContent = dynamic(
  () => import("../packer/PackerPageContent"),
  { ssr: false },
);
const StagerPageContent = dynamic(
  () => import("../stager/StagerPageContent"),
  { ssr: false },
);
const BuildsPageContent = dynamic(
  () => import("../builds/BuildsPageContent"),
  { ssr: false },
);
const ProfilesPageContent = dynamic(
  () => import("../profiles/ProfilesPageContent"),
  { ssr: false },
);

export default function GeneratePage() {
  const { t } = useI18n();
  const [tab, setTab] = useUrlState<GenerateTab>("tab", "payload", GENERATE_TABS);

  return (
    <PageContainer title={t("generate.title")} subtitle={t("generate.subtitle")}>
      <Tabs
        value={tab}
        onValueChange={(v) => {
          if (v && (GENERATE_TABS as readonly string[]).includes(v)) setTab(v as GenerateTab);
        }}
      >
        <TabsList>
          <TabsTrigger value="payload">{t("generate.tab_payload")}</TabsTrigger>
          <TabsTrigger value="profiles">{t("generate.tab_profiles")}</TabsTrigger>
          <TabsTrigger value="stager">{t("generate.tab_stager")}</TabsTrigger>
          <TabsTrigger value="packer">{t("generate.tab_packer")}</TabsTrigger>
          <TabsTrigger value="builds">{t("generate.tab_builds")}</TabsTrigger>
        </TabsList>
        <TabsContent value="payload" className="mt-0">
          <GeneratePayloadWorkspace />
        </TabsContent>
        <TabsContent value="profiles" className="mt-0">
          <ProfilesPageContent embedded />
        </TabsContent>
        <TabsContent value="stager" className="mt-0">
          <StagerPageContent embedded />
        </TabsContent>
        <TabsContent value="packer" className="mt-0">
          <PackerPageContent embedded />
        </TabsContent>
        <TabsContent value="builds" className="mt-0">
          <BuildsPageContent embedded />
        </TabsContent>
      </Tabs>
    </PageContainer>
  );
}
