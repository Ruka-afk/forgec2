"use client";

import { useState } from "react";

import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { StatusDot } from "@/components/ui/status-dot";
import { Eye, EyeOff, X } from "lucide-react";

const PROVIDER_DEFAULT_MODELS: Record<string, string> = {
  deepseek: "deepseek-chat",
  openai: "gpt-4o-mini",
  claude: "claude-3-5-sonnet-latest",
  qianwen: "qwen-plus",
  zhipu: "glm-4-flash",
  longcat: "LongCat-Flash-Chat",
  custom: "",
};

interface AIConfigPanelProps {
  enabled: boolean;
  setEnabled: (v: boolean) => void;
  provider: string;
  setProvider: (v: string) => void;
  model: string;
  setModel: (v: string) => void;
  apiKey: string;
  setApiKey: (v: string) => void;
  endpoint: string;
  setEndpoint: (v: string) => void;
  systemPrompt: string;
  setSystemPrompt: (v: string) => void;
  engagementNotes: string;
  setEngagementNotes: (v: string) => void;
  allowExecute: boolean;
  setAllowExecute: (v: boolean) => void;
  configSaving: boolean;
  onClose: () => void;
  onSave: () => void;
}

export function AIConfigPanel(props: AIConfigPanelProps) {
  const { t } = useI18n();
  const [showApiKey, setShowApiKey] = useState(false);
  const {
    enabled, setEnabled,
    provider, setProvider, model, setModel, apiKey, setApiKey,
    endpoint, setEndpoint, systemPrompt, setSystemPrompt,
    engagementNotes, setEngagementNotes,
    allowExecute, setAllowExecute, configSaving, onClose, onSave,
  } = props;
  const changeProvider = (nextProvider: string) => {
    const knownDefault = Object.values(PROVIDER_DEFAULT_MODELS).includes(model);
    setProvider(nextProvider);
    if (!model.trim() || knownDefault) setModel(PROVIDER_DEFAULT_MODELS[nextProvider] || "");
  };

  return (
    <div className="min-h-full bg-card">
      <div className="sticky top-0 z-10 flex items-center justify-between border-b border-border/75 bg-card/95 px-5 py-4 backdrop-blur">
        <div>
          <h2 className="text-base font-semibold text-foreground">{t("ai.config_title")}</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">{provider} · {model || t("ai.model_custom")}</p>
        </div>
        <Button variant="ghost" size="icon" onClick={onClose} aria-label={t("ai.close_settings")}>
          <X className="size-4" />
        </Button>
      </div>
      <div className="space-y-5 p-5">
        <Label className="flex min-h-12 cursor-pointer items-center justify-between gap-3 rounded-xl border border-border bg-muted/35 px-3 py-2.5 select-none">
          <span className="flex items-center gap-2">
            <StatusDot tone={enabled ? "success" : "muted"} size="sm" />
            <span>
              <span className="block text-sm font-medium text-foreground">{t("ai.enable_ai")}</span>
              <span className="block text-(--fs-xs-sm) text-muted-foreground">
                {enabled ? t("ai.status_enabled") : t("ai.status_disabled")}
              </span>
            </span>
          </span>
          <Checkbox checked={enabled} onCheckedChange={(v) => setEnabled(v === true)} />
        </Label>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.provider")}</span>
            <Select value={provider} onValueChange={(v) => v && changeProvider(v)}>
              <SelectTrigger aria-label={t("ai.provider")} className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="deepseek">DeepSeek</SelectItem>
                <SelectItem value="openai">OpenAI</SelectItem>
                <SelectItem value="claude">Claude</SelectItem>
                <SelectItem value="qianwen">Qianwen</SelectItem>
                <SelectItem value="zhipu">Zhipu (GLM)</SelectItem>
                <SelectItem value="longcat">LongCat</SelectItem>
                <SelectItem value="custom">{t("ai.model_custom")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.model")}</span>
            <Input maxLength={200} type="text" aria-label={t("ai.model")} value={model} onChange={(e) => setModel(e.target.value)} className="font-mono w-full" />
          </div>
          <div className="md:col-span-2">
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.endpoint")}</span>
            <Input maxLength={2048} type="url" aria-label={t("ai.endpoint")} value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://api.openai.com/v1" className="font-mono w-full text-xs" />
          </div>
          <div className="md:col-span-2">
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.api_key")}</span>
            <div className="relative">
              <Input maxLength={16 * 1024} type={showApiKey ? "text" : "password"} aria-label={t("ai.api_key")} autoComplete="new-password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." className="w-full pr-10 font-mono text-xs" />
              <Button
                type="button"
                variant="ghost"
                size="icon-xs"
                onClick={() => setShowApiKey((visible) => !visible)}
                className="absolute right-2 top-1/2 -translate-y-1/2"
                aria-label={showApiKey ? t("ai.hide_api_key") : t("ai.show_api_key")}
              >
                {showApiKey ? <EyeOff className="size-3.5" /> : <Eye className="size-3.5" />}
              </Button>
            </div>
            <span className="block text-(--fs-xs-sm) text-muted-foreground mt-0.5">{t("ai.api_key_hint")}</span>
          </div>
        </div>
        <div>
          <span className="block text-xs text-muted-foreground mb-1">{t("ai.system_prompt")}</span>
          <Textarea maxLength={16 * 1024} aria-label={t("ai.system_prompt")} value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} rows={3} className="w-full text-xs resize-y" placeholder={t("ai.system_prompt_placeholder")} />
          <span className="block text-(--fs-xs-sm) text-muted-foreground mt-0.5">{t("ai.system_prompt_hint")}</span>
        </div>
        <div>
          <span className="block text-xs text-muted-foreground mb-1">{t("ai.engagement_notes")}</span>
          <Textarea maxLength={8000} aria-label={t("ai.engagement_notes")} value={engagementNotes} onChange={(e) => setEngagementNotes(e.target.value)} rows={4} className="w-full text-xs resize-y font-mono" />
          <span className="mt-0.5 flex justify-between gap-3 text-(--fs-xs-sm) text-muted-foreground">
            <span>{t("ai.engagement_notes_hint")}</span>
            <span className="shrink-0 font-mono">{engagementNotes.length}/8000</span>
          </span>
        </div>
        <Label className="flex items-start gap-3 cursor-pointer select-none">
          <Checkbox checked={allowExecute} onCheckedChange={(v) => setAllowExecute(v === true)} className="mt-1" />
          <span>
            <span className="text-sm text-muted-foreground">{t("ai.allow_execute")}</span>
            <span className="block text-(--fs-xs-sm) text-warning mt-0.5">{t("ai.allow_execute_warn")}</span>
          </span>
        </Label>
        <Button type="button" onClick={onSave} size="lg" disabled={configSaving} className="sticky bottom-4 w-full shadow-lg">
          {configSaving ? <><Spinner size="xs" className="mr-2" />{t("common.saving")}</> : t("common.save")}
        </Button>
      </div>
    </div>
  );
}
