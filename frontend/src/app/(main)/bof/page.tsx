"use client";

import { useState } from "react";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import { ConfirmModal, PageHeader, Spinner } from "@/components/UI";
import { useBOFData } from "./_components/useBOFData";
import { quickBOFLibrary } from "./_components/types";
import type { QuickBOF } from "./_components/types";
import dynamic from "next/dynamic";
import { Card } from "@/components/ui/card";
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

  const [cfm, setCfm] = useState<{ msg: string; cb: () => void } | null>(null);

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
      <div className="text-muted-foreground py-8 text-center">
        <Spinner />
      </div>
    );

  return (
    <div className="max-w-(--content-width) mx-auto pb-12 md:pb-0 animate-fade-slide-up">
      <PageHeader title={t("bof.title")} subtitle={t("bof.subtitle")} />

      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-5 mb-6">
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-primary/10 dark:bg-primary/15 rounded-xl flex items-center justify-center">
            <Box className="w-4 h-4" />
          </div>
          <div>
            <div className="text-2xl font-bold tabular-nums text-foreground">{files.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_uploaded")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-emerald-100 dark:bg-emerald-900/20 rounded-xl flex items-center justify-center">
            <Check className="w-4 h-4" />
          </div>
          <div>
            <div className="text-2xl font-bold tabular-nums text-foreground">{executions.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_executions")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-amber-100 dark:bg-amber-900/20 rounded-xl flex items-center justify-center">
            <PieChart className="w-4 h-4" />
          </div>
          <div>
            <div className="text-2xl font-bold tabular-nums text-foreground">
              {executions.length > 0 ? `${Math.round((executions.filter((e) => (e.status) === "success").length / executions.length) * 100)}%` : "N/A"}
            </div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_success_rate")}</div>
          </div>
        </Card>
        <Card className="p-4 sm:p-5 flex items-center gap-3">
          <div className="w-10 h-10 bg-primary/10 rounded-xl flex items-center justify-center">
            <BookOpen className="w-4 h-4" />
          </div>
          <div>
            <div className="text-2xl font-bold tabular-nums text-foreground">{agents.length}</div>
            <div className="text-xs text-muted-foreground">{t("bof.stat_available_agents")}</div>
          </div>
        </Card>
      </div>

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as typeof activeTab)}>
        <TabsList className="mb-6">
          {[
            { key: "bof", Icon: Box, label: t("bof.tab_library") },
            { key: "exec", Icon: Terminal, label: t("bof.tab_exec") },
            { key: "quick", Icon: Zap, label: t("bof.tab_quick") },
            { key: "repo", Icon: BookOpen, label: t("bof.tab_repo") },
            { key: "library", Icon: Layers, label: t("bof.tab_lib") },
          ].map((tab) => (
            <TabsTrigger key={tab.key} value={tab.key} className="gap-1.5">
              <tab.Icon className="w-3 h-3" />
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
              <Card key={bof.name} className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
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
                    <Zap className="w-4 h-4" />{t("bof.quick_run")}
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
          onDeleteLibrary={(id) => {
            setCfm({ msg: t("bof.delete_library"), cb: () => deleteLibrary(id) });
          }}
        />
      </TabsContent>
      </Tabs>

      <ConfirmModal open={!!cfm} title={t("common.confirm")} message={cfm?.msg || ""} confirmText={t("common.delete")} cancelText={t("common.cancel")} danger onConfirm={() => { cfm?.cb(); setCfm(null); }} onCancel={() => setCfm(null)} />
    </div>
  );
}

