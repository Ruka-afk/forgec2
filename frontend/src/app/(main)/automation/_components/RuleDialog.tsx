"use client";

import { useI18n } from "@/lib/i18n";
import { Spinner } from "@/components/UI";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { FlaskConical } from "lucide-react";
import type { WebhookActionParams, WebhookType } from "./types";

export interface RuleFormState {
  name: string;
  event_type: string;
  action_type: string;
  action_config: string;
  webhook: WebhookActionParams;
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
                  {sendingTest ? <Spinner size="xs" /> : <FlaskConical className="w-4 h-4" />}
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
