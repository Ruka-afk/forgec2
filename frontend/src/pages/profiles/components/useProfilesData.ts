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
  const [activeConfigError, setActiveConfigError] = useState<string | null>(null);
  const [malleableLoaded, setMalleableLoaded] = useState(false);
  const [malleableError, setMalleableError] = useState<string | null>(null);
  const [profilesError, setProfilesError] = useState<string | null>(null);

  const loadActiveConfig = useCallback(async () => {
    setLoadingActiveConfig(true);
    setActiveConfigError(null);
    try {
      // Read /settings, not /integrations/malleable: the latter returns bare
      // keys (enabled/status_code/...) with no `malleable_enabled`, so the
      // card's Enabled toggle read undefined and always showed "disabled".
      // /settings carries every field the card renders under the same
      // malleable_* names the editable form below already uses.
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

  const loadMalleableSettings = useCallback(async () => {
    setMalleableError(null);
    try {
      const d = await api.get<Record<string, unknown>>(paths.settings.root);
      setMalleableForm({
        enabled: (d.malleable_enabled ?? false) as boolean,
        status_code: (d.malleable_status ?? 200) as number,
        content_type: (d.malleable_ct ?? "application/json") as string,
        headers_text: (d.malleable_headers ?? "") as string,
        prepend: (d.malleable_prepend ?? "") as string,
        append: (d.malleable_append ?? "") as string,
      });
      setMalleableLoaded(true);
    } catch (e) {
      const msg = e instanceof Error ? e.message : t("profiles.toast.load_failed");
      setMalleableError(msg);
      toast.error(msg);
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
    activeConfigError,
    malleableLoaded,
    malleableError,
    profilesError,
    loadActiveConfig,
    loadMalleableSettings,
    loadProfiles,
  };
}
