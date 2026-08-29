"use client";

import { useEffect, useState } from "react";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/ui/spinner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { FlaskConical } from "lucide-react";
import type { WebhookActionParams, WebhookType } from "./types";

interface RuleFormState {
  name: string;
  event_type: string;
  action_type: string;
  action_config: string;
  webhook: WebhookActionParams;
  macro_id: number | null;
  macro_stop_on_error: boolean;
}

interface MacroOption {
  id?: number;
  name: string;
}

interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  ruleForm: RuleFormState;
  setRuleForm: React.Dispatch<React.SetStateAction<RuleFormState>>;
  sendingTest: boolean;
  onTestWebhook: () => void;
  onSave: () => void;
}

export function RuleDialog({ open, onOpenChange, ruleForm, setRuleForm, sendingTest, onTestWebhook, onSave }: Props) {
  const { t } = useI18n();
  const [macros, setMacros] = useState<MacroOption[]>([]);

  // Load macro options when the run_macro action is picked.
  useEffect(() => {
    if (!open || ruleForm.action_type !== "run_macro") return;
    api.get<{ macros?: MacroOption[] }>(paths.macros.list)
      .then((d) => setMacros(d.macros || []))
      .catch(() => setMacros([]));
  }, [open, ruleForm.action_type]);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("auto.new_automation_rule")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>{t("auto.rule_name")}</Label>
            <Input value={ruleForm.name} onChange={(e) => setRuleForm({ ...ruleForm, name: e.target.value })} placeholder={t("auto.rule_name_placeholder")} className="mt-1" aria-label={t("auto.rule_name")} />
          </div>
          <div>
            <Label>{t("auto.event_type")}</Label>
            <Select value={ruleForm.event_type} onValueChange={(v) => setRuleForm({ ...ruleForm, event_type: v ?? "" })}>
              <SelectTrigger className="w-full mt-1" aria-label={t("auto.event_type")}><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="agent.checkin">agent.checkin</SelectItem>
                <SelectItem value="agent.disconnect">agent.disconnect</SelectItem>
                <SelectItem value="task.complete">task.complete</SelectItem>
                <SelectItem value="task.fail">task.fail</SelectItem>
                <SelectItem value="credential.found">credential.found</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label>{t("auto.action_type")}</Label>
            <Select value={ruleForm.action_type} onValueChange={(v) => setRuleForm({ ...ruleForm, action_type: v ?? "" })}>
              <SelectTrigger className="w-full mt-1" aria-label={t("auto.action_type")}><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="command">{t("auto.action_send_command")}</SelectItem>
                <SelectItem value="webhook">{t("auto.action_send_webhook")}</SelectItem>
                <SelectItem value="notify">{t("auto.action_show_alert")}</SelectItem>
                <SelectItem value="run_macro">{t("auto.action_run_macro")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {ruleForm.action_type === "command" && (
            <div>
              <Label>{t("auto.command")}</Label>
              <Input placeholder={t("auto.command_placeholder")} value={ruleForm.action_config} onChange={(e) => setRuleForm({ ...ruleForm, action_config: e.target.value })} className="mt-1" aria-label={t("auto.command")} />
            </div>
          )}
          {ruleForm.action_type === "notify" && (
            <div>
              <Label>{t("auto.notification_message")}</Label>
              <Input placeholder={t("auto.notification_placeholder")} value={ruleForm.action_config} onChange={(e) => setRuleForm({ ...ruleForm, action_config: e.target.value })} className="mt-1" aria-label={t("auto.notification_message")} />
            </div>
          )}
          {ruleForm.action_type === "run_macro" && (
            <div className="space-y-3">
              <div>
                <Label>{t("auto.macro_select")}</Label>
                <Select
                  value={ruleForm.macro_id != null ? String(ruleForm.macro_id) : ""}
                  onValueChange={(v) => setRuleForm({ ...ruleForm, macro_id: v ? Number(v) : null })}
                >
                  <SelectTrigger className="w-full mt-1" aria-label={t("auto.macro_select")}><SelectValue placeholder={t("auto.macro_select_ph")} /></SelectTrigger>
                  <SelectContent>
                    {macros.map((m) => (
                      <SelectItem key={m.id} value={String(m.id)}>{m.name}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {macros.length === 0 && (
                  <p className="text-xs text-muted-foreground mt-1">{t("auto.macro_none_hint")}</p>
                )}
              </div>
              <label className="flex items-center gap-2 cursor-pointer select-none">
                <Switch
                  checked={ruleForm.macro_stop_on_error}
                  onCheckedChange={(v) => setRuleForm({ ...ruleForm, macro_stop_on_error: v === true })}
                />
                <span className="text-sm text-muted-foreground">{t("macros.global_stop_on_error")}</span>
              </label>
              <p className="text-xs text-muted-foreground">{t("auto.macro_trigger_hint")}</p>
            </div>
          )}
          {ruleForm.action_type === "webhook" && (
            <div className="space-y-4">
              <div>
                <Label>{t("auto.webhook_type")}</Label>
                <Select value={ruleForm.webhook.type} onValueChange={(v) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, type: v as WebhookType } })}>
                  <SelectTrigger className="w-full mt-1" aria-label={t("auto.webhook_type")}><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="generic">{t("auto.webhook_type_generic")}</SelectItem>
                    <SelectItem value="slack">Slack</SelectItem>
                    <SelectItem value="discord">Discord</SelectItem>
                    <SelectItem value="email">{t("auto.webhook_type_email")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {ruleForm.webhook.type !== "email" && (
                <>
                  <div>
                    <Label>{t("auto.webhook_url")}</Label>
                    <Input placeholder="https://hooks.example.com/..." value={ruleForm.webhook.url} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, url: e.target.value } })} className="mt-1" aria-label={t("auto.webhook_url")} />
                  </div>
                  <div>
                    <Label>{t("auto.webhook_secret")}</Label>
                    <Input placeholder={t("automation.hmac_key_ph")} value={ruleForm.webhook.secret} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, secret: e.target.value } })} className="mt-1" aria-label={t("auto.webhook_secret")} />
                  </div>
                </>
              )}
              {ruleForm.webhook.type === "email" && (
                <>
                  <div>
                    <Label>{t("auto.smtp_server")}</Label>
                    <Input placeholder="smtp.gmail.com" value={ruleForm.webhook.smtp_host} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_host: e.target.value } })} className="mt-1" aria-label={t("auto.smtp_server")} />
                  </div>
                  <div className="flex gap-2">
                    <div className="flex-1">
                      <Label>{t("auto.smtp_port")}</Label>
                      <Input type="number" placeholder="587" value={ruleForm.webhook.smtp_port} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_port: parseInt(e.target.value) || 587 } })} className="mt-1" aria-label={t("auto.smtp_port")} />
                    </div>
                    <div className="flex-1">
                      <Label>{t("auto.smtp_from")}</Label>
                      <Input placeholder="alerts@example.com" value={ruleForm.webhook.from} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, from: e.target.value } })} className="mt-1" aria-label={t("auto.smtp_from")} />
                    </div>
                  </div>
                  <div>
                    <Label>{t("auto.smtp_username")}</Label>
                    <Input placeholder="user@gmail.com" value={ruleForm.webhook.smtp_user} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_user: e.target.value } })} className="mt-1" aria-label={t("auto.smtp_username")} />
                  </div>
                  <div>
                    <Label>{t("auto.smtp_password")}</Label>
                    <Input type="password" placeholder={t("automation.app_password_ph")} value={ruleForm.webhook.smtp_pass} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, smtp_pass: e.target.value } })} className="mt-1" aria-label={t("auto.smtp_password")} />
                  </div>
                  <div>
                    <Label>{t("auto.smtp_to")}</Label>
                    <Input placeholder="admin@example.com" value={ruleForm.webhook.to} onChange={(e) => setRuleForm({ ...ruleForm, webhook: { ...ruleForm.webhook, to: e.target.value } })} className="mt-1" aria-label={t("auto.smtp_to")} />
                  </div>
                </>
              )}
              <div className="flex justify-end">
                <Button onClick={onTestWebhook} disabled={sendingTest} variant="ghost" size="sm">
                  {sendingTest ? <Spinner size="xs" /> : <FlaskConical className="size-4" />}
                  {sendingTest ? t("auto.sending") : t("auto.test_notification")}
                </Button>
              </div>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} variant="ghost">{t("auto.cancel")}</Button>
          <Button onClick={onSave}>{t("auto.save_rule")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
