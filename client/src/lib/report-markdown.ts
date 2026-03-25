import { formatBytes, formatGrams, formatMilliseconds } from "@/lib/api";
import type { GreenFixResponse, ScanReport } from "@/lib/types";

const wattlessRepositoryURL = "https://github.com/tronchos/wattless";
const wattlessAppURL =
  process.env.NEXT_PUBLIC_APP_URL?.trim() || "despliegue público pendiente";

export function createMarkdownReport(
  report: ScanReport,
  greenFix: GreenFixResponse | null,
): string {
  const lines = [
    `# Wattless Report`,
    ``,
    `- Generado con Wattless: ${wattlessAppURL}`,
    `- Repo: ${wattlessRepositoryURL}`,
    `- URL: ${report.url}`,
    `- Score: ${report.score}`,
    `- CO2 por visita: ${formatGrams(report.co2_grams_per_visit)}`,
    `- Transferencia total: ${formatBytes(report.total_bytes_transferred)}`,
    `- LCP: ${formatMilliseconds(report.performance.lcp_ms)}`,
    `- FCP: ${formatMilliseconds(report.performance.fcp_ms)}`,
    `- Load: ${formatMilliseconds(report.performance.load_ms)}`,
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
    `## Elementos vampiro`,
    ``,
    ...report.vampire_elements.map(
      (element, index) =>
        `- #${index + 1} ${element.type}: ${element.url} (${formatBytes(element.bytes)})`,
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

  if (greenFix) {
    lines.push(`## Green Fix sugerido`, ``);
    lines.push(greenFix.summary, ``);
    lines.push(...greenFix.changes.map((change) => `- ${change}`), ``);
    lines.push("```tsx", greenFix.optimized_code, "```", ``);
    lines.push(`Impacto esperado: ${greenFix.expected_impact}`, ``);
  }

  return lines.join("\n");
}
