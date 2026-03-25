import { formatBytes, formatGrams, formatMilliseconds } from "@/lib/api";
import type { GreenFixResponse, ScanReport } from "@/lib/types";

export function createMarkdownReport(
  report: ScanReport,
  greenFix: GreenFixResponse | null,
): string {
  const lines = [
    `# Wattless Report`,
    ``,
    `- URL: ${report.url}`,
    `- Score: ${report.score}`,
    `- CO2 por visita: ${formatGrams(report.co2_grams_per_visit)}`,
    `- Transferencia total: ${formatBytes(report.total_bytes_transferred)}`,
    `- LCP: ${formatMilliseconds(report.performance.lcp_ms)}`,
    `- FCP: ${formatMilliseconds(report.performance.fcp_ms)}`,
    `- Load: ${formatMilliseconds(report.performance.load_ms)}`,
    `- Hosting: ${report.hosting_verdict}${report.hosted_by ? ` (${report.hosted_by})` : ""}`,
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
  ];

  if (greenFix) {
    lines.push(`## Green Fix sugerido`, ``);
    lines.push(greenFix.summary, ``);
    lines.push(...greenFix.changes.map((change) => `- ${change}`), ``);
    lines.push("```tsx", greenFix.optimized_code, "```", ``);
    lines.push(`Impacto esperado: ${greenFix.expected_impact}`, ``);
  }

  return lines.join("\n");
}
