import { formatBytes } from "@/lib/api";
import type { VampireElement } from "@/lib/types";

interface DominantAssetDetailProps {
  element: VampireElement;
}

export function DominantAssetDetail({ element }: DominantAssetDetailProps) {
  const insight = element.asset_insight;
  const evidence = Array.isArray(insight.evidence) ? insight.evidence : [];
  const primaryEvidence = evidence[0] ?? null;
  const fixChanges =
    insight.recommended_fix && Array.isArray(insight.recommended_fix.changes)
      ? insight.recommended_fix.changes
      : [];

  return (
    <div className="border-t border-outline-variant/10 px-4 pb-4 pt-3 md:px-5 space-y-3">
      <p className="text-sm leading-6 text-on-surface-variant">
        {insight.short_problem}
      </p>

      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
        <p className="text-sm leading-6 text-primary">
          <span className="mr-1 opacity-60">&#9656;</span>
          {insight.recommended_action}
        </p>
        {element.estimated_savings_bytes > 0 ? (
          <span className="shrink-0 text-xs text-on-surface-variant">
            Ahorro est. ~{formatBytes(element.estimated_savings_bytes)}
          </span>
        ) : null}
      </div>

      {primaryEvidence ? (
        <p className="text-xs leading-5 text-on-surface-variant/70">
          {primaryEvidence}
        </p>
      ) : null}

      {insight.recommended_fix ? (
        <details className="rounded-2xl bg-surface-container-highest/70 px-4 py-3">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-3 text-sm font-headline font-bold text-on-surface">
            <span>Ver fix recomendado</span>
            <span className="rounded-full bg-primary/10 px-2.5 py-1 text-[10px] font-label font-bold uppercase tracking-widest text-primary">
              {insight.recommended_fix.expected_impact}
            </span>
          </summary>
          <div className="mt-3 space-y-3 border-t border-outline-variant/10 pt-3">
            <p className="text-xs leading-5 text-on-surface-variant">
              {insight.recommended_fix.summary}
            </p>
            <div className="rounded-2xl bg-[#0c1017] p-3 text-xs text-slate-200 shadow-inner">
              <pre className="max-h-40 overflow-auto whitespace-pre-wrap leading-5">
                <code>{insight.recommended_fix.optimized_code}</code>
              </pre>
            </div>
            {fixChanges.length > 0 ? (
              <ul className="space-y-1.5">
                {fixChanges.slice(0, 2).map((change) => (
                  <li
                    key={`${element.id}-${change}`}
                    className="flex items-start gap-3 text-xs leading-5 text-on-surface-variant"
                  >
                    <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                    <span>{change}</span>
                  </li>
                ))}
              </ul>
            ) : null}
          </div>
        </details>
      ) : null}
    </div>
  );
}
