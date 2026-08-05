"use client";

import { useI18n } from "@/lib/i18n";

export interface AgentHealthRingProps {
  score: number;
  size?: number;
}

function getHealthColor(score: number): string {
  if (score < 30) return "text-red-500";
  if (score < 70) return "text-amber-500";
  return "text-emerald-500";
}

function getHealthRingColor(score: number): string {
  if (score < 30) return "stroke-red-500";
  if (score < 70) return "stroke-amber-500";
  return "stroke-emerald-500";
}

export default function AgentHealthRing({ score, size = 32 }: AgentHealthRingProps) {
  const { t } = useI18n();
  const r = 14;
  const circumference = 2 * Math.PI * r;
  return (
    <div className="relative" style={{ width: size, height: size }}>
      <svg className="w-full h-full -rotate-90" viewBox="0 0 36 36" role="img" aria-label={t("common.health_ring")}>
        <circle cx="18" cy="18" r={r} fill="none" stroke="currentColor" strokeWidth="3"
          className="text-border" />
        <circle cx="18" cy="18" r={r} fill="none" strokeWidth="3" strokeLinecap="round"
          className={getHealthRingColor(score)}
          strokeDasharray={`${(score / 100) * circumference} ${circumference}`} />
      </svg>
    </div>
  );
}

export { getHealthColor, getHealthRingColor };
