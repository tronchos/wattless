import { formatDateTime, formatMilliseconds } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface MethodologyCardProps {
  report: ScanReport;
}

export function MethodologyCard({ report }: MethodologyCardProps) {
  return (
    <section id="methodology" className="bg-surface-container-high rounded-3xl p-6 border border-outline-variant/10 flex flex-col h-full">
      <div className="text-primary text-[10px] uppercase tracking-widest font-label font-bold mb-2">
        Cómo medimos
      </div>
      <h2 className="text-xl font-headline font-bold text-on-surface">
        Metodología y trazabilidad
      </h2>
      <p className="mt-3 text-sm leading-relaxed text-on-surface-variant">
        Wattless evalúa tres dimensiones: transferencia en red, señales de 
        laboratorio de CPU (Performance Lab) y findings interpretativos sobre media y DOM.
      </p>

      <div className="mt-6 grid gap-3 sm:grid-cols-2">
        <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
          <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Modelo UI/UX</div>
          <div className="mt-1.5 text-sm font-medium text-on-surface">{report.methodology.model}</div>
        </div>
        <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
          <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Motor de Escaneo</div>
          <div className="mt-1.5 text-sm font-medium text-on-surface">{report.methodology.source}</div>
        </div>
      </div>

      <div className="mt-3 bg-primary/5 rounded-2xl p-4 border border-primary/10">
        <div className="text-primary text-[10px] uppercase tracking-widest font-label">Fórmula Base</div>
        <code className="mt-2 block overflow-x-auto text-xs leading-relaxed text-on-surface font-mono">
          {report.methodology.formula}
        </code>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-3">
        <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
          <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Generado</div>
          <div className="mt-1 text-sm font-medium text-on-surface">
            {formatDateTime(report.meta.generated_at)}
          </div>
        </div>
        <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
          <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Duración lab</div>
          <div className="mt-1 text-sm font-medium text-on-surface">
            {formatMilliseconds(report.meta.scan_duration_ms)}
          </div>
        </div>
        <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
          <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Core version</div>
          <div className="mt-1 text-sm font-medium text-on-surface">
            {report.meta.scanner_version}
          </div>
        </div>
      </div>

      <div className="mt-auto pt-6">
        <div className="flex flex-wrap gap-2">
          {report.methodology.assumptions.map((assumption) => (
            <span
              key={assumption}
              className="bg-surface-container-highest text-on-surface-variant px-3 py-1.5 rounded-full text-[10px] uppercase tracking-widest font-label border border-outline-variant/20"
            >
              {assumption}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}
