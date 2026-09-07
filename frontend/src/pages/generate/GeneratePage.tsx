import { lazy, Suspense } from "react";

import { useI18n } from "@/lib/i18n";
import { useUrlState } from "@/lib/hooks/useUrlState";
import { PageContainer } from "@/components/ui/page-container";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

import { GENERATE_TABS, type GenerateTab } from "./components/generate-tabs";

const GeneratePayloadWorkspace = lazy(() => import("./components/GeneratePayloadWorkspace"));
const PackerPageContent = lazy(() => import("../packer/PackerPageContent"));
const StagerPageContent = lazy(() => import("../stager/StagerPageContent"));
const BuildsPageContent = lazy(() => import("../builds/BuildsPageContent"));
const ProfilesPageContent = lazy(() => import("../profiles/ProfilesPageContent"));

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
          <Suspense fallback={null}><GeneratePayloadWorkspace /></Suspense>
        </TabsContent>
        <TabsContent value="profiles" className="mt-0">
          <Suspense fallback={null}><ProfilesPageContent embedded /></Suspense>
        </TabsContent>
        <TabsContent value="stager" className="mt-0">
          <Suspense fallback={null}><StagerPageContent embedded /></Suspense>
        </TabsContent>
        <TabsContent value="packer" className="mt-0">
          <Suspense fallback={null}><PackerPageContent embedded /></Suspense>
        </TabsContent>
        <TabsContent value="builds" className="mt-0">
          <Suspense fallback={null}><BuildsPageContent embedded /></Suspense>
        </TabsContent>
      </Tabs>
    </PageContainer>
  );
}
