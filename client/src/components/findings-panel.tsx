import {
  formatBytes,
  formatConfidenceLabel,
  formatSeverityLabel,
} from "@/lib/api";
import type { AnalysisFinding } from "@/lib/types";

interface FindingsPanelProps {
  findings: AnalysisFinding[];
}

export function FindingsPanel({ findings }: FindingsPanelProps) {
  if (findings.length === 0) {
    return (
      <section className="surface-primary rounded-[1.6rem] p-6">
        <p className="section-kicker">Findings</p>
        <h2 className="editorial-copy mt-3 text-2xl text-white">
          No se detectaron hallazgos prioritarios
        </h2>
        <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
          La carga observada es relativamente limpia. Wattless no encontró una
          relación fuerte entre bytes, render crítico y sobrecarga de terceros.
        </p>
      </section>
    );
  }

  return (
    <section className="surface-primary rounded-[1.6rem] p-6 lg:p-8">
      <div className="flex flex-col gap-2">
        <p className="section-kicker">Findings</p>
        <h2 className="editorial-copy text-2xl text-white">
          Qué está mal y por qué importa
        </h2>
        <p className="text-sm leading-7 text-[var(--muted)]">
          Hallazgos priorizados por severidad, confianza y ahorro potencial.
        </p>
      </div>

      <div className="mt-6 grid gap-4 xl:grid-cols-2">
        {findings.map((finding) => (
          <article
            key={finding.id}
            className="surface-secondary rounded-[1.2rem] p-5"
          >
            <div className="flex flex-wrap items-center gap-2">
              <span className="soft-chip bg-[rgba(255,255,255,0.04)]">
                {formatSeverityLabel(finding.severity)}
              </span>
              <span className="soft-chip bg-[rgba(255,255,255,0.04)]">
                {formatConfidenceLabel(finding.confidence)}
              </span>
              <span className="soft-chip bg-[rgba(155,214,126,0.08)] text-[var(--accent)]">
                {formatBytes(finding.estimated_savings_bytes)} potenciales
              </span>
            </div>

            <h3 className="mt-4 text-xl font-headline font-bold text-white">
              {finding.title}
            </h3>
            <p className="mt-3 text-sm leading-7 text-[var(--muted)]">
              {finding.summary}
            </p>

            {finding.evidence.length > 0 ? (
              <ul className="mt-5 space-y-2 text-sm leading-6 text-slate-300">
                {finding.evidence.map((item, index) => (
                  <li key={`${finding.id}-${index}`} className="flex items-start gap-3">
                    <span className="mt-2 h-1.5 w-1.5 rounded-full bg-[var(--accent)] shrink-0" />
                    <span>{item}</span>
                  </li>
                ))}
              </ul>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}
