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
  const actions = report.insights.top_actions.slice(0, 3);

  return (
    <section id="insights" className="surface-secondary rounded-[1.45rem] p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="max-w-3xl">
          <div className="flex items-center gap-2">
            <span className="section-kicker">Insights IA</span>
            <span className="soft-chip bg-[rgba(155,214,126,0.08)] text-[var(--accent)]">
              <Sparkles className="h-3.5 w-3.5" />
              {report.insights.provider}
            </span>
          </div>
          <p className="editorial-copy mt-4 text-xl leading-8 text-white">
            {report.insights.executive_summary}
          </p>
        </div>

        <div className="grid gap-2 lg:max-w-[28rem]">
          {actions.map((action, index) => {
            const isActive = selectedElementID === action.related_resource_id;

            return (
              <button
                key={action.id}
                type="button"
                onClick={() => onSelectElement(action.related_resource_id)}
                className={`rounded-[1rem] px-4 py-3 text-left transition ${
                  isActive
                    ? "bg-[rgba(155,214,126,0.08)] shadow-[inset_0_0_0_1px_rgba(155,214,126,0.16)]"
                    : "bg-[rgba(255,255,255,0.02)] hover:bg-[rgba(255,255,255,0.04)]"
                }`}
              >
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="mono inline-flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-[var(--accent)]">
                      <WandSparkles className="h-3.5 w-3.5" />
                      {String(index + 1).padStart(2, "0")} · {formatImpactLabel(action.likely_lcp_impact)}
                    </div>
                    <div className="mt-2 text-sm leading-6 text-white">
                      {action.title}
                    </div>
                  </div>
                  <div className="mono whitespace-nowrap text-sm text-[var(--accent-strong)]">
                    {formatBytes(action.estimated_savings_bytes)}
                  </div>
                </div>

                {isActive && action.recommended_fix && (
                  <div className="mt-5 pt-5 border-t border-[rgba(255,255,255,0.06)] cursor-auto text-left" onClick={(e) => e.stopPropagation()}>
                    <div className="flex flex-wrap gap-2 items-center justify-between mb-4">
                      <span className="mono text-[11px] font-bold uppercase tracking-[0.15em] text-emerald-400">Wattless Optimization</span>
                      <span className="text-[10px] px-2.5 py-1 rounded-full bg-emerald-500/10 text-emerald-300 truncate max-w-[200px]">
                        {action.recommended_fix.expected_impact}
                      </span>
                    </div>
                    <p className="text-sm text-slate-300 mb-4 leading-relaxed">{action.recommended_fix.summary}</p>
                    <pre className="!bg-[#0c1017] p-4 rounded-xl overflow-x-auto text-[13px] leading-relaxed text-slate-300 border border-white/5 shadow-inner mono">
                      <code>{action.recommended_fix.optimized_code}</code>
                    </pre>
                    {action.recommended_fix.changes.length > 0 && (
                      <ul className="mt-5 space-y-2">
                        {action.recommended_fix.changes.map((change, i) => (
                          <li key={i} className="text-xs text-slate-400 flex items-start gap-2 leading-relaxed">
                            <span className="h-1.5 w-1.5 rounded-full bg-[var(--accent)] mt-1.5 shrink-0" /> {change}
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                )}
              </button>
            );
          })}
        </div>
      </div>
    </section>
  );
}
