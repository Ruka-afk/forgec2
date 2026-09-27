import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import {
  emptyActiveConfig,
  emptyProfile,
  type ActiveMalleableConfig,
  type AgentProfile,
} from "./types";

export function useProfilesData() {
  const { t } = useI18n();
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const [selectedIdx, setSelectedIdx] = useState(-1);
  const selectedIdxRef = useRef(selectedIdx);
  selectedIdxRef.current = selectedIdx;
  const [editing, setEditing] = useState<AgentProfile>(emptyProfile);
  const [loadingProfiles, setLoadingProfiles] = useState(true);
  const [activeConfig, setActiveConfig] = useState<ActiveMalleableConfig>(emptyActiveConfig);
  const [loadingActiveConfig, setLoadingActiveConfig] = useState(true);
  const [activeConfigError, setActiveConfigError] = useState<string | null>(null);
  const [profilesError, setProfilesError] = useState<string | null>(null);

  // Single read of the server-wide Malleable config. It used to be fetched
  // twice — once here for the read-only card and once by loadMalleableSettings
  // to seed a local edit form. That form was a duplicate of the one in
  // Settings writing the same POST /settings/malleable, so it is gone; this
  // tab is now display-only and Settings owns every write.
  const loadActiveConfig = useCallback(async () => {
    setLoadingActiveConfig(true);
    setActiveConfigError(null);
    try {
      // Read /settings, not /integrations/malleable: the latter returns bare
      // keys (enabled/status_code/...) with no `malleable_enabled`, so the
      // card's Enabled toggle read undefined and always showed "disabled".
      const d = await api.get<Record<string, unknown>>(paths.settings.root);
      setActiveConfig({
        malleable_enabled: (d.malleable_enabled ?? false) as boolean,
        status_code: (d.malleable_status ?? 200) as number,
        content_type: (d.malleable_ct ?? "application/json") as string,
        headers: (d.malleable_headers ?? {}) as Record<string, string>,
        prepend: (d.malleable_prepend ?? "") as string,
        append: (d.malleable_append ?? "") as string,
      });
    } catch (e) {
      setActiveConfigError(e instanceof Error ? e.message : t("profiles.toast.load_failed"));
    } finally {
      setLoadingActiveConfig(false);
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
    loadProfiles();
  }, [loadActiveConfig, loadProfiles]);

  return {
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
    activeConfigError,
    profilesError,
    loadActiveConfig,
    loadProfiles,
  };
}
