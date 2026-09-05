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
  const connected = Boolean(listener && callback);
  return (
    <div className="mb-4 flex flex-wrap items-center gap-2.5 rounded-xl border border-border/60 bg-gradient-to-r from-card via-card to-muted/30 px-3.5 py-2.5 shadow-sm">
      <div className="grid size-8 shrink-0 place-items-center rounded-lg bg-primary/10 text-primary ring-1 ring-primary/15">
        <Radio className="size-4" />
      </div>
      {connected ? (
        <>
          <span className="inline-flex items-center gap-1.5 text-sm font-semibold text-foreground">
            <span className="size-1.5 rounded-full bg-success shadow-sm" aria-hidden="true" />
            {listener?.name || t("generate.unknown_listener")}
          </span>
          <code className="min-w-0 flex-1 truncate rounded-md bg-muted/60 px-2 py-1 font-mono text-xs text-muted-foreground ring-1 ring-border/40">{callback}</code>
          <CopyButton text={callback} label={t("generate.c2_url_auto")} size="icon-xs" />
          {id && (
            <Button variant="outline" size="xs" render={<Link href={`/listeners/${id}`} />} className="rounded-full shadow-sm">
              {t("generate.strip_open_listener")}
            </Button>
          )}
        </>
      ) : (
        <>
          <span className="inline-flex flex-1 items-center gap-1.5 text-sm font-medium text-warning-foreground">
            <span className="size-1.5 animate-pulse rounded-full bg-warning" aria-hidden="true" />
            {t("generate.toast.select_listener_first")}
          </span>
          <Button size="xs" onClick={onCreate} className="rounded-full shadow-sm">{t("generate.create_listener")}</Button>
        </>
      )}
    </div>
  );
}
