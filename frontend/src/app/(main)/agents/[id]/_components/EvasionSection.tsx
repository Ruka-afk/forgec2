"use client";

import { useState } from "react";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Banner } from "@/components/ui/banner";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "sonner";
import { api } from "@/lib/api";
import { paths } from "@/lib/api-paths";
import { useI18n } from "@/lib/i18n";
import { Shield, Zap, AlertTriangle } from "lucide-react";
import { EVASION_TECHNIQUES, EVASION_GROUPS } from "../../_components/evasion-techniques";

interface EvasionSectionProps {
  agentId: string;
  online: boolean;
}

export default function EvasionSection({ agentId, online }: EvasionSectionProps) {
  const { t } = useI18n();
  const [selectedTechnique, setSelectedTechnique] = useState("");
  const [sending, setSending] = useState(false);
  const [confirmStep, setConfirmStep] = useState(false);

  const handleExecute = async () => {
    if (!selectedTechnique) return;
    if (!confirmStep) {
      setConfirmStep(true);
      return;
    }
    setSending(true);
    try {
      await api.post(paths.agents.runEvasion(agentId), { technique: selectedTechnique });
      const tech = EVASION_TECHNIQUES.find(tech => tech.value === selectedTechnique);
      toast.success(t("agents.evasion_sent").replace("{technique}", t(tech?.labelKey || "")));
      setSelectedTechnique("");
      setConfirmStep(false);
    } catch {
      toast.error(t("agents.evasion_failed"));
    }
    setSending(false);
  };

  return (
    <Card className="mb-4 gap-0">
      <div className="px-4 py-3 border-b border-border">
        <h3 className="text-sm font-semibold text-foreground"><Shield className="w-3.5 h-3.5" />{t("agents.evasion_title")}</h3>
      </div>
      <div className="p-3">
        <p className="text-xs text-muted-foreground mb-3">{t("agents.evasion_desc")}</p>
        {confirmStep && (
          <Banner tone="destructive" icon={<AlertTriangle className="w-3.5 h-3.5" />} className="mb-3 text-xs">
            {t("agents.evasion_confirm_msg").replace("{technique}", t(EVASION_TECHNIQUES.find(tech => tech.value === selectedTechnique)?.labelKey || ""))}
          </Banner>
        )}
        <div className="flex gap-2">
          <Select value={selectedTechnique} onValueChange={(v) => { if (v !== null) { setSelectedTechnique(v); setConfirmStep(false); } }}>
            <SelectTrigger className="flex-1">
              <SelectValue placeholder={t("agents.evasion_select")} />
            </SelectTrigger>
            <SelectContent>
              {EVASION_GROUPS.map((group) => (
                <div key={group}>
                  <div className="px-2 py-1 text-xs font-semibold text-muted-foreground">{group}</div>
                  {EVASION_TECHNIQUES.filter((tech) => tech.group === group).map((tech) => (
                    <SelectItem key={tech.value} value={tech.value}>{t(tech.labelKey)}</SelectItem>
                  ))}
                </div>
              ))}
            </SelectContent>
          </Select>
          <Button
            onClick={handleExecute}
            disabled={!online || sending || !selectedTechnique}
            variant={confirmStep ? "destructive" : "default"}
            className="shrink-0"
          >
            <Zap className="w-4 h-4" />
            {sending ? t("agents.evasion_executing") : confirmStep ? t("agents.evasion_confirm") : t("agents.evasion_execute")}
          </Button>
        </div>
      </div>
    </Card>
  );
}
