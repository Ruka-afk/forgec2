import { useI18n } from "@/lib/i18n";
import { cn, formatTime } from "@/lib/utils";
import { toneStyles, toneForStatus } from "@/lib/ui/statusStyles";
import type { HealthSample } from "@/lib/listener-health";

interface HealthSparklineProps {
  samples: HealthSample[];
  className?: string;
  width?: number;
  height?: number;
}

/**
 * Tiny bar sparkline of polled circuit-breaker statuses.
 * Uses design-system tone classes (no raw hex) so dark mode tracks the theme.
 */
export function HealthSparkline({ samples, className, width = 72, height = 16 }: HealthSparklineProps) {
  const { t } = useI18n();
  if (samples.length < 2) return null;

  const n = samples.length;
  const gap = 1;
  const barW = Math.max(2, Math.floor((width - gap * (n - 1)) / n));
  const usable = barW * n + gap * (n - 1);
  const offsetX = Math.max(0, Math.floor((width - usable) / 2));

  const tip = samples
    .slice(-8)
    .map((s) => `${formatTime(new Date(s.t).toISOString())} · ${s.status}`)
    .join("\n");

  return (
    <span
      role="img"
      aria-label={t("listeners.health_trend")}
      title={`${t("listeners.health_trend")}\n${tip}`}
      data-testid="health-sparkline"
      className={cn("inline-flex shrink-0 items-end", className)}
    >
      <svg width={width} height={height} viewBox={`0 0 ${width} ${height}`} aria-hidden="true" focusable="false">
        {samples.map((s, i) => {
          const cfg = toneStyles[toneForStatus(s.status)];
          const x = offsetX + i * (barW + gap);
          const h = Math.max(4, Math.round(height * (s.status === "burned" ? 1 : s.status === "unstable" ? 0.7 : 0.45)));
          return (
            <rect
              key={`${s.t}-${i}`}
              x={x}
              y={height - h}
              width={barW}
              height={h}
              rx={1}
              className={cfg.dot}
            />
          );
        })}
      </svg>
    </span>
  );
}
