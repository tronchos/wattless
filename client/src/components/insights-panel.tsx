import { Sparkles, WandSparkles } from "lucide-react";

import { formatBytes, formatImpactLabel } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface InsightsPanelProps {
  report: ScanReport;
  selectedElementID: string | null;
  onSelectElement: (id: string) => void;
}

export function InsightsPanel({
  report,
  selectedElementID,
  onSelectElement,
}: InsightsPanelProps) {
  return (
    <section className="panel rounded-[2rem] p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            Insights IA
          </p>
          <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
            Qué contar y qué arreglar primero
          </h2>
        </div>
        <span className="mono inline-flex items-center gap-2 rounded-full border border-[var(--line-strong)] px-3 py-1 text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
          <Sparkles className="h-3.5 w-3.5" />
          {report.insights.provider}
        </span>
      </div>

      <div className="mt-6 grid gap-4 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-5">
          <p className="text-lg leading-8 text-white">
            {report.insights.executive_summary}
          </p>
          <p className="mt-4 rounded-[1.25rem] border border-[rgba(155,214,126,0.22)] bg-[rgba(155,214,126,0.08)] p-4 text-sm leading-7 text-[var(--foreground)]">
            {report.insights.pitch_line}
          </p>
        </div>

        <div className="space-y-3">
          {report.insights.top_actions.map((action) => {
            const isActive = selectedElementID === action.related_resource_id;
            return (
              <button
                key={action.id}
                type="button"
                onClick={() => onSelectElement(action.related_resource_id)}
                className={`w-full rounded-[1.4rem] border p-4 text-left transition ${
                  isActive
                    ? "border-[var(--accent)] bg-[rgba(155,214,126,0.1)]"
                    : "border-[var(--line)] bg-[rgba(255,255,255,0.02)] hover:border-[var(--line-strong)]"
                }`}
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="mono inline-flex items-center gap-2 text-xs uppercase tracking-[0.18em] text-[var(--accent)]">
                      <WandSparkles className="h-3.5 w-3.5" />
                      {formatImpactLabel(action.likely_lcp_impact)}
                    </div>
                    <h3 className="mt-3 text-base font-medium text-white">
                      {action.title}
                    </h3>
                  </div>
                  <div className="mono whitespace-nowrap text-sm text-[var(--accent-strong)]">
                    {formatBytes(action.estimated_savings_bytes)}
                  </div>
                </div>
                <p className="mt-3 text-sm leading-6 text-[var(--muted)]">
                  {action.reason}
                </p>
              </button>
            );
          })}
        </div>
      </div>
    </section>
  );
}
