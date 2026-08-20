"use client";

import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

interface WebhookFormState {
  name: string;
  url: string;
  event_type: string;
  method: string;
}

interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  webhookForm: WebhookFormState;
  setWebhookForm: React.Dispatch<React.SetStateAction<WebhookFormState>>;
  onSave: () => void;
}

export function WebhookDialog({ open, onOpenChange, webhookForm, setWebhookForm, onSave }: Props) {
  const { t } = useI18n();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("auto.new_webhook_dialog")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>{t("auto.name")}</Label>
            <Input value={webhookForm.name} onChange={(e) => setWebhookForm({ ...webhookForm, name: e.target.value })} placeholder="e.g. Slack alert" className="mt-1" aria-label={t("auto.name")} />
          </div>
          <div>
            <Label>{t("auto.url")}</Label>
            <Input placeholder="https://hooks.example.com/forgec2" value={webhookForm.url} onChange={(e) => setWebhookForm({ ...webhookForm, url: e.target.value })} className="mt-1" aria-label={t("auto.url")} />
          </div>
          <div>
            <Label>{t("auto.event_type")}</Label>
            <Select value={webhookForm.event_type} onValueChange={(v) => setWebhookForm({ ...webhookForm, event_type: v ?? "" })}>
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
            <Label>{t("auto.request_method")}</Label>
            <Select value={webhookForm.method} onValueChange={(v) => setWebhookForm({ ...webhookForm, method: v ?? "" })}>
              <SelectTrigger className="w-full mt-1" aria-label={t("auto.request_method")}><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="POST">POST</SelectItem>
                <SelectItem value="PUT">PUT</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>
        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} variant="ghost">{t("auto.cancel")}</Button>
          <Button onClick={onSave}>{t("auto.save")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
