"use client";

import {
  AnimatePresence,
  LazyMotion,
  domAnimation,
  m,
} from "framer-motion";
import {
  Gauge,
  Globe2,
  Leaf,
  Link2,
  LoaderCircle,
  Sparkles,
  Zap,
} from "lucide-react";
import { startTransition, useEffect, useState, type FormEvent } from "react";

import {
  formatBytes,
  formatGrams,
  formatMilliseconds,
  generateGreenFix,
  scanURL,
} from "@/lib/api";
import type {
  GreenFixResponse,
  ScanReport,
  VampireElement,
} from "@/lib/types";
import { BreakdownBars } from "@/components/breakdown-bars";
import { CompareBanner } from "@/components/compare-banner";
import { GreenFixStudio } from "@/components/green-fix-studio";
import { InsightsPanel } from "@/components/insights-panel";
import { MarkdownReportCard } from "@/components/markdown-report-card";
import { MethodologyCard } from "@/components/methodology-card";
import { MetricCard } from "@/components/metric-card";
import { ScoreRing } from "@/components/score-ring";
import { ScreenshotInspector } from "@/components/screenshot-inspector";
import { VampireList } from "@/components/vampire-list";

const sampleURL = "https://example.com";
const scanProgressLabels = [
  "Midiendo transferencia de red",
  "Estimando coste energético",
  "Identificando carga crítica del LCP",
];
const minimumGreenFixCodeLength = 20;
const maximumGreenFixCodeLength = 20_000;

