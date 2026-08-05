import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";

interface ListenerForm {
  name: string;
  ltype: string;
  host: string;
  port: string;
  proto: string;
}

export default function ListenerModal({
  show,
  form,
  onChange,
  onSubmit,
  onClose,
}: {
  show: boolean;
  form: ListenerForm;
  onChange: (f: ListenerForm) => void;
  onSubmit: () => void;
  onClose: () => void;
}) {
  const { t } = useI18n();
  useEffect(() => {
    if (!show) {
      onChange({ name: "", ltype: "http", host: "", port: "8080", proto: "http" });
    }
  }, [show, onChange]);

  return (
    <Dialog open={show} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("generate.listener_new")}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_name_aria")}</span>
            <Input aria-label={t("generate.listener_name_aria")} name="input-0" autoFocus value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} />
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_type")}</span>
            <Select value={form.ltype} onValueChange={(val) => val != null && onChange({ ...form, ltype: val })}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="http">http</SelectItem>
                <SelectItem value="tcp">tcp</SelectItem>
                <SelectItem value="dns">dns</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_domain")}</span>
            <Input aria-label={t("generate.listener_domain_aria")} name="input-2" value={form.host} onChange={(e) => onChange({ ...form, host: e.target.value })} />
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_port")}</span>
            <Input aria-label={t("generate.listener_port_aria")} name="input-3" type="number" min="1" max="65535" value={form.port} onChange={(e) => onChange({ ...form, port: e.target.value })} />
          </div>
          {form.ltype !== "dns" && (
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_protocol")}</span>
              <Input aria-label={t("generate.listener_protocol_aria")} name="input-4" value={form.proto} onChange={(e) => onChange({ ...form, proto: e.target.value })} placeholder="http/https/tcp/tls" />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button type="button" onClick={onSubmit}>{t("common.confirm")}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
