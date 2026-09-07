"use client";

import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

interface AlertRuleFormState {
  name: string;
  type: string;
  threshold: number;
  description: string;
}

interface Props {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  alertRuleForm: AlertRuleFormState;
  setAlertRuleForm: React.Dispatch<React.SetStateAction<AlertRuleFormState>>;
  onSave: () => void;
}

export function AlertRuleDialog({ open, onOpenChange, alertRuleForm, setAlertRuleForm, onSave }: Props) {
  const { t } = useI18n();
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t("auto.new_alert_rule")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label>{t("auto.name")}</Label>
            <Input value={alertRuleForm.name} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, name: e.target.value })} className="mt-1" aria-label={t("auto.name")} />
          </div>
          <div>
            <Label>{t("auto.type")}</Label>
            <Select value={alertRuleForm.type} onValueChange={(v) => setAlertRuleForm({ ...alertRuleForm, type: v ?? "" })}>
              <SelectTrigger className="w-full mt-1" aria-label={t("auto.type")}><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="agent_offline">{t("auto.type_agent_offline")}</SelectItem>
                <SelectItem value="agent_online">{t("auto.type_agent_online")}</SelectItem>
                <SelectItem value="cpu_high">{t("auto.type_cpu_high")}</SelectItem>
                <SelectItem value="memory_high">{t("auto.type_memory_high")}</SelectItem>
                <SelectItem value="credential_found">{t("auto.type_credential_found")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <Label>{t("auto.threshold")}</Label>
            <Input type="number" value={alertRuleForm.threshold} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, threshold: parseInt(e.target.value) || 0 })} className="mt-1" aria-label={t("auto.threshold")} />
          </div>
          <div>
            <Label>{t("auto.description")}</Label>
            <Input value={alertRuleForm.description} onChange={(e) => setAlertRuleForm({ ...alertRuleForm, description: e.target.value })} className="mt-1" aria-label={t("auto.description")} />
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
