"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { fetchAgentListCached } from "@/lib/agents";
import { useI18n } from "@/lib/i18n";
import { logger } from "@/lib/logger";
import { toast } from "sonner";
import type { BOFFile, BOFLibraryItem, Execution, RepoItem } from "./types";

const MAX_BOF_SIZE = 10 * 1024 * 1024; // 10MB client-side sanity cap for .o files

const log = logger.withScope("bof");

export function useBOFData() {
  const { t } = useI18n();
  const [files, setFiles] = useState<BOFFile[]>([]);
  const [repoItems, setRepoItems] = useState<RepoItem[]>([]);
  const [libraryItems, setLibraryItems] = useState<BOFLibraryItem[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string }>>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"bof" | "exec" | "quick" | "repo" | "library">("bof");


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
    try {
      const data = await api.get<{ data?: RepoItem[]; repo?: RepoItem[]; items?: RepoItem[] }>("/bof_repo");
      setRepoItems(data.data || data.repo || data.items || []);
    } catch {
      setRepoItems([]);
    }
  }, []);

  const loadLibrary = useCallback(async () => {
    try {
      const d = await api.get<{ bofs?: BOFLibraryItem[] }>(paths.bof.list);
      setLibraryItems(d.bofs || []);
    } catch {
      setLibraryItems([]);
    }
  }, []);

  useEffect(() => {
    loadFiles();
  }, [loadFiles]);

  useEffect(() => {
    if (activeTab === "repo") loadRepo();
  }, [activeTab, loadRepo]);

  useEffect(() => {
    if (activeTab === "library") loadLibrary();
  }, [activeTab, loadLibrary]);

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
      }
      loadFiles();
    },
    [loadFiles]
  );

  const runBOF = useCallback(
    async (id: string, agentId: string, args: string) => {
      try {
        await api.post(paths.bof.run(id), { agent_id: agentId, args });
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("run failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const editBOF = useCallback(
    async (id: string, name: string, description: string) => {
      try {
        await api.post(paths.bof.edit(id), { name, description });
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("edit failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const importFromUrl = useCallback(
    async (url: string, name?: string) => {
      try {
        const data = await api.postJson(paths.bof.reposImport, { url, name: name || undefined });
        loadRepo();
        loadFiles();
        return { success: true, message: (data.message as string) || t("bof.toast.import_success") };
      } catch {
        return { success: false, message: t("bof.toast.import_failed_url") };
      }
    },
    [loadRepo, loadFiles, t]
  );

  const importFromRepo = useCallback(
    async (item: RepoItem) => {
      try {
        await api.postJson(paths.bof.reposImport, {
          url: item.url,
          name: item.name,
        });
        loadRepo();
        loadFiles();
        return { success: true, message: t("bof.toast.imported_name", { name: item.name || t("bof.unnamed") }) };
      } catch {
        return { success: false, message: t("bof.toast.import_failed") };
      }
    },
    [loadRepo, loadFiles, t]
  );

  const rateRepoItem = useCallback(
    async (itemId: string, rating: number) => {
      try {
        await api.postJson(paths.bof.reposRate(itemId), { rating });
        loadRepo();
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("repo rating failed", e);
      }
    },
    [loadRepo]
  );

  const uploadLibrary = useCallback(
    async (file: File, arch: string, name: string, desc: string, author: string) => {
      if (file.size > MAX_BOF_SIZE) {
        toast.error(t("bof.toast.file_too_large", { max: "10MB" }));
        return;
      }
      const formData = new FormData();
      formData.append("file", file);
      formData.append("name", name);
      formData.append("description", desc);
      formData.append("arch", arch);
      formData.append("author", author);
      try {
        await api.postFormData(paths.bof.upload, formData);
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("library upload failed", e);
        toast.error(t("bof.toast.upload_failed"));
      }
      loadLibrary();
    },
    [loadLibrary, t]
  );

  const runLibrary = useCallback(
    async (id: number | string, agentId: string, args: string) => {
      try {
        await api.post(paths.bof.run(id), { agent_id: agentId, args });
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("library execute failed", e);
      }
    },
    []
  );

  const deleteLibrary = useCallback(
    async (id: number | string) => {
      try {
        await api.del(paths.bof.one(id));
      } catch (e) {
        if (process.env.NODE_ENV === "development") log.error("library delete failed", e);
      }
      loadLibrary();
    },
    [loadLibrary]
  );

  return {
    files,
    repoItems,
    libraryItems,
    executions,
    agents,
    loading,
    activeTab,
    setActiveTab,
    loadFiles,
    loadRepo,
    loadLibrary,
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
  };
}

