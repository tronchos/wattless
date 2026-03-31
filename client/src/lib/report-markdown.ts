import { formatBytes, formatGrams, formatMilliseconds } from "@/lib/api";
import type { ScanReport } from "@/lib/types";

const wattlessRepositoryURL = "https://github.com/tronchos/wattless";

export function createMarkdownReport(
  report: ScanReport,
): string {
  const wattlessAppURL = resolveWattlessAppURL();
  const textualFirstRenderNote = inferTextualFirstRenderNote(report);
  const lcpValue = report.performance.render_metrics_complete
    ? formatMilliseconds(report.performance.lcp_ms)
    : "No capturado";
  const fcpValue = report.performance.render_metrics_complete
    ? formatMilliseconds(report.performance.fcp_ms)
    : "No capturado";
  const lines = [
    `# Wattless Report`,
    ``,
    `- Generado con Wattless: ${wattlessAppURL}`,
    `- Repo: ${wattlessRepositoryURL}`,
    `- URL: ${report.url}`,
    `- Score: ${report.score}`,
    `- CO2 por visita: ${formatGrams(report.co2_grams_per_visit)}`,
    `- Transferencia total: ${formatBytes(report.total_bytes_transferred)}`,
    `- LCP: ${lcpValue}`,
    `- FCP: ${fcpValue}`,
    `- Long Tasks: ${formatMilliseconds(report.performance.long_tasks_total_ms)} (${report.performance.long_tasks_count})`,
    `- Load: ${formatMilliseconds(report.performance.load_ms)}`,
    `- Inspector coverage: ${formatInspectorCoverage(report)}`,
    `- Hosting: ${report.hosting_verdict}${report.hosted_by ? ` (${report.hosted_by})` : ""}`,
    `- Generado: ${report.meta.generated_at}`,
    `- Duración del escaneo: ${formatMilliseconds(report.meta.scan_duration_ms)}`,
    `- Versión del scanner: ${report.meta.scanner_version}`,
    ``,
    `## Resumen ejecutivo`,
    ``,
    report.insights.executive_summary,
    ``,
    `> ${report.insights.pitch_line}`,
    ``,
    `## Acciones prioritarias`,
    ``,
    ...report.insights.top_actions.map(
      (action, index) =>
        `${index + 1}. ${action.title} - ${action.reason} (ahorro estimado: ${formatBytes(action.estimated_savings_bytes)})`,
    ),
    ``,
    `## Findings`,
    ``,
    ...(report.analysis.findings.length > 0
      ? report.analysis.findings.map(
          (finding) =>
            `- [${finding.severity}/${finding.confidence}] ${finding.title}: ${finding.summary}`,
        )
      : [`- No se detectaron hallazgos prioritarios.`]),
    ``,
    `## Evidence`,
    ``,
    `- Above the fold visual bytes: ${formatBytes(report.analysis.summary.above_fold_visual_bytes)}`,
    `- Below the fold bytes: ${formatBytes(report.analysis.summary.below_fold_bytes)}`,
    `- LCP resource: ${report.analysis.summary.lcp_resource_url || "Sin match"}${report.analysis.summary.lcp_resource_bytes ? ` (${formatBytes(report.analysis.summary.lcp_resource_bytes)})` : ""}`,
    `- Analytics bytes: ${formatBytes(report.analysis.summary.analytics_bytes)}`,
    `- Font bytes: ${formatBytes(report.analysis.summary.font_bytes)}`,
    `- Render critical bytes: ${formatBytes(report.analysis.summary.render_critical_bytes)}`,
    ``,
    ...(textualFirstRenderNote ? [`> ${textualFirstRenderNote}`, ``] : []),
    `## Elementos vampiro`,
    ``,
    ...report.vampire_elements.map(
      (element, index) =>
        `- #${index + 1} ${element.type}: ${element.url} (${formatBytes(element.bytes)}, ${element.visual_role}, ${element.position_band})`,
    ),
    ``,
    `## Metodología`,
    ``,
    `- Modelo: ${report.methodology.model}`,
    `- Fuente: ${report.methodology.source}`,
    `- Fórmula: \`${report.methodology.formula}\``,
    ...report.methodology.assumptions.map((assumption) => `- Supuesto: ${assumption}`),
    ``,
  ];

  if (report.warnings.length > 0) {
    lines.push(`## Advertencias`, ``);
    lines.push(...report.warnings.map((warning) => `- ${warning}`), ``);
  }

  const firstAssetFix =
    report.vampire_elements.find((element) => element.asset_insight.recommended_fix)
      ?.asset_insight.recommended_fix ?? report.insights.top_actions[0]?.recommended_fix;
  if (firstAssetFix) {
    const fix = firstAssetFix;
    lines.push(`## Wattless Optimization (Sugerencia Automatizada)`, ``);
    lines.push(fix.summary, ``);
    lines.push(...fix.changes.map((change) => `- ${change}`), ``);
    lines.push("```tsx", fix.optimized_code, "```", ``);
    lines.push(`Impacto esperado: ${fix.expected_impact}`, ``);
  }

  return lines.join("\n");
}

function resolveWattlessAppURL(): string {
  return import.meta.env.VITE_PUBLIC_APP_URL?.trim() || "despliegue público pendiente";
}

function formatInspectorCoverage(report: ScanReport): string {
  const { captured_height, document_height } = report.screenshot;
  if (captured_height >= document_height) {
    return `${captured_height} / ${document_height} px`;
  }

  return `${captured_height} / ${document_height} px (truncated)`;
}

function inferTextualFirstRenderNote(report: ScanReport): string | null {
  if (report.analysis.summary.above_fold_visual_bytes !== 0) {
    return null;
  }
  if (report.analysis.summary.render_critical_bytes <= 0) {
    return null;
  }
  if (!report.analysis.findings.some((finding) => finding.id === "render_lcp_dom_node")) {
    return null;
  }
  return "El primer render depende sobre todo de texto, fuentes y CSS. Que los `above_fold_visual_bytes` sean 0 no implica que el hero esté vacío: aquí el coste crítico vive en estilos y tipografía, no en media visible.";
}
