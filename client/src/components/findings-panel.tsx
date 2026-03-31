import {
  formatBytes,
  formatConfidenceLabel,
  formatSeverityLabel,
} from "@/lib/api";
import type { AnalysisFinding } from "@/lib/types";
import { AlertCircle, CheckCircle2, ChevronRight, ChevronDown, Zap } from "lucide-react";

interface FindingsPanelProps {
  findings: AnalysisFinding[];
}

export function FindingsPanel({ findings }: FindingsPanelProps) {
  if (findings.length === 0) {
    return (
      <section className="bg-surface-container-low border border-outline-variant/10 rounded-[2rem] p-8 mt-8">
        <div className="flex flex-col items-center justify-center text-center max-w-lg mx-auto py-6">
          <div className="w-16 h-16 bg-success/10 text-success rounded-full flex items-center justify-center mb-6">
            <CheckCircle2 className="w-8 h-8" />
          </div>
          <h2 className="text-2xl font-headline font-bold text-on-surface">
            Carga Optimizada
          </h2>
          <p className="mt-3 text-sm leading-relaxed text-on-surface-variant">
            La carga observada es excepcionalmente limpia. Wattless no detectó hallazgos 
            prioritarios que comprometan significativamente el rendimiento o la transferencia.
          </p>
        </div>
      </section>
    );
  }

  // Helper to get color accents based on severity
  const getSeverityAccent = (severity: string) => {
    switch (severity) {
      case "high": return "bg-error/10 text-error border-error/20";
      case "medium": return "bg-[var(--accent)]/10 text-[var(--accent)] border-[var(--accent)]/20";
      case "low": return "bg-surface-container-highest text-on-surface border-outline-variant/20";
      default: return "bg-surface-container-highest text-on-surface border-outline-variant/20";
    }
  };

  return (
    <section className="bg-surface-container-low border border-outline-variant/10 rounded-[2rem] p-8 lg:p-10 mt-8 relative overflow-hidden">
      {/* Background glow decoration */}
      <div className="absolute top-0 right-0 w-96 h-96 bg-primary/5 rounded-full blur-[100px] pointer-events-none -translate-y-1/2 translate-x-1/3"></div>

      <div className="relative">
        <div className="flex flex-col gap-2 max-w-2xl">
          <div className="flex items-center gap-2">
            <AlertCircle className="w-4 h-4 text-primary" />
            <p className="text-primary text-[10px] uppercase tracking-widest font-label font-bold">
              Findings Estratégicos
            </p>
          </div>
          <h2 className="text-3xl font-headline font-bold text-on-surface mt-2">
            Diagnóstico Priorizado
          </h2>
          <p className="text-sm leading-relaxed text-on-surface-variant mt-2">
            Qué compromete el performance y por qué importa. Priorizado por severidad, 
            grado de confianza algorítmico y potencial de ahorro real.
          </p>
        </div>

        <div className="mt-10 grid gap-6 xl:grid-cols-2">
          {findings.map((finding) => {
            const evidence = Array.isArray(finding.evidence) ? finding.evidence : [];

            return (
              <article
                key={finding.id}
                className="bg-surface-container rounded-3xl p-6 border border-outline-variant/5 flex flex-col hover:border-outline-variant/20 transition-colors"
              >
                <div className="flex flex-wrap items-center gap-2.5 mb-5">
                  <span className={`px-2.5 py-1 rounded-md text-[10px] uppercase tracking-widest font-label font-bold border ${getSeverityAccent(finding.severity)}`}>
                    {formatSeverityLabel(finding.severity)}
                  </span>
                  <span className="px-2.5 py-1 rounded-md text-[10px] uppercase tracking-widest font-label font-bold bg-surface-container-highest text-on-surface-variant border border-outline-variant/10">
                    {formatConfidenceLabel(finding.confidence)}
                  </span>
                  {finding.estimated_savings_bytes > 0 && (
                    <span className="px-2.5 py-1 rounded-md text-[10px] uppercase tracking-widest font-label font-bold bg-primary/10 text-primary border border-primary/20 ml-auto flex items-center gap-1">
                      <Zap className="w-3 h-3" />
                      {formatBytes(finding.estimated_savings_bytes)}
                    </span>
                  )}
                </div>

                <h3 className="text-xl font-headline font-bold text-on-surface leading-snug">
                  {finding.title}
                </h3>
                <p className="mt-3 text-sm leading-relaxed text-on-surface-variant">
                  {finding.summary}
                </p>

                {evidence.length > 0 && (
                  <details className="mt-auto pt-6 group">
                    <summary className="text-[10px] tracking-widest uppercase font-label text-on-surface-variant font-semibold cursor-pointer list-none flex items-center justify-between hover:text-primary transition-colors bg-surface-container-low p-2 rounded-lg">
                      <span>Ver Evidencia ({evidence.length})</span>
                      <ChevronDown className="w-3.5 h-3.5 group-open:rotate-180 transition-transform" />
                    </summary>
                    <ul className="mt-3 space-y-2.5 text-sm leading-relaxed text-on-surface bg-surface-container-low p-5 rounded-2xl border border-outline-variant/5">
                      {evidence.map((item, index) => (
                        <li key={`${finding.id}-${index}`} className="flex items-start gap-2.5">
                          <ChevronRight className="w-4 h-4 text-primary shrink-0 mt-0.5" />
                          <span className="font-medium opacity-90">{item}</span>
                        </li>
                      ))}
                    </ul>
                  </details>
                )}
              </article>
            );
          })}
        </div>
      </div>
    </section>
  );
}
