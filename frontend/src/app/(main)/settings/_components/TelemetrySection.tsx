"use client";

import { useEffect, useState } from "react";
import { useI18n } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { EmptyState } from "@/components/ui/empty-state";
import { Activity, Eraser } from "lucide-react";
import {
  getTelemetryEntries,
  clearTelemetry,
  subscribeTelemetry,
  type TelemetryEntry,
} from "@/lib/telemetry";

function formatValue(entry: TelemetryEntry): string {
  if (entry.kind === "vital") {
    if (entry.name === "CLS") return entry.value.toFixed(3);
    return `${Math.round(entry.value)} ms`;
  }
  return entry.message;
}

export default function TelemetrySection() {
  const { t } = useI18n();
  const [entries, setEntries] = useState<readonly TelemetryEntry[]>(() => getTelemetryEntries());

  useEffect(() => subscribeTelemetry(() => setEntries([...getTelemetryEntries()])), []);

  return (
    <div className="space-y-4">
      <p className="text-sm text-muted-foreground">{t("settings.telemetry_desc")}</p>
      <Card>
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <CardTitle className="flex items-center gap-2">
            <Activity className="w-4 h-4 text-primary" />
            {t("settings.telemetry_title")}
          </CardTitle>
          <Button
            size="sm"
            variant="outline"
            onClick={clearTelemetry}
            disabled={entries.length === 0}
            className="gap-1.5"
          >
            <Eraser className="w-3.5 h-3.5" />
            {t("settings.telemetry_clear")}
          </Button>
        </CardHeader>
        <CardContent>
          {entries.length === 0 ? (
            <EmptyState
              icon={Activity}
              title={t("settings.telemetry_empty")}
              message={t("settings.telemetry_empty_desc")}
            />
          ) : (
            <div className="max-h-80 overflow-y-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead className="w-24">{t("settings.telemetry_col_time")}</TableHead>
                    <TableHead className="w-28">{t("settings.telemetry_col_kind")}</TableHead>
                    <TableHead>{t("settings.telemetry_col_name")}</TableHead>
                    <TableHead className="text-right">{t("settings.telemetry_col_value")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {[...entries].reverse().map((entry, idx) => (
                    <TableRow key={`${entry.ts}-${idx}`}>
                      <TableCell className="py-2 font-mono text-(--fs-micro)">
                        {new Date(entry.ts).toLocaleTimeString()}
                      </TableCell>
                      <TableCell className="py-2">
                        <span
                          className={`rounded px-1.5 py-px text-(--fs-micro) font-medium ${
                            entry.kind === "error"
                              ? "bg-destructive/15 text-destructive"
                              : "bg-success/15 text-success"
                          }`}
                        >
                          {entry.kind === "error"
                            ? t("settings.telemetry_kind_error")
                            : t("settings.telemetry_kind_vital")}
                        </span>
                      </TableCell>
                      <TableCell className="py-2 text-(--fs-compact)">
                        {entry.kind === "error" ? entry.source : entry.name}
                      </TableCell>
                      <TableCell className="py-2 text-right font-mono text-(--fs-micro)">
                        {formatValue(entry)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}