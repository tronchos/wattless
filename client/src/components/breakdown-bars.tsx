import { formatBytes, formatPercentage, formatResourceLabel } from "@/lib/api";
import type { ResourceBreakdown } from "@/lib/types";

interface BreakdownBarsProps {
  title: string;
  subtitle: string;
  items: ResourceBreakdown[];
}

export function BreakdownBars({
  title,
  subtitle,
  items,
}: BreakdownBarsProps) {
  return (
    <section className="surface-primary rounded-[1.6rem] p-5">
      <p className="section-kicker">{title}</p>
      <h3 className="editorial-copy mt-3 text-xl font-medium tracking-[-0.05em] text-white">
        {subtitle}
      </h3>

      <div className="mt-5 space-y-4">
        {items.map((item) => (
          <div key={item.label} className="space-y-2">
            <div className="flex items-start justify-between gap-4">
              <div>
                <div className="editorial-copy text-sm font-medium text-white">
                  {formatResourceLabel(item.label)}
                </div>
                <div className="mono mt-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  {item.requests} peticiones
                </div>
              </div>

              <div className="text-right">
                <div className="mono text-sm text-[var(--accent-strong)]">
                  {formatBytes(item.bytes)}
                </div>
                <div className="mono mt-1 text-[11px] uppercase tracking-[0.18em] text-[var(--muted)]">
                  {formatPercentage(item.percentage)}
                </div>
              </div>
            </div>

            <div className="energy-bar">
              <span style={{ width: `${Math.max(item.percentage, 4)}%` }} />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
