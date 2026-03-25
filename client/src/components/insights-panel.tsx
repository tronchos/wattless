import { Sparkles, WandSparkles } from "lucide-react";

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
  const activeAction =
    report.insights.top_actions.find(
      (a) => selectedElementID && a.related_resource_ids.includes(selectedElementID)
    ) || report.insights.top_actions[0];

  return (
    <section id="insights" className="surface-secondary rounded-[1.45rem] p-6 lg:p-8">
      <div className="flex flex-col gap-6">
        <div>
          <div className="flex items-center gap-2">
            <span className="section-kicker">Síntesis editorial</span>
            <span className="soft-chip bg-[rgba(155,214,126,0.08)] text-[var(--accent)]">
              <Sparkles className="h-3.5 w-3.5" />
              {report.insights.provider}
            </span>
          </div>
          <p className="editorial-copy mt-4 text-xl lg:text-2xl leading-relaxed text-white">
            {report.insights.executive_summary}
          </p>
        </div>

        {activeAction?.recommended_fix && (
          <div className="mt-2 lg:mt-6 pt-6 lg:pt-8 border-t border-[rgba(255,255,255,0.06)]">
            <div className="flex flex-wrap gap-4 items-center justify-between mb-5">
              <div className="flex items-center gap-2.5">
                <div className="h-6 w-6 rounded-full bg-emerald-500/10 flex items-center justify-center border border-emerald-500/20">
                  <WandSparkles className="h-3 w-3 text-emerald-400" />
                </div>
                <span className="mono text-[11px] font-bold uppercase tracking-[0.2em] text-emerald-400">
                  Wattless Optimization
                </span>
              </div>
              <span className="text-[10px] px-3 py-1.5 rounded-full bg-emerald-500/10 text-emerald-300 font-bold tracking-wide uppercase">
                {activeAction.recommended_fix.expected_impact}
              </span>
            </div>
            
            <p className="text-[15px] text-slate-300 mb-5 leading-relaxed">
              {activeAction.recommended_fix.summary}
            </p>
            
            <pre className="!bg-[#0c1017] p-5 rounded-2xl overflow-x-auto text-[13px] leading-relaxed text-slate-200 border border-white/5 shadow-inner mono">
              <code>{activeAction.recommended_fix.optimized_code}</code>
            </pre>
            
            {activeAction.recommended_fix.changes.length > 0 && (
              <ul className="mt-6 space-y-3">
                {activeAction.recommended_fix.changes.map((change, i) => (
                  <li key={i} className="text-sm text-slate-400 flex items-start gap-3 leading-relaxed">
                    <span className="h-1.5 w-1.5 rounded-full bg-[var(--accent)] mt-2 shrink-0 opacity-80" /> 
                    {change}
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}

        {report.insights.top_actions.length > 0 ? (
          <div className="mt-2 border-t border-[rgba(255,255,255,0.06)] pt-6">
            <div className="section-kicker">Top Actions</div>
            <div className="mt-4 space-y-3">
              {report.insights.top_actions.map((action) => {
                const isActive = selectedElementID
                  ? action.related_resource_ids.includes(selectedElementID)
                  : action.id === activeAction?.id;

                return (
                  <button
                    key={action.id}
                    type="button"
                    onClick={() => {
                      const nextID = action.related_resource_ids[0];
                      if (nextID) {
                        onSelectElement(nextID);
                      }
                    }}
                    className={`w-full rounded-2xl px-4 py-4 text-left transition-colors ${
                      isActive
                        ? "bg-[rgba(155,214,126,0.08)]"
                        : "bg-[rgba(255,255,255,0.03)] hover:bg-[rgba(255,255,255,0.05)]"
                    }`}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="soft-chip bg-[rgba(255,255,255,0.05)]">
                        {action.confidence}
                      </span>
                      <span className="soft-chip bg-[rgba(255,255,255,0.05)]">
                        {action.likely_lcp_impact}
                      </span>
                    </div>
                    <div className="mt-3 text-base font-headline font-bold text-white">
                      {action.title}
                    </div>
                    <p className="mt-2 text-sm leading-6 text-slate-300">
                      {action.reason}
                    </p>
                  </button>
                );
              })}
            </div>
          </div>
        ) : null}
      </div>
    </section>
  );
}
