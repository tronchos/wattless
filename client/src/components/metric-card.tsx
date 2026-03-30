import type { LucideIcon } from "lucide-react";

interface MetricCardProps {
  label: string;
  value: string;
  caption: string;
  hint: string;
  progress?: number;
  icon?: LucideIcon;
}

export function MetricCard({
  label,
  value,
  caption,
  hint,
  progress,
  icon: Icon,
}: MetricCardProps) {
  return (
    <article className="bg-surface-container-low p-6 lg:p-8 rounded-3xl flex flex-col gap-2 group hover:bg-surface-container transition-colors h-full w-full border border-outline-variant/5 hover:border-outline-variant/20">
      <span className="text-on-surface-variant text-xs uppercase tracking-widest font-label flex items-center gap-2">
        {Icon ? <Icon className="h-4 w-4" /> : null} {label}
      </span>
      <div className="text-5xl font-headline font-bold text-on-surface mt-2">
        {value}
      </div>
      <p className="text-sm text-on-surface-variant mt-2 italic">{caption}</p>

      {typeof progress === "number" ? (
        <div className="w-full h-1.5 bg-surface-container-highest rounded-full mt-4 overflow-hidden">
          <div
            className="h-full bg-primary"
            style={{ width: `${Math.max(progress, 6)}%` }}
          />
        </div>
      ) : null}

      <p className="mt-4 text-sm leading-7 text-on-surface-variant/80">{hint}</p>
    </article>
  );
}
