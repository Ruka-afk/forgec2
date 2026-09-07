import { Switch } from "@/components/ui/switch";

export default function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return <Switch checked={checked} onCheckedChange={onChange} />;
}