export function ScanWorkbench() {
  const [inputURL, setInputURL] = useState(sampleURL);
  const [report, setReport] = useState<ScanReport | null>(null);
  const [previousReport, setPreviousReport] = useState<ScanReport | null>(null);
  const [selectedElementID, setSelectedElementID] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isScanning, setIsScanning] = useState(false);
  const [scanProgressIndex, setScanProgressIndex] = useState(0);
  const [greenFixCode, setGreenFixCode] = useState("");
  const [isGeneratingFix, setIsGeneratingFix] = useState(false);
  const [greenFixResult, setGreenFixResult] = useState<GreenFixResponse | null>(null);

  const selectedElement =
    report?.vampire_elements.find((element) => element.id === selectedElementID) ?? null;

  useEffect(() => {
    if (!isScanning) {
      setScanProgressIndex(0);
      return;
    }

    const intervalID = window.setInterval(() => {
      setScanProgressIndex((current) => (current + 1) % scanProgressLabels.length);
    }, 1200);

    return () => window.clearInterval(intervalID);
  }, [isScanning]);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const nextURL = inputURL.trim();
    if (!nextURL) {
      setError("Escribe una URL para empezar el análisis.");
      return;
    }

    const currentReport = report;
    setIsScanning(true);
    setError(null);
    setReport(null);
    setSelectedElementID(null);
    setGreenFixResult(null);

    try {
      const nextReport = await scanURL(nextURL);
      startTransition(() => {
        setPreviousReport(resolveComparablePreviousReport(currentReport, nextReport));
        setReport(nextReport);
        setSelectedElementID(resolvePreferredElement(nextReport)?.id ?? null);
      });
    } catch (submitError) {
      setError(
        submitError instanceof Error ? submitError.message : "El escaneo falló",
      );
    } finally {
      setIsScanning(false);
    }
  }

  async function handleGenerateGreenFix() {
    if (!report) {
      return;
    }

    const trimmedCode = greenFixCode.trim();
    if (trimmedCode.length < minimumGreenFixCodeLength) {
      setGreenFixResult(null);
      setError("Pega al menos 20 caracteres de código para generar un Green Fix útil.");
      return;
    }
    if (trimmedCode.length > maximumGreenFixCodeLength) {
      setGreenFixResult(null);
      setError("Reduce el snippet a 20.000 caracteres o menos para generar el Green Fix.");
      return;
    }

    setIsGeneratingFix(true);
    setError(null);

    try {
      const response = await generateGreenFix({
        framework: "next",
        language: "tsx",
        code: trimmedCode,
        related_resource_id:
          report.insights.top_actions[0]?.related_resource_id ??
          selectedElement?.id,
        report_context: {
          url: report.url,
          score: report.score,
          co2_grams_per_visit: report.co2_grams_per_visit,
          total_bytes_transferred: report.total_bytes_transferred,
          lcp_ms: report.performance.lcp_ms,
          fcp_ms: report.performance.fcp_ms,
        },
      });
      setGreenFixResult(response);
    } catch (generationError) {
      setError(
        generationError instanceof Error
          ? generationError.message
          : "No se pudo generar el Green Fix",
      );
    } finally {
      setIsGeneratingFix(false);
    }
  }

  function handleGreenFixCodeChange(value: string) {
    setGreenFixCode(value);
    setGreenFixResult(null);
  }

  return (
    <LazyMotion features={domAnimation}>
      <section className="space-y-6">
        <section className="panel rounded-[2rem] p-6">
          <div className="grid gap-6 xl:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
            <div>
              <div className="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
                <div className="max-w-3xl">
                  <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                    Lanzar escaneo
                  </p>
                  <h2 className="mt-3 text-3xl font-medium tracking-[-0.05em] text-white sm:text-4xl">
                    Escanea una web y conecta CO2, transferencia y LCP en una sola historia.
                  </h2>
                </div>
                <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                  {isScanning
                    ? scanProgressLabels[scanProgressIndex]
                    : "Listo para analizar"}
                </div>
              </div>

              <form
                className="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1fr)_180px]"
                onSubmit={handleSubmit}
              >
                <label className="block">
                  <span className="sr-only">URL del sitio</span>
                  <input
                    type="text"
                    inputMode="url"
                    value={inputURL}
                    onChange={(event) => setInputURL(event.target.value)}
                    placeholder="https://example.com"
                    autoCapitalize="none"
                    autoCorrect="off"
                    spellCheck={false}
                    className="w-full rounded-[1.35rem] border border-[var(--line)] bg-[rgba(255,255,255,0.03)] px-5 py-4 text-base text-white outline-none transition placeholder:text-[var(--muted)] focus:border-[var(--accent)]"
                  />
                </label>
                <button
                  type="submit"
                  disabled={isScanning}
                  className="inline-flex items-center justify-center gap-2 rounded-[1.35rem] bg-[linear-gradient(135deg,#9bd67e,#d8ff7f)] px-5 py-4 text-sm font-medium uppercase tracking-[0.18em] text-[#08110d] transition hover:scale-[0.99] disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {isScanning ? (
                    <>
                      <LoaderCircle className="h-4 w-4 animate-spin" />
                      Escaneando
                    </>
                  ) : (
                    <>
                      <Sparkles className="h-4 w-4" />
                      Auditar URL
                    </>
                  )}
                </button>
              </form>

              <div className="mt-4 flex flex-wrap items-center gap-3">
                <button
                  type="button"
                  onClick={() => setInputURL(sampleURL)}
                  className="mono rounded-full border border-[var(--line)] px-3 py-1 text-xs uppercase tracking-[0.22em] text-[var(--muted)] transition hover:border-[var(--line-strong)] hover:text-white"
                >
                  Usar ejemplo
                </button>
                {error ? (
                  <span className="rounded-full border border-[rgba(255,126,107,0.3)] bg-[rgba(255,126,107,0.08)] px-3 py-1 text-sm text-[var(--danger)]">
                    {error}
                  </span>
                ) : null}
              </div>
            </div>

            <div className="rounded-[1.7rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-5">
              <p className="mono text-xs uppercase tracking-[0.22em] text-[var(--accent)]">
                Qué mide Wattless
              </p>
              <div className="mt-4 space-y-3 text-sm leading-7 text-[var(--muted)]">
                <p>1. Transferencia real observada durante el runtime.</p>
                <p>2. CO2 estimado por visita con metodología explícita.</p>
                <p>3. Core Web Vitals del render principal: LCP y FCP.</p>
                <p>4. Recursos dominantes, terceros y acciones prioritarias.</p>
              </div>
              <div className="mt-5 grid gap-3 sm:grid-cols-2">
                <div className="rounded-2xl border border-[var(--line)] bg-[var(--panel-muted)] p-4">
                  <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                    Método
                  </div>
                  <div className="mt-2 text-lg text-white">
                    Runtime + Sustainable Web Design
                  </div>
                </div>
                <div className="rounded-2xl border border-[var(--line)] bg-[var(--panel-muted)] p-4">
                  <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                    Entrega
                  </div>
                  <div className="mt-2 text-lg text-white">
                    Informe técnico listo para compartir
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {report && previousReport ? (
          <CompareBanner current={report} previous={previousReport} />
        ) : null}

        <AnimatePresence mode="wait">
          {report ? (
            <m.section
              key={report.url + report.total_bytes_transferred}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.28, ease: "easeOut" }}
              className="grid gap-6"
            >
              <section className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
                <MetricCard
                  label="CO2 por visita"
                  value={formatGrams(report.co2_grams_per_visit)}
                  hint="La métrica principal para contar la historia de sostenibilidad."
                  icon={Leaf}
                />
                <MetricCard
                  label="Transferencia"
                  value={formatBytes(report.total_bytes_transferred)}
                  hint="Peso total capturado durante el runtime del escaneo."
                  icon={Link2}
                />
                <MetricCard
                  label="LCP"
                  value={formatMilliseconds(report.performance.lcp_ms)}
                  hint="Tu render principal y la UX suelen sentirse aquí."
                  icon={Gauge}
                />
                <MetricCard
                  label="Terceros"
                  value={formatBytes(report.summary.third_party_bytes)}
                  hint="Útil para detectar dependencia y variabilidad de red."
                  icon={Globe2}
                />
                <MetricCard
                  label="FCP"
                  value={formatMilliseconds(report.performance.fcp_ms)}
                  hint="Primer contenido visible durante la carga."
                  icon={Zap}
                />
                <MetricCard
                  label="Ahorro potencial"
                  value={formatBytes(report.summary.potential_savings_bytes)}
                  hint="Estimación rápida del peso que puedes atacar primero."
                  icon={Leaf}
                />
                <MetricCard
                  label="Peticiones"
                  value={report.summary.total_requests.toLocaleString("es-CO")}
                  hint={`${report.summary.failed_requests} peticiones fallidas capturadas durante el análisis.`}
                  icon={Link2}
                />
                <MetricCard
                  label="Hosting"
                  value={
                    report.hosting_verdict === "unknown"
                      ? "Desconocido"
                      : report.hosting_is_green
                        ? "Verde"
                        : "No verde"
                  }
                  hint={
                    report.hosted_by
                      ? `Proveedor detectado: ${report.hosted_by}`
                      : "No se pudo resolver el proveedor."
                  }
                  icon={Globe2}
                />
              </section>

              <section className="grid gap-6 xl:grid-cols-[minmax(0,1.08fr)_minmax(350px,0.92fr)]">
                <div className="space-y-6">
                  <InsightsPanel
                    report={report}
                    selectedElementID={selectedElementID}
                    onSelectElement={setSelectedElementID}
                  />
                  <ScreenshotInspector
                    screenshot={report.screenshot}
                    elements={report.vampire_elements}
                    selectedElement={selectedElement}
                    onSelect={setSelectedElementID}
                  />
                  <section className="grid gap-6 lg:grid-cols-2">
                    <BreakdownBars
                      title="Mix de transferencia"
                      subtitle="Peso por tipo de recurso"
                      items={report.breakdown_by_type}
                    />
                    <BreakdownBars
                      title="Propiedad"
                      subtitle="Primera parte vs terceros"
                      items={report.breakdown_by_party}
                    />
                  </section>
                </div>

                <div className="space-y-6">
                  <ScoreRing
                    score={report.score}
                    grams={formatGrams(report.co2_grams_per_visit)}
                  />

                  <MethodologyCard report={report} />

                  <section className="panel rounded-[2rem] p-6">
                    <p className="mono text-xs uppercase tracking-[0.24em] text-[var(--muted)]">
                      Contexto del escaneo
                    </p>
                    <h2 className="mt-3 break-all text-2xl font-medium tracking-[-0.05em] text-white">
                      {report.url}
                    </h2>
                    <div className="mt-5 grid gap-3">
                      <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
                        <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                          DOM Content Loaded
                        </div>
                        <div className="mt-2 text-xl text-white">
                          {formatMilliseconds(
                            report.performance.dom_content_loaded_ms,
                          )}
                        </div>
                      </div>
                      <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
                        <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                          Tiempo de script
                        </div>
                        <div className="mt-2 text-xl text-white">
                          {formatMilliseconds(report.performance.script_duration_ms)}
                        </div>
                      </div>
                      <div className="rounded-[1.5rem] border border-[var(--line)] bg-[rgba(255,255,255,0.02)] p-4">
                        <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--muted)]">
                          Anclajes visuales
                        </div>
                        <div className="mt-2 text-xl text-white">
                          {report.summary.visual_mapped_vampires} /{" "}
                          {report.vampire_elements.length}
                        </div>
                      </div>
                    </div>
                    {report.warnings.length > 0 ? (
                      <div className="mt-5 rounded-[1.5rem] border border-[rgba(255,184,107,0.28)] bg-[rgba(255,184,107,0.08)] p-4">
                        <div className="mono text-xs uppercase tracking-[0.22em] text-[var(--warning)]">
                          Advertencias
                        </div>
                        <ul className="mt-3 space-y-2 text-sm leading-6 text-[var(--foreground)]">
                          {report.warnings.map((warning) => (
                            <li key={warning}>- {warning}</li>
                          ))}
                        </ul>
                      </div>
                    ) : null}
                  </section>

                  <VampireList
                    elements={report.vampire_elements}
                    selectedElementID={selectedElementID}
                    onSelect={setSelectedElementID}
                  />
                </div>
              </section>

              <GreenFixStudio
                report={report}
                code={greenFixCode}
                onCodeChange={handleGreenFixCodeChange}
                onGenerate={handleGenerateGreenFix}
                isGenerating={isGeneratingFix}
                result={greenFixResult}
              />

              <MarkdownReportCard report={report} greenFix={greenFixResult} />
            </m.section>
          ) : (
            <m.section
              key={isScanning ? "loading" : "empty"}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              className="panel rounded-[2rem] border-dashed p-8 text-center"
            >
              <p className="mono text-xs uppercase tracking-[0.26em] text-[var(--accent)]">
                {isScanning ? "Escaneo en curso" : "Sin informe todavía"}
              </p>
              <h3 className="mt-4 text-3xl font-medium tracking-[-0.05em] text-white">
                {isScanning
                  ? scanProgressLabels[scanProgressIndex]
                  : "Ejecuta el primer análisis para poblar el dashboard."}
              </h3>
              <p className="mx-auto mt-4 max-w-2xl text-sm leading-7 text-[var(--muted)]">
                {isScanning
                  ? "Wattless está calculando CO2 por visita, LCP, FCP, hosting y las oportunidades de optimización más visibles."
                  : "El informe combina score, bytes, Core Web Vitals, screenshot, hosting, metodología explícita, insights IA y un Green Fix orientado a código real."}
              </p>
            </m.section>
          )}
        </AnimatePresence>
      </section>
    </LazyMotion>
  );
}

function resolvePreferredElement(report: ScanReport): VampireElement | null {
  const action = report.insights.top_actions[0];
  if (action) {
    const matching = report.vampire_elements.find(
      (element) => element.id === action.related_resource_id,
    );
    if (matching) {
      return matching;
    }
  }
  return report.vampire_elements[0] ?? null;
}

function resolveComparablePreviousReport(
  currentReport: ScanReport | null,
  nextReport: ScanReport,
): ScanReport | null {
  if (!currentReport) {
    return null;
  }

  return currentReport.url === nextReport.url ? currentReport : null;
}
