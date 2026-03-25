import { formatDateTime, formatMilliseconds } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface MethodologyCardProps {
  report: ScanReport;
}

export function MethodologyCard({ report }: MethodologyCardProps) {
  return (
    <section className="panel rounded-[2rem] p-6">
      <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
        Cómo medimos
      </p>
      <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
        Metodología y trazabilidad del escaneo
      </h2>
      <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
        El reporte combina transferencia real observada en runtime, Core Web
        Vitals y la fórmula base de Sustainable Web Design para traducir bytes
        a impacto por visita.
      </p>

      <div className="mt-5 grid gap-3 sm:grid-cols-2">
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Modelo
          </div>
          <div className="mt-2 text-sm text-white">{report.methodology.model}</div>
        </div>
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Fuente
          </div>
          <div className="mt-2 text-sm text-white">{report.methodology.source}</div>
        </div>
      </div>

      <div className="mt-4 rounded-[1.5rem] border border-[var(--line)] bg-[rgba(5,10,8,0.72)] p-4">
        <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--accent)]">
          Fórmula base
        </div>
        <code className="mt-3 block overflow-x-auto text-sm leading-7 text-[var(--foreground)]">
          {report.methodology.formula}
        </code>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Generado
          </div>
          <div className="mt-2 text-sm text-white">{formatDateTime(report.meta.generated_at)}</div>
        </div>
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Duración
          </div>
          <div className="mt-2 text-sm text-white">
            {formatMilliseconds(report.meta.scan_duration_ms)}
          </div>
        </div>
        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
            Versión
          </div>
          <div className="mt-2 text-sm text-white">{report.meta.scanner_version}</div>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {report.methodology.assumptions.map((assumption) => (
          <span
            key={assumption}
            className="rounded-full border border-[var(--line)] bg-[rgba(255,255,255,0.02)] px-3 py-1 text-xs text-[var(--muted)]"
          >
            {assumption}
          </span>
        ))}
      </div>
    </section>
  );
}
