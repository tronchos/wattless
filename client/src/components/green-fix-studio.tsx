"use client";

import { LoaderCircle, Sparkles, WandSparkles } from "lucide-react";

import type { GreenFixResponse, ScanReport } from "@/lib/types";

interface GreenFixStudioProps {
  report: ScanReport;
  code: string;
  onCodeChange: (value: string) => void;
  onUseDemoSnippet: () => void;
  onGenerate: () => void;
  isGenerating: boolean;
  result: GreenFixResponse | null;
}

export function GreenFixStudio({
  report,
  code,
  onCodeChange,
  onUseDemoSnippet,
  onGenerate,
  isGenerating,
  result,
}: GreenFixStudioProps) {
  return (
    <section className="panel rounded-[2rem] p-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
            Green Fix Studio
          </p>
          <h2 className="mt-3 text-2xl font-medium tracking-[-0.05em] text-white">
            Refactor útil para la demo
          </h2>
        </div>
        <span className="mono inline-flex items-center gap-2 rounded-full border border-[var(--line-strong)] px-3 py-1 text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
          <Sparkles className="h-3.5 w-3.5" />
          {report.insights.provider}
        </span>
      </div>

      <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
        Pega un snippet propio o usa el ejemplo de la demo. Wattless propondrá
        una versión más ligera y alineada con Next.js.
      </p>

      <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,1.05fr)]">
        <div className="space-y-3">
          <div className="flex flex-wrap gap-3">
            <button
              type="button"
              onClick={onUseDemoSnippet}
              className="rounded-full border border-[var(--line)] px-3 py-2 text-sm text-white transition hover:border-[var(--line-strong)]"
            >
              Usar snippet demo
            </button>
            <button
              type="button"
              onClick={onGenerate}
              disabled={isGenerating}
              className="inline-flex items-center gap-2 rounded-full bg-[linear-gradient(135deg,#9bd67e,#d8ff7f)] px-4 py-2 text-sm font-medium text-[#08110d] transition disabled:cursor-not-allowed disabled:opacity-60"
            >
              {isGenerating ? (
                <>
                  <LoaderCircle className="h-4 w-4 animate-spin" />
                  Generando...
                </>
              ) : (
                <>
                  <WandSparkles className="h-4 w-4" />
                  Generar Green Fix
                </>
              )}
            </button>
          </div>

          <label className="block">
            <span className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
              Snippet de entrada
            </span>
            <textarea
              value={code}
              onChange={(event) => onCodeChange(event.target.value)}
              className="mt-3 min-h-[320px] w-full rounded-[1.5rem] border border-[var(--line)] bg-[rgba(5,10,8,0.84)] p-4 font-[var(--font-ibm-plex-mono)] text-sm leading-7 text-[var(--foreground)] outline-none transition focus:border-[var(--accent)]"
              spellCheck={false}
            />
          </label>
        </div>

        <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
          {result ? (
            <div className="space-y-4">
              <div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--accent)]">
                  Resumen
                </div>
                <p className="mt-2 text-sm leading-7 text-[var(--foreground)]">
                  {result.summary}
                </p>
              </div>

              <div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                  Cambios sugeridos
                </div>
                <ul className="mt-2 space-y-2 text-sm leading-6 text-[var(--foreground)]">
                  {result.changes.map((change) => (
                    <li key={change}>- {change}</li>
                  ))}
                </ul>
              </div>

              <div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                  Impacto esperado
                </div>
                <p className="mt-2 text-sm leading-7 text-[var(--accent-strong)]">
                  {result.expected_impact}
                </p>
              </div>

              <div>
                <div className="mono text-xs uppercase tracking-[0.18em] text-[var(--muted)]">
                  Código optimizado
                </div>
                <pre className="mt-3 overflow-x-auto rounded-[1.3rem] border border-[var(--line)] bg-[rgba(5,10,8,0.9)] p-4 text-sm leading-7 text-[var(--foreground)]">
                  <code>{result.optimized_code}</code>
                </pre>
              </div>
            </div>
          ) : (
            <div className="flex min-h-[320px] items-center justify-center rounded-[1.2rem] border border-dashed border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-6 text-center">
              <div>
                <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--accent)]">
                  Esperando snippet
                </p>
                <p className="mt-4 text-sm leading-7 text-[var(--muted)]">
                  Genera un refactor guiado para reforzar el momento wow del
                  pitch. El resultado se puede exportar junto al reporte.
                </p>
              </div>
            </div>
          )}
        </div>
      </div>
    </section>
  );
}
