import { Sparkles } from "lucide-react";

import type { InsightsStatus, ScanReport } from "@/lib/types";

interface InsightsPanelProps {
  report: ScanReport;
  insightsStatus: InsightsStatus;
  selectedElementID: string | null;
  onSelectElement: (id: string) => void;
}

export function InsightsPanel({
  report,
  insightsStatus,
  selectedElementID,
  onSelectElement,
}: InsightsPanelProps) {
  const getVisibleRelatedResourceIDs = (
    action: ScanReport["insights"]["top_actions"][number],
  ) =>
    Array.isArray(action.visible_related_resource_ids)
      ? action.visible_related_resource_ids
      : [];

  const getActionEvidence = (
    action: ScanReport["insights"]["top_actions"][number],
  ) => (Array.isArray(action.evidence) ? action.evidence : []);

  const isAnchoredAction = (action: ScanReport["insights"]["top_actions"][number]) =>
    getVisibleRelatedResourceIDs(action).length > 0;

  const resolveActionTargetID = (action: ScanReport["insights"]["top_actions"][number]) => {
    if (!isAnchoredAction(action)) {
      return null;
    }
    const matchedByAssetInsight = report.vampire_elements.find(
      (element) => element.asset_insight.related_action_id === action.id,
    );
    if (matchedByAssetInsight) {
      return matchedByAssetInsight.id;
    }

    const matching = report.vampire_elements.find((element) =>
      getVisibleRelatedResourceIDs(action).includes(element.id)
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
          getVisibleRelatedResourceIDs(action).includes(selectedElement.id)),
    ) ??
    null;

  const fallbackActiveAction =
    !selectedElementID
      ? compactActions.find((action) => isAnchoredAction(action)) ?? null
      : null;

  const resolvedActiveAction = activeAction ?? fallbackActiveAction;
  const isProcessing = insightsStatus === "processing";

  return (
    <section id="insights" className="bg-surface-container/40 backdrop-blur-xl border border-primary/20 rounded-3xl p-6 lg:p-8 relative overflow-hidden shadow-xl">
      <div className="absolute top-0 right-0 w-64 h-64 bg-primary/5 rounded-full blur-[80px] pointer-events-none -translate-y-1/2 translate-x-1/3"></div>
      <div className="flex flex-col gap-6 relative z-10">
        <div>
          <div className="flex items-center gap-2">
            <span className="text-[10px] uppercase font-bold tracking-widest text-primary">RESUMEN EJECUTIVO</span>
            <span className="px-2 py-1 rounded-full text-[10px] font-bold bg-primary/10 text-primary flex items-center gap-1 border border-primary/20">
              <Sparkles aria-hidden="true" className="h-3 w-3" />
              {report.insights.provider}
            </span>
            {isProcessing ? (
              <span className="px-2 py-1 rounded-full text-[10px] font-bold bg-amber-400/10 text-amber-200 border border-amber-300/20 animate-pulse">
                Generando insights con IA...
              </span>
            ) : null}
          </div>
          <p className="mt-4 text-xl lg:text-2xl leading-relaxed text-on-surface font-headline font-semibold">
            {report.insights.executive_summary}
          </p>
          {report.insights.pitch_line ? (
            <p className="mt-4 text-sm leading-6 text-slate-300">
              {report.insights.pitch_line}
            </p>
          ) : null}
        </div>

        {compactActions.length > 0 ? (
          <div className="mt-2 border-t border-outline-variant/10 pt-6">
            <div className="text-[10px] uppercase font-bold tracking-widest text-on-surface-variant">Acciones prioritarias</div>
            <div className="mt-4 space-y-3">
              {compactActions.map((action) => {
                const isActive = selectedElementID
                  ? selectedElement?.asset_insight.related_action_id === action.id ||
                    getVisibleRelatedResourceIDs(action).includes(selectedElementID)
                  : action.id === resolvedActiveAction?.id;
                const evidence = getActionEvidence(action);

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
                    className={`w-full rounded-2xl px-4 py-4 text-left transition-colors border ${
                      isActive
                        ? "bg-primary/10 border-primary/20"
                        : "bg-surface-container-low border-outline-variant/5 hover:bg-surface-container-highest"
                    }`}
                  >
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="px-2 py-1 bg-surface-container-highest rounded-md text-[10px] font-bold text-on-surface-variant font-label">
                        {action.confidence}
                      </span>
                      <span className="px-2 py-1 bg-surface-container-highest rounded-md text-[10px] font-bold text-on-surface-variant font-label">
                        {action.likely_lcp_impact}
                      </span>
                    </div>
                    <div className="mt-3 text-base font-headline font-bold text-white">
                      {action.title}
                    </div>
                    <p className="mt-2 text-sm leading-6 text-slate-300">
                      {action.reason}
                    </p>
                    {evidence[0] ? (
                      <p className="mt-2 text-xs leading-5 text-slate-400">
                        {evidence[0]}
                      </p>
                    ) : null}
                    {isProcessing ? (
                      <div className="mt-3 h-1.5 w-24 rounded-full bg-primary/20 overflow-hidden">
                        <div className="h-full w-1/2 rounded-full bg-primary/60 animate-pulse"></div>
                      </div>
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
