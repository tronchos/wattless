import { formatDateTime, formatMilliseconds } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface MethodologyCardProps {
  report: ScanReport;
}

export function MethodologyCard({ report }: MethodologyCardProps) {
  return (
    <section id="methodology" className="surface-primary rounded-[1.6rem] p-5">
      <p className="section-kicker">Cómo medimos</p>
      <h2 className="editorial-copy mt-3 text-xl font-medium tracking-[-0.05em] text-white">
        Metodología y trazabilidad
      </h2>
      <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
        Wattless separa tres capas: score por transferencia observada, señales
        de laboratorio como LCP/FCP/Long Tasks y findings interpretativos basados
        en evidencia visual, red y posición dentro del documento.
      </p>

      <div className="mt-5 grid gap-3 sm:grid-cols-2">
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Modelo</div>
          <div className="mt-2 text-sm text-white">{report.methodology.model}</div>
        </div>
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Fuente</div>
          <div className="mt-2 text-sm text-white">{report.methodology.source}</div>
        </div>
      </div>

      <div className="tonal-pocket mt-4 rounded-[1rem] p-4">
        <div className="section-kicker text-[var(--accent)]">Fórmula base</div>
        <code className="mt-3 block overflow-x-auto text-sm leading-7 text-[var(--foreground)]">
          {report.methodology.formula}
        </code>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Generado</div>
          <div className="mt-2 text-sm text-white">
            {formatDateTime(report.meta.generated_at)}
          </div>
        </div>
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Duración</div>
          <div className="mt-2 text-sm text-white">
            {formatMilliseconds(report.meta.scan_duration_ms)}
          </div>
        </div>
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Versión</div>
          <div className="mt-2 text-sm text-white">
            {report.meta.scanner_version}
          </div>
        </div>
      </div>

      <div className="mt-4 grid gap-3">
        <div className="surface-secondary rounded-[1rem] p-4">
          <div className="section-kicker">Performance lab</div>
          <div className="mt-2 text-sm leading-7 text-white">
            LCP {formatMilliseconds(report.performance.lcp_ms)}, FCP{" "}
            {formatMilliseconds(report.performance.fcp_ms)} y Long Tasks{" "}
            {formatMilliseconds(report.performance.long_tasks_total_ms)}.
          </div>
          <p className="mt-2 text-sm leading-6 text-[var(--muted)]">
            `script_resource_duration_ms` es un proxy de carga/red de scripts,
            no una medida de bloqueo real de CPU.
          </p>
        </div>
      </div>

      <div className="mt-4 flex flex-wrap gap-2">
        {report.methodology.assumptions.map((assumption) => (
          <span
            key={assumption}
            className="soft-chip bg-[rgba(255,255,255,0.03)] normal-case tracking-[0.08em]"
          >
            {assumption}
          </span>
        ))}
      </div>
    </section>
  );
}
