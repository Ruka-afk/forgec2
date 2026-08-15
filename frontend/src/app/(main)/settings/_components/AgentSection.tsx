import { AgentForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import Toggle from "./Toggle";
import { Bot, Save } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function AgentSection({
  form, setForm, saving, onSave,
}: {
  form: AgentForm;
  setForm: React.Dispatch<React.SetStateAction<AgentForm>>;
  saving: boolean;
  onSave: (e: React.FormEvent) => void;
}) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <div className="bg-success/10 border-b border-success/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Bot className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.agent.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.agent.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <Label htmlFor="agent-heartbeat-interval" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.heartbeat_interval")}</Label>
              <Input id="agent-heartbeat-interval" type="number" min={0} max={300} value={form.interval} onChange={(e) => setForm({ ...form, interval: Number(e.target.value) })}  />
              <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("settings.agent.heartbeat_hint")}</p>
            </div>
            <div>
              <Label htmlFor="agent-jitter" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.jitter")}</Label>
              <Input id="agent-jitter" type="number" min={0} max={100} value={form.jitter} onChange={(e) => setForm({ ...form, jitter: Number(e.target.value) })}  />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.skip_tls")}</span>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.skip_tls} onChange={(v) => setForm({ ...form, skip_tls: v })} />
                <span className="text-sm text-muted-foreground">{form.skip_tls ? t("settings.agent.enabled") : t("settings.agent.disabled")}</span>
              </div>
            </div>
          </div>
          <div>
              <Label htmlFor="agent-user-agent" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.user_agent")}</Label>
              <Input id="agent-user-agent" type="text" value={form.user_agent} onChange={(e) => setForm({ ...form, user_agent: e.target.value })} className="font-mono" />
          </div>
          <div className="border-t pt-4">
            <h3 className="text-sm font-medium text-muted-foreground mb-3">{t("settings.agent.working_hours")}</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <Label htmlFor="agent-working-start" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.working_start")}</Label>
                <Input id="agent-working-start" type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })}  />
              </div>
              <div>
                <Label htmlFor="agent-working-end" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.working_end")}</Label>
                <Input id="agent-working-end" type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })}  />
              </div>
              <div>
                <Label htmlFor="agent-working-tz" className="block text-xs text-muted-foreground mb-1.5">{t("settings.agent.timezone")}</Label>
                <Input id="agent-working-tz" type="text" placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono" />
                <p className="text-(--fs-micro-sm) text-muted-foreground mt-1">{t("settings.agent.tz_hint")}</p>
              </div>
            </div>
            <p className="text-(--fs-micro-sm) text-muted-foreground mt-2">{t("settings.agent.working_hint")}</p>
          </div>
          <Button type="submit" size="lg" disabled={saving} className="px-6 text-sm font-medium transition-colors disabled:opacity-50">
            <Save className="w-4 h-4" />{t("settings.agent.save")}
          </Button>
        </form>
      </div>
    </Card>
  );
}
