import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { AgentProfile, ProfileValidationData } from "./types";

interface ImportResponse {
  success?: boolean;
  profile?: AgentProfile;
  warnings?: string[];
  diagnostics?: ProfileValidationData["diagnostics"];
  ignored?: string[];
}

export function useProfilesData() {
  const { t } = useI18n();
  const [profiles, setProfiles] = useState<AgentProfile[]>([]);
  const profilesRef = useRef<AgentProfile[]>([]);
  profilesRef.current = profiles;
  const [selectedIdx, setSelectedIdx] = useState(-1);
  const selectedIdxRef = useRef(-1);
  selectedIdxRef.current = selectedIdx;
  const [source, setSource] = useState("");
  const [savedSource, setSavedSource] = useState("");
  const [loadingProfiles, setLoadingProfiles] = useState(true);
  const [profilesError, setProfilesError] = useState<string | null>(null);
  const [validating, setValidating] = useState(false);
  const [validation, setValidation] = useState<ProfileValidationData | null>(null);
  const validationSeq = useRef(0);

  const selectProfile = useCallback((idx: number, list = profilesRef.current) => {
    const profile = list[idx];
    if (!profile) return;
    validationSeq.current += 1;
    setValidating(false);
    setSelectedIdx(idx);
    const nextSource = profile.source || "";
    setSource(nextSource);
    setSavedSource(nextSource);
    setValidation(null);
  }, []);

  const loadProfiles = useCallback(async (selectName?: string) => {
    setLoadingProfiles(true);
    setProfilesError(null);
    try {
      const d = await api.get<{ profiles?: AgentProfile[] }>(paths.generate.profiles);
      const list = d.profiles || [];
      setProfiles(list);
      if (list.length === 0) {
        validationSeq.current += 1;
        setValidating(false);
        setSelectedIdx(-1);
        setSource("");
        setSavedSource("");
        setValidation(null);
        return list;
      }
      let idx = 0;
      if (selectName) {
        const found = list.findIndex((p) => p.name === selectName);
        if (found >= 0) idx = found;
      } else if (selectedIdxRef.current >= 0) {
        idx = Math.min(selectedIdxRef.current, list.length - 1);
      }
      selectProfile(idx, list);
      return list;
    } catch (e) {
      const msg = e instanceof Error ? e.message : t("profiles.toast.load_profiles_failed");
      setProfilesError(msg);
      toast.error(msg);
      return [];
    } finally {
      setLoadingProfiles(false);
    }
  }, [selectProfile, t]);

  useEffect(() => {
    void loadProfiles();
  }, [loadProfiles]);

  const validate = useCallback(async (name: string, content: string) => {
    const seq = ++validationSeq.current;
    setValidating(true);
    try {
      const data = await api.postJson<ProfileValidationData>(paths.generate.profileValidate, {
        name,
        format: "cs",
        content,
      });
      if (seq === validationSeq.current) setValidation(data);
      return data;
    } finally {
      if (seq === validationSeq.current) setValidating(false);
    }
  }, []);

  const save = useCallback(async (name: string, content: string) => {
    const d = await api.postJson<ImportResponse>(paths.generate.profileImportText, {
      name,
      format: "cs",
      content,
    });
    const savedName = d.profile?.name || name;
    await loadProfiles(savedName);
    return d;
  }, [loadProfiles]);

  const remove = useCallback(async (name: string) => {
    await api.del(paths.generate.profileDelete(name));
    return loadProfiles();
  }, [loadProfiles]);

  const importFile = useCallback(async (file: File) => {
    const fd = new FormData();
    fd.append("profile", file);
    const d = await api.postFormData<ImportResponse>(paths.generate.profileImport, fd);
    if (d.profile?.name) await loadProfiles(d.profile.name);
    return d;
  }, [loadProfiles]);

  return {
    profiles,
    setProfiles,
    selectedIdx,
    setSelectedIdx,
    source,
    setSource,
    savedSource,
    setSavedSource,
    dirty: source !== savedSource,
    loadingProfiles,
    profilesError,
    validating,
    validation,
    setValidation,
    selectProfile,
    loadProfiles,
    validate,
    save,
    remove,
    importFile,
  };
}
