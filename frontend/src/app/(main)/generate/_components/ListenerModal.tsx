import { useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";

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
  useEffect(() => {
    if (!show) {
      onChange({ name: "", ltype: "http", host: "", port: "8080", proto: "http" });
    }
  }, [show, onChange]);

  return (
    <Dialog open={show} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New Listener</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">Name</span>
            <Input aria-label="Listener name" name="input-0" autoFocus value={form.name} onChange={(e) => onChange({ ...form, name: e.target.value })} />
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">Type</span>
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
            <span className="block text-xs font-semibold text-muted-foreground mb-1">Domain/IP</span>
            <Input aria-label="Listener domain or IP address" name="input-2" value={form.host} onChange={(e) => onChange({ ...form, host: e.target.value })} />
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">Port</span>
            <Input aria-label="Listener port number" name="input-3" type="number" min="1" max="65535" value={form.port} onChange={(e) => onChange({ ...form, port: e.target.value })} />
          </div>
          {form.ltype !== "dns" && (
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1">Protocol</span>
              <Input aria-label="Protocol (http/https/tcp/tls)" name="input-4" value={form.proto} onChange={(e) => onChange({ ...form, proto: e.target.value })} placeholder="http/https/tcp/tls" />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
          <Button type="button" onClick={onSubmit}>Confirm</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
