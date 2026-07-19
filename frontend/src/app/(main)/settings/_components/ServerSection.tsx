import { SettingsData, ServerForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import Toggle from "./Toggle";
import { Info, Lock, Save, Server, Unlock } from "lucide-react";

export default function ServerSection({
  data, form, setForm, saving, inputCls, onSave,
}: {
  data: SettingsData;
  form: ServerForm;
  setForm: React.Dispatch<React.SetStateAction<ServerForm>>;
  saving: boolean;
  inputCls: string;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-indigo-600 to-indigo-700 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Server className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Server Configuration</h2><p className="text-xs text-indigo-200">Listen address, log level, transport settings</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">HTTP Port</span>
              <Input aria-label="HTTP port" name="input-0" type="number" defaultValue={data.server_port} readOnly className={`${inputCls} bg-muted cursor-not-allowed text-muted-foreground`} />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Listen Address</span>
              <Input aria-label="Listen address" name="input-1" type="text" defaultValue={data.server_address ?? ""} readOnly className={`${inputCls} bg-muted cursor-not-allowed text-muted-foreground font-mono`} />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Log Level</span>
              <Select value={form.log_level} onValueChange={(v) => v && setForm({ ...form, log_level: v })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="debug">Debug</SelectItem>
                  <SelectItem value="info">Info</SelectItem>
                  <SelectItem value="warn">Warning</SelectItem>
                  <SelectItem value="error">Error</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">TLS</span>
              <div className="h-11 flex items-center">
                <span className={`inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-xl ${data.tls_enabled ? "bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400" : "bg-secondary text-muted-foreground"}`}>
                  {data.tls_enabled ? <Lock className="w-3 h-3 mr-1" /> : <Unlock className="w-3 h-3 mr-1" />}
                  {data.tls_enabled ? "Enabled" : "Disabled"}
                </span>
              </div>
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">TCP Transport</span>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.tcp_enabled} onChange={(v) => setForm({ ...form, tcp_enabled: v })} />
                <span className="text-sm text-muted-foreground">{form.tcp_enabled ? "Enabled" : "Disabled"}</span>
              </div>
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">TCP Address</span>
              <Input aria-label="0.0.0.0:4444" name="0-0-0-0-4444-3" type="text" placeholder="0.0.0.0:4444" value={form.tcp_addr} onChange={(e) => setForm({ ...form, tcp_addr: e.target.value })} className={`${inputCls} font-mono`} />
            </div>
          </div>
          <div className="border-t border-border pt-4 mt-2">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Offline Threshold (sec)</span>
                <Input aria-label="Offline threshold in seconds" name="input-4" type="number" min={5} max={3600} value={form.offline_threshold} onChange={(e) => setForm({ ...form, offline_threshold: Number(e.target.value) })} className={inputCls} />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Session Timeout (hours)</span>
                <Input aria-label="Session timeout in hours" name="input-5" type="number" min={1} max={720} value={form.session_max_age} onChange={(e) => setForm({ ...form, session_max_age: Number(e.target.value) })} className={inputCls} />
              </div>
              <div>
                <span className="block text-xs text-muted-foreground mb-1.5">Cleanup Retention (days)</span>
                <Input aria-label="Cleanup retention in days" name="input-6" type="number" min={1} max={365} value={form.cleanup_retention} onChange={(e) => setForm({ ...form, cleanup_retention: Number(e.target.value) })} className={inputCls} />
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Button type="submit" disabled={saving} className="h-11 px-6 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <Save className="w-4 h-4" />Save Server Config
            </Button>
            <span className="text-xs text-muted-foreground"><Info className="w-4 h-4" />Some changes require restart</span>
          </div>
        </form>
      </div>
    </Card>
  );
}

