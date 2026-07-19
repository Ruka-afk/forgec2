import type { ReactNode } from "react";

import { Listener, OneLinerForm, OneLinerType, OneLinerData } from "./types";
import { Spinner } from "@/components/UI";
import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { CheckCircle, Copy, Terminal, Zap } from "lucide-react";

export default function OneLinerPanel({
  form, setForm, busy, result, onelinerData, listeners, getListenerInfo, onGenerate, onCopy,
}: {
  form: OneLinerForm;
  setForm: React.Dispatch<React.SetStateAction<OneLinerForm>>;
  busy: boolean;
  result: ReactNode;
  onelinerData?: OneLinerData;
  listeners: Listener[];
  getListenerInfo: (id: string) => { scheme: string; host: string; port: string | number; type: string; name: string } | null;
  onGenerate: () => void;
  onCopy: (text: string) => void;
}) {
  return (
    <div className="mt-8">
      <div className="flex items-center gap-x-3 mb-5">
        <div className="w-10 h-10 bg-rose-100 dark:bg-rose-900/30 rounded-xl flex items-center justify-center">
          <Terminal className="w-4 h-4" />
        </div>
        <div>
          <div className="text-sm font-semibold text-foreground">One-Liner</div>
          <div className="text-xs text-muted-foreground">Generate 10+ one-liner commands with remote hosting</div>
        </div>
      </div>
      <Card className="p-4 sm:p-5 hover:shadow-lg dark:hover:shadow-black/30 transition-shadow">
        <div className="space-y-4">
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Payload Type</span>
              <Select value={form.payload_type} onValueChange={(val) => val != null && setForm({ ...form, payload_type: val })}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="exe">Windows EXE</SelectItem>
                  <SelectItem value="ps1">PowerShell</SelectItem>
                  <SelectItem value="linux">Linux ELF</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Listener</span>
              <Select value={form.listener_id} onValueChange={(val) => {
                if (val == null) return;
                const info = getListenerInfo(val);
                let c2url = "";
                let protocol = "http";
                if (info) {
                  c2url = `${info.scheme}://${info.host}:${info.port}`;
                  if (info.scheme === "tcp" || info.scheme === "tls") protocol = "tcp";
                  else if (info.scheme === "dns" || info.type === "dns") protocol = "dns";
                }
                setForm({ ...form, listener_id: val, c2_url: c2url, protocol });
              }}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder="-- Select --" />
                </SelectTrigger>
                <SelectContent>
                  {listeners.map((l, i) => {
                    const id = String(l.id ?? i);
                    const name = l.name || "Unknown";
                    const scheme = l.scheme || l.type || "http";
                    const host = l.host || "";
                    const port = l.port || "";
                    return <SelectItem key={id} value={id}>{name} ({scheme}://{host}:{port})</SelectItem>;
                  })}
                </SelectContent>
              </Select>
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Heartbeat (sec)</span>
              <Input aria-label="Heartbeat interval in seconds" name="input-2" type="number" value={form.beacon_time} onChange={(e) => setForm({ ...form, beacon_time: e.target.value })} />
            </div>
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1.5">Jitter (%)</span>
              <Input aria-label="Jitter percentage" name="input-3" type="number" value={form.jitter} onChange={(e) => setForm({ ...form, jitter: e.target.value })} />
            </div>
          </div>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="flex items-center gap-x-2">
              <Checkbox aria-label="Skip TLS verification" checked={form.skip_tls} onCheckedChange={(v) => setForm({ ...form, skip_tls: v === true })} id="skip-tls-oneliner" />
              <Label htmlFor="skip-tls-oneliner" className="text-sm text-foreground">Skip TLS</Label>
            </div>
            <div className="flex items-center gap-x-2">
              <Checkbox aria-label="Enable persistence" checked={form.persist} onCheckedChange={(v) => setForm({ ...form, persist: v === true })} id="persist-oneliner" />
              <Label htmlFor="persist-oneliner" className="text-sm text-muted-foreground">Persist</Label>
            </div>
          </div>
          <Button type="button" onClick={onGenerate} disabled={busy} className="w-full h-10 bg-rose-600 hover:bg-rose-700 disabled:opacity-50 transition-colors text-white font-medium rounded-xl flex items-center justify-center gap-x-2">
            {busy ? <><Spinner size="xs" /> Generating...</> : <><Zap className="w-4 h-4" /> Generate One-Liners</>}
          </Button>
        </div>
        {result === "success" && onelinerData ? (
          <div className="mt-4">
            <div className="text-xs text-emerald-600 mb-3 flex items-center gap-x-2">
              <CheckCircle className="w-4 h-4" />
              Download URL <code className="text-xs bg-muted px-2 py-0.5 rounded">{onelinerData.download_url}</code> (valid 1hr)
            </div>
            <div className="space-y-2">
              {onelinerData.types.map((item: OneLinerType, idx: number) => (
                <div key={idx} className="border border-border rounded-xl p-3 hover:border-rose-200 transition-colors">
                  <div className="flex items-center justify-between mb-1.5">
                    <div>
                      <span className="text-sm font-medium text-foreground">{item.name}</span>
                      <span className="text-[10px] text-muted-foreground ml-2">{item.desc}</span>
                    </div>
                    <Button variant="outline" size="xs" onClick={() => onCopy(item.command)}>
                      <Copy className="w-4 h-4" />Copy
                    </Button>
                  </div>
                  <code className="block text-[11px] font-mono bg-muted text-foreground p-2 rounded-xl whitespace-pre-wrap break-all leading-relaxed select-all">{item.command}</code>
                </div>
              ))}
            </div>
          </div>
        ) : result ? (
          <div className="mt-4">{result}</div>
        ) : null}
      </Card>
    </div>
  );
}

