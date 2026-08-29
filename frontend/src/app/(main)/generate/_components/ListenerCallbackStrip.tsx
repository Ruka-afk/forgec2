"use client";

import Link from "next/link";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { CopyButton } from "@/components/ui/copy-button";
import { Radio } from "lucide-react";
import type { Listener } from "@/types/generate";

export function ListenerCallbackStrip({
  listener,
  callback,
  onCreate,
}: {
  listener?: Listener;
  callback: string;
  onCreate: () => void;
}) {
  const { t } = useI18n();
  const id = listener?.id != null ? String(listener.id) : "";
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2 rounded-lg border border-border bg-card px-3 py-2">
      <Radio className="size-4 shrink-0 text-primary" />
      {listener && callback ? (
        <>
          <span className="text-sm font-medium">{listener.name || t("generate.unknown_listener")}</span>
          <code className="min-w-0 flex-1 truncate font-mono text-xs text-muted-foreground">{callback}</code>
          <CopyButton text={callback} label={t("generate.c2_url_auto")} size="icon-xs" />
          {id && (
            <Button variant="ghost" size="xs" render={<Link href={`/listeners/${id}`} />}>
              {t("generate.strip_open_listener")}
            </Button>
          )}
        </>
      ) : (
        <>
          <span className="flex-1 text-sm text-warning-foreground">{t("generate.toast.select_listener_first")}</span>
          <Button size="xs" onClick={onCreate}>{t("generate.create_listener")}</Button>
        </>
      )}
    </div>
  );
}
