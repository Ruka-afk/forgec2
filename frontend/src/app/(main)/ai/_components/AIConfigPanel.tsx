"use client";

import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { StatusDot } from "@/components/ui/status-dot";
import { X } from "lucide-react";

interface AIConfigPanelProps {
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
  allowExecute: boolean;
  setAllowExecute: (v: boolean) => void;
  configSaving: boolean;
  onClose: () => void;
  onSave: () => void;
}

export function AIConfigPanel(props: AIConfigPanelProps) {
  const { t } = useI18n();
  const {
    provider, setProvider, model, setModel, apiKey, setApiKey,
    endpoint, setEndpoint, systemPrompt, setSystemPrompt,
    allowExecute, setAllowExecute, configSaving, onClose, onSave,
  } = props;

  return (
    <Card className="shrink-0 mb-3 p-(--card-spacing)">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-semibold text-foreground">{t("ai.config_title")}</h2>
        <Button variant="ghost" size="icon" onClick={onClose} aria-label={t("ai.close_settings")}>
          <X className="w-4 h-4" />
        </Button>
      </div>
      <div className="space-y-4">
        <div className="flex items-center gap-3">
          <span className="flex items-center gap-2 cursor-pointer">
            <StatusDot tone="success" size="sm" />
            <span className="text-sm text-muted-foreground">{t("ai.enable_ai")}</span>
          </span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.provider")}</span>
            <Select value={provider} onValueChange={(v) => v && setProvider(v)}>
              <SelectTrigger aria-label={t("ai.provider")} className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="deepseek">DeepSeek</SelectItem>
                <SelectItem value="openai">OpenAI</SelectItem>
                <SelectItem value="claude">Claude</SelectItem>
                <SelectItem value="qianwen">Qianwen</SelectItem>
                <SelectItem value="longcat">LongCat</SelectItem>
                <SelectItem value="custom">{t("ai.model_custom")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.model")}</span>
            <Input type="text" aria-label={t("ai.model")} value={model} onChange={(e) => setModel(e.target.value)} className="font-mono w-full" />
          </div>
          <div className="md:col-span-2">
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.endpoint")}</span>
            <Input type="text" aria-label={t("ai.endpoint")} value={endpoint} onChange={(e) => setEndpoint(e.target.value)} placeholder="https://api.openai.com/v1" className="font-mono w-full text-xs" />
          </div>
          <div className="md:col-span-2">
            <span className="block text-xs text-muted-foreground mb-1">{t("ai.api_key")}</span>
            <Input type="password" aria-label={t("ai.api_key")} autoComplete="new-password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." className="font-mono w-full text-xs" />
            <span className="block text-(--fs-xs-sm) text-muted-foreground mt-0.5">{t("ai.api_key_hint")}</span>
          </div>
        </div>
        <div>
          <span className="block text-xs text-muted-foreground mb-1">{t("ai.system_prompt")}</span>
          <Textarea aria-label={t("ai.system_prompt")} value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} rows={3} className="w-full text-xs resize-y" />
        </div>
        <Label className="flex items-start gap-3 cursor-pointer select-none">
          <Checkbox checked={allowExecute} onCheckedChange={(v) => setAllowExecute(v === true)} className="mt-1" />
          <span>
            <span className="text-sm text-muted-foreground">{t("ai.allow_execute")}</span>
            <span className="block text-(--fs-xs-sm) text-warning mt-0.5">{t("ai.allow_execute_warn")}</span>
          </span>
        </Label>
        <Button type="button" onClick={onSave} size="lg" disabled={configSaving} className="w-full">
          {configSaving ? <><Spinner size="xs" className="mr-2" />{t("common.saving")}</> : t("common.save")}
        </Button>
      </div>
    </Card>
  );
}
