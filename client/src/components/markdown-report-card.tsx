"use client";

import { Copy, Download, FileText } from "lucide-react";

import { createMarkdownReport } from "@/lib/report-markdown";
import type { ScanReport } from "@/lib/types";

interface MarkdownReportCardProps {
  report: ScanReport;
}

export function MarkdownReportCard({
  report,
}: MarkdownReportCardProps) {
  const markdown = createMarkdownReport(report);

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
    <section className="bg-surface-container-low rounded-[2rem] p-8 md:p-12 border border-outline-variant/5 mt-8">
      <div className="max-w-3xl mx-auto space-y-8">
        <div className="text-center">
          <FileText className="w-12 h-12 text-primary mx-auto mb-4 opacity-80" />
          <h2 className="text-3xl font-bold font-headline text-on-surface">Audit Summary</h2>
          <p className="text-on-surface-variant mt-2 text-sm">
            Download your sustainability report in markdown format.
          </p>
        </div>

        <div className="bg-surface-container-highest/50 p-6 rounded-xl font-body text-sm text-on-surface-variant leading-loose border border-outline-variant/10 max-h-[400px] overflow-y-auto">
          <pre className="whitespace-pre-wrap font-body text-xs">
            {markdown}
          </pre>
        </div>

        <div className="flex justify-center gap-4">
          <button
            onClick={copyMarkdown}
            className="bg-surface-container-highest text-on-surface px-8 py-3 rounded-xl font-bold hover:bg-surface-container-high transition-colors flex items-center gap-2 text-sm"
          >
            <Copy className="w-4 h-4" /> Copy Markdown
          </button>
          <button
            onClick={downloadMarkdown}
            className="bg-primary text-on-primary px-8 py-3 rounded-xl font-bold hover:bg-primary-dim transition-colors flex items-center gap-2 shadow-lg shadow-primary/10 text-sm"
          >
            <Download className="w-4 h-4" /> Export MD
          </button>
        </div>
      </div>
    </section>
  );
}
