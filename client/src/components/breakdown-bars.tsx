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
    <section className="panel rounded-[2rem] p-6">
      <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
        {title}
      </p>
      <h3 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
        {subtitle}
      </h3>

      <div className="mt-6 space-y-4">
        {items.map((item) => (
          <div key={item.label} className="space-y-2">
            <div className="flex items-center justify-between gap-4">
              <div>
                <div className="text-sm font-medium text-white">
                  {formatResourceLabel(item.label)}
                </div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                  {item.requests} peticiones
                </div>
              </div>
              <div className="text-right">
                <div className="mono text-sm text-[var(--accent-strong)]">
                  {formatBytes(item.bytes)}
                </div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                  {formatPercentage(item.percentage)}
                </div>
              </div>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-[rgba(255,255,255,0.06)]">
              <div
                className="h-full rounded-full bg-[linear-gradient(90deg,#9bd67e,#d8ff7f)]"
                style={{ width: `${Math.max(item.percentage, 4)}%` }}
              />
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
