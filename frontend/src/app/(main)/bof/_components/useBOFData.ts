"use client";

import { useEffect, useState, useCallback } from "react";
import { api } from "@/lib/api";
import type { BOFFile, BOFLibraryItem, Execution, RepoItem } from "./types";

export function useBOFData() {
  const [files, setFiles] = useState<BOFFile[]>([]);
  const [repoItems, setRepoItems] = useState<RepoItem[]>([]);
  const [libraryItems, setLibraryItems] = useState<BOFLibraryItem[]>([]);
  const [executions, setExecutions] = useState<Execution[]>([]);
  const [agents, setAgents] = useState<Array<{ id: string; hostname: string }>>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<"bof" | "exec" | "quick" | "repo" | "library">("bof");


  const loadFiles = useCallback(async () => {
    try {
      const data = await api.json<{ BOFFiles?: BOFFile[]; bofs?: BOFFile[]; files?: BOFFile[] }>("/api/bof/list");
      setFiles(data.bofs || data.files || []);
      const execData = await api.get<{ results?: Execution[]; Results?: Execution[] }>("/api/bof/results");
      setExecutions(execData.results || []);
      const agentData = await api.get<{ Agents?: Array<Record<string, unknown>>; agents?: Array<Record<string, unknown>> }>("/agents");
      setAgents(
        (agentData.agents || []).map((a: Record<string, unknown>) => ({
          id: String(a.id || ""),
          hostname: String(a.hostname || ""),
        }))
      );
    } catch (e) {
      if (process.env.NODE_ENV === "development") console.error("BOF: load data failed", e);
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
      const d = await api.json<{ bofs?: BOFLibraryItem[] }>("/api/bof/list");
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
      const formData = new FormData();
      formData.append("file", file);
      formData.append("name", name);
      formData.append("description", desc);
      formData.append("architecture", arch);
      try {
        await api.postFormData("/api/bof/upload?format=json", formData);
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOF: upload failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const deleteBOF = useCallback(
    async (id: string) => {
      try {
        await api.del(`/api/bof/${id}`);
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOF: delete failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const runBOF = useCallback(
    async (id: string, agentId: string, args: string) => {
      try {
        await api.post(`/api/bof/${id}/run`, { agent_id: agentId, args });
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOF: run failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const editBOF = useCallback(
    async (id: string, name: string, description: string) => {
      try {
        await api.post(`/api/bof/${id}/edit`, { name, description });
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOF: edit failed", e);
      }
      loadFiles();
    },
    [loadFiles]
  );

  const importFromUrl = useCallback(
    async (url: string, name?: string) => {
      try {
        const data = await api.postJson("/api/bof/repos/import", { url, name: name || undefined });
        loadRepo();
        loadFiles();
        return { success: true, message: (data.message as string) || "Import completed successfully" };
      } catch {
        return { success: false, message: "Import failed - check URL and try again" };
      }
    },
    [loadRepo, loadFiles]
  );

  const importFromRepo = useCallback(
    async (item: RepoItem) => {
      try {
        await api.postJson("/api/bof/repos/import", {
          url: item.url,
          name: item.name,
        });
        loadRepo();
        loadFiles();
        return { success: true, message: `${item.name} imported successfully` };
      } catch {
        return { success: false, message: "Import failed" };
      }
    },
    [loadRepo, loadFiles]
  );

  const rateRepoItem = useCallback(
    async (itemId: string, rating: number) => {
      try {
        await api.postJson(`/api/bof/repos/${itemId}/rate`, { rating });
        loadRepo();
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOF Repo: rating failed", e);
      }
    },
    [loadRepo]
  );

  const uploadLibrary = useCallback(
    async (file: File, arch: string, name: string, desc: string, author: string) => {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("name", name);
      formData.append("description", desc);
      formData.append("arch", arch);
      formData.append("author", author);
      try {
        await api.postFormData("/api/bof/upload?format=json", formData);
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOFLibrary: upload failed", e);
      }
      loadLibrary();
    },
    [loadLibrary]
  );

  const runLibrary = useCallback(
    async (id: number | string, agentId: string, args: string) => {
      try {
        await api.post(`/api/bof/${id}/run`, { agent_id: agentId, args });
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOFLibrary: execute failed", e);
      }
    },
    []
  );

  const deleteLibrary = useCallback(
    async (id: number | string) => {
      try {
        await api.del(`/api/bof/${id}`);
      } catch (e) {
        if (process.env.NODE_ENV === "development") console.error("BOFLibrary: delete failed", e);
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

