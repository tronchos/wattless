import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, Download, FileText } from "lucide-react";

import { createMarkdownReport } from "@/lib/report-markdown";
import type { ScanReport } from "@/lib/types";

interface MarkdownReportCardProps {
  report: ScanReport;
}

export function MarkdownReportCard({
  report,
}: MarkdownReportCardProps) {
  const markdown = useMemo(() => createMarkdownReport(report), [report]);
  const [copyStatus, setCopyStatus] = useState<"idle" | "copied" | "error">("idle");
  const resetTimerRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimerRef.current !== null) {
        window.clearTimeout(resetTimerRef.current);
      }
    };
  }, []);

  async function copyMarkdown() {
    try {
      await navigator.clipboard.writeText(markdown);
      showCopyFeedback("copied");
      return;
    } catch {
      if (copyWithLegacyFallback(markdown)) {
        showCopyFeedback("copied");
        return;
      }
    }

    showCopyFeedback("error");
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
            aria-label="Copy report as markdown"
            className="bg-surface-container-highest text-on-surface px-8 py-3 rounded-xl font-bold hover:bg-surface-container-high transition-colors flex items-center gap-2 text-sm"
          >
            {copyStatus === "copied" ? (
              <><Check className="w-4 h-4" /> Copied!</>
            ) : copyStatus === "error" ? (
              <><Copy className="w-4 h-4" /> Copy failed</>
            ) : (
              <><Copy className="w-4 h-4" /> Copy Markdown</>
            )}
          </button>
          <button
            onClick={downloadMarkdown}
            aria-label="Download report as markdown file"
            className="bg-primary text-on-primary px-8 py-3 rounded-xl font-bold hover:bg-primary-dim transition-colors flex items-center gap-2 shadow-lg shadow-primary/10 text-sm"
          >
            <Download className="w-4 h-4" /> Export MD
          </button>
        </div>
      </div>
    </section>
  );

  function showCopyFeedback(status: "copied" | "error") {
    setCopyStatus(status);
    if (resetTimerRef.current !== null) {
      window.clearTimeout(resetTimerRef.current);
    }
    resetTimerRef.current = window.setTimeout(() => {
      setCopyStatus("idle");
      resetTimerRef.current = null;
    }, 2000);
  }
}

function copyWithLegacyFallback(text: string): boolean {
  const textarea = document.createElement("textarea");
  try {
    textarea.value = text;
    textarea.setAttribute("readonly", "true");
    textarea.style.position = "fixed";
    textarea.style.opacity = "0";
    document.body.appendChild(textarea);
    textarea.select();
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    textarea.remove();
  }
}
