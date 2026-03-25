import type { LucideIcon } from "lucide-react";

interface MetricCardProps {
  label: string;
  value: string;
  hint: string;
  icon?: LucideIcon;
}

export function MetricCard({ label, value, hint, icon: Icon }: MetricCardProps) {
  return (
    <article className="panel rounded-[1.75rem] p-5">
      <div className="flex items-start justify-between gap-3">
        <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
          {label}
        </p>
        {Icon ? <Icon className="h-4 w-4 text-[var(--accent)]" /> : null}
      </div>
      <p className="mt-4 text-3xl font-medium tracking-[-0.05em] text-white">
        {value}
      </p>
      <p className="mt-3 text-sm leading-6 text-[var(--muted)]">{hint}</p>
    </article>
  );
}
