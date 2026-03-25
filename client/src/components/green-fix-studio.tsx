"use client";

import { LoaderCircle } from "lucide-react";

import type { GreenFixResponse, ScanReport } from "@/lib/types";

interface GreenFixStudioProps {
  report: ScanReport;
  code: string;
  onCodeChange: (value: string) => void;
  onGenerate: () => void;
  isGenerating: boolean;
  result: GreenFixResponse | null;
  error: string | null;
}

export function GreenFixStudio({
  report,
  code,
  onCodeChange,
  onGenerate,
  isGenerating,
  result,
  error,
}: GreenFixStudioProps) {
  return (
    <section id="green-fix" className="space-y-8 mt-8">
      <div className="flex flex-col md:flex-row justify-between md:items-center gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h2 className="text-3xl font-bold font-headline text-on-surface">Green Fix Studio</h2>
            {report.insights?.provider && (
               <span className="bg-primary/10 text-primary px-3 py-1 rounded-full text-xs font-bold font-label">
                 ✨ {report.insights.provider}
               </span>
            )}
          </div>
          <p className="text-on-surface-variant text-sm mt-1">
            Suggested code optimizations for low-carbon rendering.
          </p>
        </div>
        <button
          onClick={onGenerate}
          disabled={isGenerating}
          className="bg-surface-container-highest text-primary px-6 py-2 rounded-xl text-sm font-bold border border-primary/20 hover:border-primary/50 transition-all disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
        >
          {isGenerating ? (
            <>
              <LoaderCircle className="w-4 h-4 animate-spin" />
              Applying...
            </>
          ) : (
             "Apply Fixes"
          )}
        </button>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 bg-surface-container-lowest rounded-2xl overflow-hidden border border-outline-variant/10 shadow-2xl">
        {/* Original Code */}
        <div className="p-6 border-r border-outline-variant/10 flex flex-col">
          <div className="text-[10px] text-on-surface-variant uppercase font-label mb-4 flex items-center justify-between gap-2">
            <span className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-error"></span> Current Implementation
            </span>
            <span className="opacity-50">Paste your code below</span>
          </div>
          
          <textarea
            value={code}
            onChange={(event) => onCodeChange(event.target.value)}
            placeholder={`export function Hero() {\n  return (\n    <section>\n      <img src="/hero.jpg" alt="Hero" />\n    </section>\n  );\n}`}
            className="flex-1 min-h-[300px] w-full bg-transparent text-xs font-body text-on-surface-variant/60 leading-relaxed outline-none resize-none"
            spellCheck={false}
          />
          {error && (
             <div className="mt-4 text-xs text-error bg-error-container/10 p-2 rounded border border-error-container/20">
               {error}
             </div>
          )}
        </div>

        {/* Optimized Code */}
        <div className="p-6 bg-primary-container/5 flex flex-col">
          <div className="text-[10px] text-primary uppercase font-label mb-4 flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-primary"></span> Wattless Optimization
          </div>
          
          {result ? (
            <div className="flex-1 flex flex-col gap-4">
               <pre className="text-xs font-body text-on-surface leading-relaxed overflow-x-auto bg-surface-container-lowest p-4 rounded-xl border border-primary/10 flex-1">
                 <code>{result.optimized_code}</code>
               </pre>
               <div className="text-xs text-on-surface-variant bg-surface-container-highest/50 p-3 rounded-lg border border-outline-variant/5">
                 <strong className="text-primary font-bold">Impact: </strong> {result.expected_impact}
                 <ul className="mt-2 pl-4 list-disc opacity-80">
                   {result.changes.map(c => <li key={c}>{c}</li>)}
                 </ul>
               </div>
            </div>
          ) : (
            <div className="flex-1 flex items-center justify-center text-xs text-on-surface-variant/40 font-body text-center p-8 border border-dashed border-outline-variant/20 rounded-xl">
               Waiting for code input to generate low-carbon rendering fix...
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
