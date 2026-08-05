"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { toast } from "sonner";
import { useI18n } from "@/lib/i18n";
import type { AIConfig } from "./types";

export function useAIConfig() {
  const { t } = useI18n();
  const [provider, setProvider] = useState("deepseek");
  const [model, setModel] = useState("deepseek-chat");
  const [apiKey, setApiKey] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");
  const [allowExecute, setAllowExecute] = useState(false);
  const [configSaving, setConfigSaving] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  const loadConfig = useCallback(async () => {
    try {
      const data = await api.get(paths.ai.root);
      if (data.AIConfig) {
        const cfg = data.AIConfig as AIConfig;
        if (cfg.provider) setProvider(cfg.provider);
        if (cfg.model) setModel(cfg.model);
        if (cfg.endpoint) setEndpoint(cfg.endpoint);
        if (cfg.system_prompt) setSystemPrompt(cfg.system_prompt);
        setAllowExecute(Boolean(cfg.allow_execute));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.load_config_failed"));
    }
  }, [t]);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const handleSaveConfig = async () => {
    try {
      setConfigSaving(true);
      const data = await api.postJson(paths.ai.config, {
        enabled: true,
        provider,
        model,
        api_key: apiKey,
        endpoint,
        system_prompt: systemPrompt,
        allow_execute: allowExecute,
      });
      if (data.success) {
        setShowSettings(false);
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("ai.toast.save_config_failed"));
    } finally {
      setConfigSaving(false);
    }
  };

  return {
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
    allowExecute,
    setAllowExecute,
    configSaving,
    showSettings,
    setShowSettings,
    loadConfig,
    handleSaveConfig,
  };
}
