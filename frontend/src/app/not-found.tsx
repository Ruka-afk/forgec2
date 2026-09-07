"use client";

import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import { useI18n } from "@/lib/i18n";
import { SystemStatePage } from "@/components/ui/system-state-page";

export default function NotFound() {
  const { t } = useI18n();
  return (
    <SystemStatePage
      code="404"
      title={t("notfound.title")}
      message={t("notfound.message")}
      action={<Button render={<Link to="/dashboard" />}>{t("notfound.back")}</Button>}
    />
  );
}
