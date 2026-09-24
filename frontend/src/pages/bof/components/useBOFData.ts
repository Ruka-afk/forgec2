import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentListCached } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { logger } from "@/lib/logger";
import { toast } from "sonner";
import type { BOFFile, Execution, RepoItem } from "./types";
import { bofImportFilename } from "./import-target";

const MAX_BOF_SIZE = 10 * 1024 * 1024; // 10MB client-side sanity cap for .o files

const log = logger.withScope("bof");

export function useBOFData() {
  const { t } = useI18n();
  const [files, setFiles] = useState<BOFFile[]>([]);
  const [repoItems, setRepoItems] = useState<RepoItem[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string }>>([]);
  const [loading, setLoading] = useState(true);
  const [repoLoading, setRepoLoading] = useState(false);
  const [activeTab, setActiveTab] = useState<"bof" | "exec" | "quick" | "repo">("bof");

  const loadFiles = useCallback(async () => {
    try {
      const data = await api.get<{ BOFFiles?: BOFFile[]; bofs?: BOFFile[]; files?: BOFFile[] }>(paths.bof.list);
      setFiles(data.bofs || data.files || []);
      const execData = await api.get<{ results?: Execution[]; Results?: Execution[] }>(paths.bof.results);
      setExecutions(execData.results || []);
      const agentList = await fetchAgentListCached();
      setAgents(
        agentList.map((a) => ({
          id: String(a.id || ""),
          hostname: String(a.hostname || ""),
        }))
      );
    } catch (e) {
      if (process.env.NODE_ENV === "development") log.error("load data failed", e);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadRepo = useCallback(async () => {
    setRepoLoading(true);
    try {
      const data = await api.get<RepoItem[]>(paths.bof.repos);
      setRepoItems(Array.isArray(data) ? data : []);
    } catch {
      setRepoItems([]);
    } finally {
      setRepoLoading(false);
    }
  }, []);

  useEffect(() => {
    loadFiles();
  }, [loadFiles]);

  useEffect(() => {
    if (activeTab === "repo") loadRepo();
  }, [activeTab, loadRepo]);

  const uploadBOF = useCallback(
    async (file: File, arch: string, name: string, desc: string) => {
      if (file.size > MAX_BOF_SIZE) {
        toast.error(t("bof.toast.file_too_large", { max: "10MB" }));
        return;
      }
      const formData = new FormData();
      formData.append("file", file);
      formData.append("name", name);
      formData.append("description", desc);
      formData.append("architecture", arch);
      try {
        await api.postFormData(paths.bof.upload, formData);
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("upload failed", e);
        toast.error(t("bof.toast.upload_failed"));
      }
      loadFiles();
    },
    [loadFiles, t]
  );

  const deleteBOF = useCallback(
    async (id: string) => {
      try {
        await api.del(paths.bof.one(id));
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("delete failed", e);
        toast.error(t("bof.toast.delete_failed"));
      }
      loadFiles();
    },
    [loadFiles, t]
  );

  const runBOF = useCallback(
    async (id: string, agentId: string, args: string) => {
      try {
        await api.post(paths.bof.run(id), { agent_id: agentId, args });
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("run failed", e);
        toast.error(t("bof.toast.run_failed"));
      }
      loadFiles();
    },
    [loadFiles, t]
  );

  const editBOF = useCallback(
    async (id: string, name: string, description: string) => {
      try {
        await api.post(paths.bof.edit(id), { name, description });
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("edit failed", e);
        toast.error(t("bof.toast.edit_failed"));
      }
      loadFiles();
    },
    [loadFiles, t]
  );

  // The endpoint requires `url` AND `filename`; sending only `url` (as this page
  // used to) made every import fail with "url and filename are required".
  const importFromUrl = useCallback(
    async (url: string, name?: string) => {
      try {
        const data = await api.postJson<{ message?: string }>(paths.bof.reposImport, {
          url,
          filename: bofImportFilename(url, name),
        });
        loadFiles();
        return { success: true, message: data.message || t("bof.toast.import_success") };
      } catch {
        return { success: false, message: t("bof.toast.import_failed_url") };
      }
    },
    [loadFiles, t]
  );

  return {
    files,
    repoItems,
    executions,
    agents,
    loading,
    repoLoading,
    activeTab,
    setActiveTab,
    loadFiles,
    loadRepo,
    uploadBOF,
    deleteBOF,
    runBOF,
    editBOF,
    importFromUrl,
  };
}
