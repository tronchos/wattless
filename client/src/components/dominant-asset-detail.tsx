import {
  formatBytes,
  formatConfidenceLabel,
  formatImpactLabel,
  formatPositionBand,
  formatThirdPartyKind,
  formatVisualRole,
} from "@/lib/api";
import type { VampireElement } from "@/lib/types";

interface DominantAssetDetailProps {
  element: VampireElement;
  coverageLabel: string;
  coverageClassName: string;
}

export function DominantAssetDetail({
  element,
  coverageLabel,
  coverageClassName,
}: DominantAssetDetailProps) {
  const insight = element.asset_insight;

  return (
    <div className="border-t border-outline-variant/10 px-4 pb-4 pt-4 md:px-5">
      <div className="flex flex-wrap gap-2 text-[10px] font-label font-bold uppercase tracking-widest">
        <span className={`rounded-full px-2.5 py-1 ${coverageClassName}`}>
          {coverageLabel}
        </span>
        <span className="rounded-full bg-surface-container-highest px-2.5 py-1 text-on-surface-variant">
          {formatPositionBand(element.position_band)}
        </span>
        {element.visual_role !== "unknown" ? (
          <span className="rounded-full bg-surface-container-highest px-2.5 py-1 text-on-surface-variant">
            {formatVisualRole(element.visual_role)}
          </span>
        ) : null}
        <span className="rounded-full bg-surface-container-highest px-2.5 py-1 text-on-surface-variant">
          {formatConfidenceLabel(insight.confidence)}
        </span>
        <span className="rounded-full bg-surface-container-highest px-2.5 py-1 text-on-surface-variant">
          {formatImpactLabel(insight.likely_lcp_impact)}
        </span>
        {element.is_third_party_tool && element.third_party_kind !== "unknown" ? (
          <span className="rounded-full bg-surface-container-highest px-2.5 py-1 text-on-surface-variant">
            {formatThirdPartyKind(element.third_party_kind)}
          </span>
        ) : null}
      </div>

      <div className="mt-4">
        <h3 className="text-lg font-headline font-bold text-on-surface">
          {insight.title}
        </h3>
        <p className="mt-2 text-sm leading-6 text-on-surface-variant">
          {insight.short_problem}
        </p>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetaItem label="Bytes" value={formatBytes(element.bytes)} />
        <MetaItem
          label="Ahorro estimado"
          value={formatBytes(element.estimated_savings_bytes)}
        />
        <MetaItem label="Rol visual" value={formatVisualRole(element.visual_role)} />
        <MetaItem label="Scope" value={insight.scope} />
      </div>

      <div className="mt-5 space-y-4">
        <DetailBlock label="Por qué importa" body={insight.why_it_matters} />
        <DetailBlock
          label="Siguiente acción"
          body={insight.recommended_action}
          accent="text-primary"
        />
      </div>

      {insight.evidence.length > 0 ? (
        <div className="mt-5">
          <div className="text-[10px] font-label font-bold uppercase tracking-widest text-on-surface-variant">
            Evidencia
          </div>
          <ul className="mt-3 space-y-2">
            {insight.evidence.map((item) => (
              <li
                key={`${element.id}-${item}`}
                className="flex items-start gap-3 text-sm leading-6 text-on-surface-variant"
              >
                <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {insight.recommended_fix ? (
        <details className="mt-5 rounded-2xl bg-surface-container-highest/70 p-4">
          <summary className="cursor-pointer list-none text-sm font-headline font-bold text-on-surface">
            Ver fix recomendado
          </summary>
          <div className="mt-4 space-y-4">
            <p className="text-sm leading-6 text-on-surface-variant">
              {insight.recommended_fix.summary}
            </p>
            <div className="rounded-2xl bg-[#0c1017] p-4 text-xs text-slate-200 shadow-inner">
              <pre className="overflow-x-auto whitespace-pre-wrap leading-6">
                <code>{insight.recommended_fix.optimized_code}</code>
              </pre>
            </div>
            {insight.recommended_fix.changes.length > 0 ? (
              <ul className="space-y-2">
                {insight.recommended_fix.changes.map((change) => (
                  <li
                    key={`${element.id}-${change}`}
                    className="flex items-start gap-3 text-sm leading-6 text-on-surface-variant"
                  >
                    <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-primary" />
                    <span>{change}</span>
                  </li>
                ))}
              </ul>
            ) : null}
            <p className="text-sm leading-6 text-primary">
              {insight.recommended_fix.expected_impact}
            </p>
          </div>
        </details>
      ) : null}

      <div className="mt-5 space-y-2">
        {!element.bounding_box ? (
          <p className="text-sm leading-6 text-warning">
            No visual anchor: este recurso no tiene un anclaje visual directo en la captura.
          </p>
        ) : null}
        {coverageLabel === "Outside captured range" ? (
          <p className="text-sm leading-6 text-warning">
            Outside captured range: el asset está por debajo del rango visible del inspector.
          </p>
        ) : null}
        {element.failed && element.failure_reason ? (
          <p className="text-sm leading-6 text-error">
            Request failed: {element.failure_reason}
          </p>
        ) : null}
      </div>
    </div>
  );
}

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl bg-surface-container-highest/55 p-3">
      <div className="text-[10px] font-label font-bold uppercase tracking-widest text-on-surface-variant">
        {label}
      </div>
      <div className="mt-1 text-sm font-headline font-bold text-on-surface">{value}</div>
    </div>
  );
}

function DetailBlock({
  label,
  body,
  accent,
}: {
  label: string;
  body: string;
  accent?: string;
}) {
  return (
    <div>
      <div className="text-[10px] font-label font-bold uppercase tracking-widest text-on-surface-variant">
        {label}
      </div>
      <p className={`mt-2 text-sm leading-6 text-on-surface-variant ${accent ?? ""}`}>
        {body}
      </p>
    </div>
  );
}
