
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { ShieldAlert } from "lucide-react";
import { SystemStatePage } from "@/components/ui/system-state-page";

export default function Forbidden() {
  const { t } = useI18n();
  return (
    <SystemStatePage
      code="403"
      tone="destructive"
      icon={<ShieldAlert className="size-7" aria-hidden="true" />}
      title={t("forbidden.title")}
      message={t("forbidden.message")}
      action={<Button render={<Link to="/dashboard" />}>{t("forbidden.back")}</Button>}
    />
  );
}
