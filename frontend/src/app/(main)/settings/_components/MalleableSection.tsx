import { MalleableForm } from "./types";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import Toggle from "./Toggle";
import { AlertTriangle, Save, Shield } from "lucide-react";

export default function MalleableSection({
  form, setForm, saving, onSave,
}: {
  form: MalleableForm;
  setForm: React.Dispatch<React.SetStateAction<MalleableForm>>;
  saving: boolean;
  onSave: (e: React.FormEvent) => void;
}) {
  return (
    <Card className="overflow-hidden">
      <div className="bg-gradient-to-r from-violet-600 to-violet-800 px-6 py-4">
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 bg-secondary/50 rounded-xl flex items-center justify-center"><Shield className="w-4 h-4" /></div>
          <div><h2 className="text-lg font-semibold text-white">Malleable C2 Profile</h2><p className="text-xs text-violet-200">Customize beacon traffic characteristics</p></div>
        </div>
      </div>
      <div className="p-4 sm:p-5">
        <form onSubmit={onSave} className="space-y-4">
          <div className="flex items-center gap-3">
            <Toggle checked={form.enabled} onChange={(v) => setForm({ ...form, enabled: v })} />
            <span className="text-sm text-muted-foreground dark:text-muted-foreground">{form.enabled ? "Enabled" : "Disabled"}</span>
            <span className="text-xs text-muted-foreground">Override default JSON response format</span>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">HTTP Status Code</span>
              <Input aria-label="HTTP status code" name="input-0" type="number" min={100} max={599} value={form.status_code} onChange={(e) => setForm({ ...form, status_code: Number(e.target.value) })}  />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Content-Type</span>
              <Input aria-label="application/json" name="application-json-1" type="text" placeholder="application/json" value={form.content_type} onChange={(e) => setForm({ ...form, content_type: e.target.value })} className="font-mono" />
            </div>
          </div>
          <div>
            <span className="block text-xs text-muted-foreground mb-1.5">Custom Headers (one per line)</span>
            <Textarea rows={3} value={form.headers_text} onChange={(e) => setForm({ ...form, headers_text: e.target.value })} placeholder={"Server: nginx/1.24.0\nX-Powered-By: ASP.NET"}  />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Prepend Content</span>
              <Textarea rows={2} value={form.prepend} onChange={(e) => setForm({ ...form, prepend: e.target.value })} placeholder="<html><body><!--"  />
            </div>
            <div>
              <span className="block text-xs text-muted-foreground mb-1.5">Append Content</span>
              <Textarea rows={2} value={form.append} onChange={(e) => setForm({ ...form, append: e.target.value })} placeholder="--></body></html>"  />
            </div>
          </div>
          <div className="p-3 bg-warning/10 rounded-xl border border-warning/20 text-xs text-warning-foreground">
            <AlertTriangle className="w-4 h-4" />
            Enabling profile requires compatible agents. Prepend/append is for traffic camouflage only.
          </div>
          <Button type="submit" disabled={saving} className="h-11 px-6 bg-violet-600 hover:bg-violet-700 text-white rounded-xl text-sm font-medium transition-colors disabled:opacity-50">
            <Save className="w-4 h-4" />Save Profile
          </Button>
        </form>
      </div>
    </Card>
  );
}
