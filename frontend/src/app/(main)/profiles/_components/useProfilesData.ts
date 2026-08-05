"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import {
  emptyActiveConfig,
  emptyMalleableForm,
  emptyProfile,
  type ActiveMalleableConfig,
  type AgentProfile,
  type MalleableForm,
} from "./types";

export function useProfilesData() {
  const { t } = useI18n();
  const [malleableForm, setMalleableForm] = useState<MalleableForm>(emptyMalleableForm);
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [selectedIdx, setSelectedIdx] = useState(-1);
  const selectedIdxRef = useRef(selectedIdx);
  selectedIdxRef.current = selectedIdx;
  const [editing, setEditing] = useState<AgentProfile>(emptyProfile);
  const [loadingProfiles, setLoadingProfiles] = useState(true);
  const [activeConfig, setActiveConfig] = useState<ActiveMalleableConfig>(emptyActiveConfig);
  const [loadingActiveConfig, setLoadingActiveConfig] = useState(true);
  const [profilesError, setProfilesError] = useState<string | null>(null);

  const loadActiveConfig = useCallback(async () => {
    setLoadingActiveConfig(true);
    try {
      const data = await api.get<ActiveMalleableConfig>(paths.integrations.malleable);
      setActiveConfig({
        malleable_enabled: (data.malleable_enabled ?? false) as boolean,
        malleable_profile: (data.malleable_profile ?? "") as string,
        status_code: (data.status_code ?? 200) as number,
        content_type: (data.content_type ?? "application/json") as string,
        headers: (data.headers ?? {}) as Record<string, string>,
        user_agent: (data.user_agent ?? "") as string,
        jitter: (data.jitter ?? 0) as number,
        interval: (data.interval ?? 0) as number,
        prepend: (data.prepend ?? "") as string,
        append: (data.append ?? "") as string,
      });
    } catch {
      /* active config optional */
    } finally {
      setLoadingActiveConfig(false);
    }
  }, []);

  const loadMalleableSettings = useCallback(async () => {
    try {
      const d = await api.get(paths.settings.root);
      setMalleableForm({
        enabled: (d.malleable_enabled ?? false) as boolean,
        status_code: (d.malleable_status ?? 200) as number,
        content_type: (d.malleable_ct ?? "application/json") as string,
        headers_text: "",
        prepend: (d.malleable_prepend ?? "") as string,
        append: (d.malleable_append ?? "") as string,
      });
    } catch {
      toast.error(t("profiles.toast.load_failed"));
    }
  }, [t]);

  const loadProfiles = useCallback(async () => {
    setLoadingProfiles(true);
    setProfilesError(null);
    try {
      const d = await api.get(paths.generate.profiles);
      const list = (d.profiles || d.Profiles || []) as AgentProfile[];
      setProfiles(list);
      if (list.length > 0 && selectedIdxRef.current < 0) {
        setSelectedIdx(0);
        setEditing({ ...list[0] });
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : t("profiles.toast.load_profiles_failed");
      setProfilesError(msg);
      toast.error(msg);
    } finally {
      setLoadingProfiles(false);
    }
  }, [t]);

  useEffect(() => {
    loadActiveConfig();
    loadMalleableSettings();
    loadProfiles();
  }, [loadActiveConfig, loadMalleableSettings, loadProfiles]);

  return {
    malleableForm,
    setMalleableForm,
    profiles,
    setProfiles,
    selectedIdx,
    setSelectedIdx,
    selectedIdxRef,
    editing,
    setEditing,
    loadingProfiles,
    activeConfig,
    setActiveConfig,
    loadingActiveConfig,
    profilesError,
    loadActiveConfig,
    loadMalleableSettings,
    loadProfiles,
  };
}
