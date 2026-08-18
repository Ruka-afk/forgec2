"use client";

import { useEffect } from "react";
import { useReportWebVitals } from "next/web-vitals";
import { recordVital, recordClientError, type VitalName } from "@/lib/telemetry";

const VITAL_NAMES = new Set(["TTFB", "FCP", "LCP", "CLS", "FID", "INP"]);

/**
 * Mounted once in AppLayout. Feeds the local telemetry ring buffer from
 * Web Vitals (via Next's built-in useReportWebVitals) and uncaught
 * window errors / promise rejections. Nothing leaves the browser.
 */
export default function TelemetryCollector() {
  useReportWebVitals((metric) => {
    if (VITAL_NAMES.has(metric.name)) {
      recordVital(metric.name as VitalName, metric.value);
    }
  });

  useEffect(() => {
    const onError = (e: ErrorEvent) => recordClientError("window.error", e.message || e.filename || "unknown");
    const onRejection = (e: PromiseRejectionEvent) => {
      const reason = e.reason instanceof Error ? e.reason.message : String(e.reason ?? "unknown");
      recordClientError("unhandledrejection", reason);
    };
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onRejection);
    return () => {
      window.removeEventListener("error", onError);
      window.removeEventListener("unhandledrejection", onRejection);
    };
  }, []);

  return null;
}