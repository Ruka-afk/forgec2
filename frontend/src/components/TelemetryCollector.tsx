import { useEffect } from "react";
import { onCLS, onFCP, onINP, onLCP, onTTFB, type Metric } from "web-vitals";
import { recordVital, recordClientError, type VitalName } from "@/lib/telemetry";

const VITAL_NAMES = new Set(["TTFB", "FCP", "LCP", "CLS", "FID", "INP"]);

/**
 * Feeds the local telemetry ring buffer from Web Vitals (via the web-vitals
 * core listeners) and uncaught window errors / promise rejections. Nothing
 * leaves the browser.
 *
 * Mounted in AppLayout (authed) AND LoginPage (pre-auth) so login crashes
 * are observable too. Ref-counted: the two never coexist for long, but route
 * transitions and StrictMode double-mount overlap — listeners stay registered
 * exactly while ≥1 instance is mounted, regardless of mount/unmount order.
 */
let collectorRefs = 0;
let collectorHandlers: { onError: (e: ErrorEvent) => void; onRejection: (e: PromiseRejectionEvent) => void } | null = null;

export default function TelemetryCollector() {
  useEffect(() => {
    const report = (metric: Metric) => {
      if (VITAL_NAMES.has(metric.name)) {
        recordVital(metric.name as VitalName, metric.value);
      }
    };
    // web-vitals v5 on* functions register once; no cleanup needed.
    onLCP(report);
    onFCP(report);
    onINP(report);
    onCLS(report);
    onTTFB(report);
  }, []);

  useEffect(() => {
    // Ref-counted: a second instance (StrictMode double-mount, or the
    // login+app overlap during route transition) must not double-register
    // window listeners and double-count every error — and unmount order
    // must not strand the survivor unprotected.
    collectorRefs += 1;
    if (!collectorHandlers) {
      const onError = (e: ErrorEvent) => recordClientError("window.error", e.message || e.filename || "unknown");
      const onRejection = (e: PromiseRejectionEvent) => {
        const reason = e.reason instanceof Error ? e.reason.message : String(e.reason ?? "unknown");
        recordClientError("unhandledrejection", reason);
      };
      window.addEventListener("error", onError);
      window.addEventListener("unhandledrejection", onRejection);
      collectorHandlers = { onError, onRejection };
    }
    return () => {
      collectorRefs -= 1;
      // Any instance's cleanup can release: handlers live at module level
      // so unmount order never strands (or leaks) the registration.
      if (collectorRefs <= 0) {
        collectorRefs = 0;
        if (collectorHandlers) {
          window.removeEventListener("error", collectorHandlers.onError);
          window.removeEventListener("unhandledrejection", collectorHandlers.onRejection);
          collectorHandlers = null;
        }
      }
    };
  }, []);

  return null;
}