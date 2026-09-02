"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { AIConfig } from "./types";

export function useAIConfig() {
  const { t } = useI18n();
  const [enabled, setEnabled] = useState(false);
  const [provider, setProvider] = useState("deepseek");
  const [model, setModel] = useState("deepseek-chat");
  const [apiKey, setApiKey] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [engagementNotes, setEngagementNotes] = useState("");
  const [allowExecute, setAllowExecute] = useState(false);
  const [configSaving, setConfigSaving] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [hasApiKey, setHasApiKey] = useState(false);
  const [configLoading, setConfigLoading] = useState(true);
  const loadSeqRef = useRef(0);
  const saveLockRef = useRef(false);

  const loadConfig = useCallback(async () => {
    const seq = ++loadSeqRef.current;
    setConfigLoading(true);
    try {
      const data = await api.get(paths.ai.root);
      if (seq !== loadSeqRef.current) return;
      if (data.AIConfig) {
        const cfg = data.AIConfig as AIConfig;
        setEnabled(Boolean(cfg.enabled));
        if (cfg.provider) setProvider(cfg.provider);
        if (cfg.model) setModel(cfg.model);
        if (cfg.endpoint) setEndpoint(cfg.endpoint);
        if (cfg.system_prompt) setSystemPrompt(cfg.system_prompt);
        setEngagementNotes(typeof cfg.engagement_notes === "string" ? cfg.engagement_notes : "");
        setAllowExecute(Boolean(cfg.allow_execute));
        setHasApiKey(Boolean(cfg.has_api_key));
      }
    } catch (e) {
      if (seq !== loadSeqRef.current) return;
      toast.error(e instanceof Error ? e.message : t("ai.toast.load_config_failed"));
    } finally {
      if (seq === loadSeqRef.current) setConfigLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const handleSaveConfig = async () => {
    if (saveLockRef.current) return;
    saveLockRef.current = true;
    try {
      setConfigSaving(true);
      const data = await api.postJson(paths.ai.config, {
        enabled,
        provider,
        model,
        api_key: apiKey,
        endpoint,
        system_prompt: systemPrompt,
        engagement_notes: engagementNotes,
        allow_execute: allowExecute,
      });
      if (data.success) {
        // The API never echoes the secret. Update the local capability
        // immediately, then refresh the redacted config so first-time setup
        // enables the composer without requiring a full page reload.
        if (apiKey.trim()) setHasApiKey(true);
        setApiKey("");
        setShowSettings(false);
        await loadConfig();
        toast.success(t("ai.toast.config_saved"));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.save_config_failed"));
    } finally {
      setConfigSaving(false);
      saveLockRef.current = false;
    }
  };

  return {
    enabled,
    setEnabled,
    provider,
    setProvider,
    model,
    setModel,
    apiKey,
    setApiKey,
    endpoint,
    setEndpoint,
    systemPrompt,
    setSystemPrompt,
    engagementNotes,
    setEngagementNotes,
    allowExecute,
    setAllowExecute,
    configSaving,
    showSettings,
    setShowSettings,
    loadConfig,
    handleSaveConfig,
    hasApiKey,
    configLoading,
  };
}
