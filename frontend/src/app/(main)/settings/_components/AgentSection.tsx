import { AgentForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import Toggle from "./Toggle";
import { Bot, Save } from "lucide-react";

export default function AgentSection({
  form, setForm, saving, onSave,
}: {
  form: AgentForm;
  setForm: React.Dispatch<React.SetStateAction<AgentForm>>;
  saving: boolean;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-emerald-600 to-emerald-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Bot className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Implant Default Config</h2><p className="text-xs text-emerald-200">Default values for new implants</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Heartbeat Interval (sec)</span>
              <Input aria-label="Heartbeat interval in seconds" name="input-0" type="number" min={0} max={300} value={form.interval} onChange={(e) => setForm({ ...form, interval: Number(e.target.value) })}  />
              <p className="text-(--font-size-micro-sm) text-muted-foreground mt-1">0 = real-time mode</p>
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Jitter (%)</span>
              <Input aria-label="Jitter percentage" name="input-1" type="number" min={0} max={100} value={form.jitter} onChange={(e) => setForm({ ...form, jitter: Number(e.target.value) })}  />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Skip TLS Verification</span>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.skip_tls} onChange={(v) => setForm({ ...form, skip_tls: v })} />
                <span className="text-sm text-muted-foreground">{form.skip_tls ? "Enabled" : "Disabled"}</span>
              </div>
            </div>
          </div>
          <div>
            <span className="block text-xs text-muted-foreground mb-1.5">Default User-Agent</span>
            <Input aria-label="Default User-Agent string" name="input-2" type="text" value={form.user_agent} onChange={(e) => setForm({ ...form, user_agent: e.target.value })} className="font-mono" />
          </div>
          <div className="border-t pt-4">
            <h3 className="text-sm font-medium text-muted-foreground mb-3">Working Hours</h3>
            <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Start Time (HH:MM)</span>
                <Input aria-label="Working hours start time" type="time" value={form.working_start} onChange={(e) => setForm({ ...form, working_start: e.target.value })}  />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">End Time (HH:MM)</span>
                <Input aria-label="Working hours end time" type="time" value={form.working_end} onChange={(e) => setForm({ ...form, working_end: e.target.value })}  />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Timezone</span>
                <Input aria-label="Working hours timezone" type="text" placeholder="UTC" value={form.working_tz} onChange={(e) => setForm({ ...form, working_tz: e.target.value })} className="font-mono" />
                <p className="text-(--font-size-micro-sm) text-muted-foreground mt-1">IANA tz (e.g. America/New_York)</p>
              </div>
            </div>
            <p className="text-(--font-size-micro-sm) text-muted-foreground mt-2">Agents will only communicate during these hours (local to specified timezone)</p>
          </div>
          <Button type="submit" disabled={saving} className="h-11 px-6 bg-emerald-600 hover:bg-emerald-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <Save className="w-4 h-4" />Save Agent Config
          </Button>
        </form>
      </div>
    </Card>
  );
}
