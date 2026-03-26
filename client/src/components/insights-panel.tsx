import { Sparkles } from "lucide-react";

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
  const resolveActionTargetID = (action: ScanReport["insights"]["top_actions"][number]) => {
    const matchedByAssetInsight = report.vampire_elements.find(
      (element) => element.asset_insight.related_action_id === action.id,
    );
    if (matchedByAssetInsight) {
      return matchedByAssetInsight.id;
    }

    const matching = report.vampire_elements.find((element) =>
      action.related_resource_ids.includes(element.id)
    );
    return matching?.id ?? null;
  };

  const compactActions = report.insights.top_actions.slice(0, 2);
  const selectedElement = report.vampire_elements.find(
    (element) => element.id === selectedElementID,
  );
  const activeAction =
    compactActions.find(
      (action) =>
        selectedElement &&
        (selectedElement.asset_insight.related_action_id === action.id ||
          action.related_resource_ids.includes(selectedElement.id)),
    ) ??
    compactActions[0] ??
    null;

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
          {report.insights.pitch_line ? (
            <p className="mt-4 text-sm leading-6 text-slate-300">
              {report.insights.pitch_line}
            </p>
          ) : null}
        </div>

        {compactActions.length > 0 ? (
          <div className="mt-2 border-t border-[rgba(255,255,255,0.06)] pt-6">
            <div className="section-kicker">Top Actions</div>
            <div className="mt-4 space-y-3">
              {compactActions.map((action) => {
                const isActive = selectedElementID
                  ? selectedElement?.asset_insight.related_action_id === action.id ||
                    action.related_resource_ids.includes(selectedElementID)
                  : action.id === activeAction?.id;

                return (
                  <button
                    key={action.id}
                    type="button"
                    onClick={() => {
                      const nextID = resolveActionTargetID(action);
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
                    {action.evidence[0] ? (
                      <p className="mt-2 text-xs leading-5 text-slate-400">
                        {action.evidence[0]}
                      </p>
                    ) : null}
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
