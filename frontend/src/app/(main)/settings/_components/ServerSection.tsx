import { SettingsData, ServerForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import Toggle from "./Toggle";
import { Info, Lock, Save, Server, Unlock } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function ServerSection({
  data, form, setForm, saving, onSave,
}: {
  data: SettingsData;
  form: ServerForm;
  setForm: React.Dispatch<React.SetStateAction<ServerForm>>;
  saving: boolean;
  onSave: (e: React.FormEvent) => void;
}) {
  const { t } = useI18n();
  return (
    <Card className="overflow-hidden">
      <div className="bg-primary/10 border-b border-primary/20 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Server className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-foreground">{t("settings.server.title")}</h2><p className="text-xs text-muted-foreground">{t("settings.server.subtitle")}</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <Label htmlFor="server-http-port" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.http_port")}</Label>
              <Input id="server-http-port" type="number" defaultValue={data.server_port} readOnly className="bg-muted cursor-not-allowed text-muted-foreground" />
            </div>
            <div>
              <Label htmlFor="server-listen-address" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.listen_address")}</Label>
              <Input id="server-listen-address" type="text" defaultValue={data.server_address ?? ""} readOnly className="bg-muted cursor-not-allowed text-muted-foreground font-mono" />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.log_level")}</span>
              <Select value={form.log_level} onValueChange={(v) => v && setForm({ ...form, log_level: v })}>
                <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="debug">{t("settings.server.log_level_debug")}</SelectItem>
                  <SelectItem value="info">{t("settings.server.log_level_info")}</SelectItem>
                  <SelectItem value="warn">{t("settings.server.log_level_warn")}</SelectItem>
                  <SelectItem value="error">{t("settings.server.log_level_error")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.tls")}</span>
              <div className="h-11 flex items-center">
                <span className={`inline-flex items-center px-3 py-1.5 text-xs font-medium rounded-xl ${data.tls_enabled ? "bg-emerald-100 dark:bg-emerald-900/30 text-emerald-700 dark:text-emerald-400" : "bg-secondary text-muted-foreground"}`}>
                  {data.tls_enabled ? <Lock className="w-3 h-3 mr-1" /> : <Unlock className="w-3 h-3 mr-1" />}
                  {data.tls_enabled ? t("settings.server.enabled") : t("settings.server.disabled")}
                </span>
              </div>
            </div>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.tcp_transport")}</span>
              <div className="h-11 flex items-center gap-3">
                <Toggle checked={form.tcp_enabled} onChange={(v) => setForm({ ...form, tcp_enabled: v })} />
                <span className="text-sm text-muted-foreground">{form.tcp_enabled ? t("settings.server.enabled") : t("settings.server.disabled")}</span>
              </div>
            </div>
            <div>
              <Label htmlFor="server-tcp-address" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.tcp_address")}</Label>
              <Input id="server-tcp-address" type="text" placeholder="0.0.0.0:4444" value={form.tcp_addr} onChange={(e) => setForm({ ...form, tcp_addr: e.target.value })} className="font-mono" />
            </div>
          </div>
          <div className="border-t border-border pt-4 mt-2">
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div>
                <Label htmlFor="server-offline-threshold" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.offline_threshold")}</Label>
                <Input id="server-offline-threshold" type="number" min={5} max={3600} value={form.offline_threshold} onChange={(e) => setForm({ ...form, offline_threshold: Number(e.target.value) })} />
              </div>
              <div>
                <Label htmlFor="server-session-timeout" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.session_timeout")}</Label>
                <Input id="server-session-timeout" type="number" min={1} max={720} value={form.session_max_age} onChange={(e) => setForm({ ...form, session_max_age: Number(e.target.value) })} />
              </div>
              <div>
                <Label htmlFor="server-cleanup-retention" className="block text-xs text-muted-foreground mb-1.5">{t("settings.server.cleanup_retention")}</Label>
                <Input id="server-cleanup-retention" type="number" min={1} max={365} value={form.cleanup_retention} onChange={(e) => setForm({ ...form, cleanup_retention: Number(e.target.value) })} />
              </div>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <Button type="submit" disabled={saving} className="h-11 px-6 rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
              <Save className="w-4 h-4" />{t("settings.server.save")}
            </Button>
            <span className="text-xs text-muted-foreground"><Info className="w-4 h-4" />{t("settings.server.restart_hint")}</span>
          </div>
        </form>
      </div>
    </Card>
  );
}
