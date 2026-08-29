"use client";

import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { useConfirm } from "@/lib/hooks/useConfirm";
import { PageContainer } from "@/components/ui/page-container";
import { Spinner } from "@/components/ui/spinner";
import { useBOFData } from "./_components/useBOFData";
import { quickBOFLibrary } from "./_components/types";
import type { QuickBOF } from "./_components/types";
import dynamic from "next/dynamic";
import { Card } from "@/components/ui/card";
import { StatCard } from "@/components/ui/animated-stat-card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { BookOpen, Box, Check, Layers, PieChart, Terminal, Zap } from "lucide-react";

const BOFListTab = dynamic(() => import("./_components/BOFListTab"), { ssr: false });
const BOFRepoTab = dynamic(() => import("./_components/BOFRepoTab"), { ssr: false });
const BOFLibraryTab = dynamic(() => import("./_components/BOFLibraryTab"), { ssr: false });
const BOFExecutionsTab = dynamic(() => import("./_components/BOFExecutionsTab"), { ssr: false });

export default function BOFPage() {
  const { t } = useI18n();
  const {
    files,
    repoItems,
    libraryItems,
    executions,
    agents,
    loading,
    activeTab,
    setActiveTab,
    uploadBOF,
    deleteBOF,
    runBOF,
    editBOF,
    importFromUrl,
    importFromRepo,
    rateRepoItem,
    uploadLibrary,
    runLibrary,
    deleteLibrary,
  } = useBOFData();

  const { confirm, modal } = useConfirm();

  const handleQuickRun = (bof: QuickBOF) => {
    const bofFile = files.find((f) => (f.name || "").toLowerCase() === bof.name.toLowerCase());
    if (bofFile) {
      runBOF(String(bofFile.id || ""), agents[0]?.id || "", bof.args);
    } else {
      toast.error(t("bof.toast.not_uploaded", { name: bof.name }));
    }
  };

  if (loading)
    return (
      <PageContainer title={t("bof.title")} subtitle={t("bof.subtitle")}>
        <div className="flex items-center justify-center py-16">
          <Spinner />
        </div>
      </PageContainer>
    );

  return (
    <PageContainer title={t("bof.title")} subtitle={t("bof.subtitle")}>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 sm:gap-5 mb-6">
        <StatCard label={t("bof.stat_uploaded")} value={files.length} color="primary" icon={<Box className="size-4" />} iconSide="left" />
        <StatCard label={t("bof.stat_executions")} value={executions.length} color="success" icon={<Check className="size-4" />} iconSide="left" />
        <StatCard
          label={t("bof.stat_success_rate")}
          value={executions.length > 0 ? `${Math.round((executions.filter((e) => (e.status) === "success").length / executions.length) * 100)}%` : "N/A"}
          color="warning"
          icon={<PieChart className="size-4" />}
          iconSide="left"
        />
        <StatCard label={t("bof.stat_available_agents")} value={agents.length} color="primary" icon={<BookOpen className="size-4" />} iconSide="left" />
      </div>

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as typeof activeTab)}>
        <TabsList>
          {[
            { key: "bof", Icon: Box, label: t("bof.tab_library") },
            { key: "exec", Icon: Terminal, label: t("bof.tab_exec") },
            { key: "quick", Icon: Zap, label: t("bof.tab_quick") },
            { key: "repo", Icon: BookOpen, label: t("bof.tab_repo") },
            { key: "library", Icon: Layers, label: t("bof.tab_lib") },
          ].map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-1.5">
              <tab.Icon className="size-3" />
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>

      <TabsContent value="bof">
        <BOFListTab
          files={files}
          loading={loading}
          onUpload={(file, arch, name, desc) => uploadBOF(file, arch, name, desc)}
          onDelete={(id) => deleteBOF(String(id))}
          onRun={(id, agentId, args) => runBOF(String(id), agentId, args)}
          onEdit={editBOF}
          agents={agents}
        />
      </TabsContent>

      <TabsContent value="exec"><BOFExecutionsTab executions={executions} loading={loading} /></TabsContent>

      <TabsContent value="quick">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {quickBOFLibrary.map((bof) => {
            const isUploaded = files.some((f) => (f.name || "").toLowerCase() === bof.name.toLowerCase());
            return (
              <Card key={bof.name} className="p-(--card-spacing) hover:shadow-lg dark:hover:shadow-xl transition-shadow">
                <div className="flex items-center justify-between mb-2">
                  <div className="text-sm font-medium text-foreground font-mono">{bof.name}</div>
                  <Badge variant={isUploaded ? "default" : "secondary"} className="text-(--fs-micro-sm)">
                    {isUploaded ? t("bof.uploaded_ready") : t("bof.not_installed")}
                  </Badge>
                </div>
                <div className="text-xs text-muted-foreground mb-1">{bof.desc}</div>
                <div className="flex items-center justify-between mt-3">
                  <Badge variant="secondary" className="text-(--fs-micro-sm) font-mono">{bof.arch}</Badge>
                  <Button size="sm" onClick={() => handleQuickRun(bof)} disabled={!isUploaded}>
                    <Zap className="size-4" />{t("bof.quick_run")}
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      </TabsContent>

      <TabsContent value="repo"><BOFRepoTab repoItems={repoItems} loading={loading} onImport={importFromRepo} onImportUrl={importFromUrl} onRate={rateRepoItem} /></TabsContent>

      <TabsContent value="library">
        <BOFLibraryTab
          libraryItems={libraryItems}
          loading={loading}
          agents={agents}
          onUploadLibrary={(file, arch, name, desc, author) => uploadLibrary(file, arch, name, desc, author)}
          onRunLibrary={(id, agentId, args) => runLibrary(id, agentId, args)}
          onDeleteLibrary={async (id) => {
            if (await confirm({ message: t("bof.delete_library") })) deleteLibrary(id);
          }}
        />
      </TabsContent>
      </Tabs>

      {modal}
    </PageContainer>
  );
}

