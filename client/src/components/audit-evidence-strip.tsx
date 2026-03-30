import { formatBytes, formatResourceLabel } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

interface AuditEvidenceStripProps {
  report: ScanReport;
}

export function AuditEvidenceStrip({ report }: AuditEvidenceStripProps) {
  const lcpResource = report.vampire_elements.find(
    (element) => element.id === report.analysis.summary.lcp_resource_id,
  );
  const lcpBytes = lcpResource?.bytes ?? report.analysis.summary.lcp_resource_bytes ?? null;
  const lcpDetail = lcpResource
    ? formatResourceLabel(lcpResource.type)
    : report.analysis.summary.lcp_resource_url
      ? "Recurso LCP mapeado fuera del overlay"
      : "No se detectó un asset visual único";

  const items = [
    {
      label: "Above the fold",
      value: formatBytes(report.analysis.summary.above_fold_visual_bytes),
      detail: "Peso visual visible en el primer viewport",
    },
    {
      label: "Below the fold",
      value: formatBytes(report.analysis.summary.below_fold_bytes),
      detail: "Contenido que no merece la misma prioridad",
    },
    {
      label: "LCP resource",
      value: lcpBytes !== null ? formatBytes(lcpBytes) : "Sin match",
      detail: lcpDetail,
    },
    {
      label: "Analytics",
      value: formatBytes(report.analysis.summary.analytics_bytes),
      detail: `${report.analysis.summary.analytics_requests} requests`,
    },
    {
      label: "Fonts",
      value: formatBytes(report.analysis.summary.font_bytes),
      detail: `${report.analysis.summary.font_requests} archivos`,
    },
  ];

  return (
    <section className="grid gap-3 lg:grid-cols-5">
      {items.map((item) => (
        <article
          key={item.label}
          className="bg-surface-container-low rounded-3xl px-6 py-6 border border-outline-variant/5 hover:bg-surface-container transition-colors"
        >
          <p className="section-kicker">{item.label}</p>
          <div className="mt-3 text-2xl font-headline font-bold text-white">
            {item.value}
          </div>
          <p className="mt-2 text-sm leading-6 text-[var(--muted)]">
            {item.detail}
          </p>
        </article>
      ))}
    </section>
  );
}
