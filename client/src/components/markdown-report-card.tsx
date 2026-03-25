"use client";

import { Copy, Download } from "lucide-react";

import { createMarkdownReport } from "@/lib/report-markdown";
import type { GreenFixResponse, ScanReport } from "@/lib/types";

interface MarkdownReportCardProps {
  report: ScanReport;
  greenFix: GreenFixResponse | null;
}

export function MarkdownReportCard({
  report,
  greenFix,
}: MarkdownReportCardProps) {
  const markdown = createMarkdownReport(report, greenFix);

  async function copyMarkdown() {
    await navigator.clipboard.writeText(markdown);
  }

  function downloadMarkdown() {
    const blob = new Blob([markdown], { type: "text/markdown;charset=utf-8" });
    const objectURL = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = objectURL;
    anchor.download = "wattless-report.md";
    anchor.click();
    URL.revokeObjectURL(objectURL);
  }

  return (
    <section className="panel rounded-[2rem] p-6">
      <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
        Markdown Report
      </p>
      <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
        Llévalo a un README o PR
      </h2>
      <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
        Genera un reporte listo para compartir con jueces, equipo o comunidad
        sin tocar el formato manualmente.
      </p>

      <div className="mt-5 flex flex-wrap gap-3">
        <button
          type="button"
          onClick={copyMarkdown}
          className="inline-flex items-center gap-2 rounded-full border border-[var(--line)] px-4 py-2 text-sm text-white transition hover:border-[var(--line-strong)]"
        >
          <Copy className="h-4 w-4" />
          Copiar Markdown
        </button>
        <button
          type="button"
          onClick={downloadMarkdown}
          className="inline-flex items-center gap-2 rounded-full border border-[var(--line)] px-4 py-2 text-sm text-white transition hover:border-[var(--line-strong)]"
        >
          <Download className="h-4 w-4" />
          Descargar .md
        </button>
      </div>

      <pre className="mt-5 max-h-[260px] overflow-auto rounded-[1.35rem] border border-[var(--line)] bg-[rgba(5,10,8,0.9)] p-4 text-xs leading-6 text-[var(--foreground)]">
        <code>{markdown}</code>
      </pre>
    </section>
  );
}
