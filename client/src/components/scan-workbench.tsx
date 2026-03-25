"use client";

import {
  AnimatePresence,
  LazyMotion,
  domAnimation,
  m,
} from "framer-motion";
import {
  ChevronDown,
  Gauge,
  Leaf,
  LoaderCircle,
  ScanSearch,
} from "lucide-react";
import { useState } from "react";

import { BreakdownBars } from "@/components/breakdown-bars";
import { CompareBanner } from "@/components/compare-banner";
import { AuditEvidenceStrip } from "@/components/audit-evidence-strip";
import { FindingsPanel } from "@/components/findings-panel";
import { InsightsPanel } from "@/components/insights-panel";
import { MarkdownReportCard } from "@/components/markdown-report-card";
import { MethodologyCard } from "@/components/methodology-card";
import { MetricCard } from "@/components/metric-card";
import { ScreenshotInspector } from "@/components/screenshot-inspector";
import { ScoreRing } from "@/components/score-ring";
import { VampireList } from "@/components/vampire-list";
import { formatBytes, formatGrams, formatMilliseconds } from "@/lib/api";
import type { ScanReport } from "@/lib/types";
import { useAudit, sampleURL, scanProgressLabels } from "@/hooks/use-audit";

const emptyStateHighlights = [
  {
    id: "empty-diagnostic",
    title: "Diagnóstico",
    description: "Inspector visual, activos dominantes y jerarquía de impacto real.",
  },
  {
    id: "findings",
    title: "Findings",
    description: "Hallazgos con severidad, confianza y evidencia para decidir qué arreglar primero.",
  },
  {
    id: "methodology",
    title: "Método",
    description: "Trazabilidad técnica transparente después del escaneo.",
  },
];

export function ScanWorkbench() {
  const {
    inputURL,
    setInputURL,
    report,
    previousReport,
    selectedElementID,
    setSelectedElementID,
    selectedElement,
    scanError,
    isScanning,
    scanProgressIndex,
    handleSubmit,
  } = useAudit();

  const [isTechnicalDetailsOpen, setIsTechnicalDetailsOpen] = useState(false);

  return (
    <LazyMotion features={domAnimation}>
      <section className="flex flex-col gap-12 w-full">
        {/* Scanner Form Section - Collapses dynamically */}
        <m.section
          layout
          id="scanner"
          className={`flex flex-col items-center text-center mx-auto w-full transition-all duration-700 ease-in-out ${
            report
              ? "mt-0 max-w-3xl"
              : "mt-12 sm:mt-24 space-y-8 max-w-2xl"
          }`}
        >
          {/* Header/Title shrinks if report exists */}
          {!report && (
            <m.div layoutId="scanner-header" className="w-full">
              <h1 className="text-4xl md:text-5xl lg:text-6xl font-bold tracking-tight mb-4 text-on-surface font-headline">
                Wattless Audit
              </h1>
              <p className="text-on-surface-variant max-w-lg mx-auto mb-10 leading-relaxed text-sm sm:text-base">
                Mide tu huella de carbono digital, analiza activos críticos y optimiza 
                render con precisión clínica en un solo flujo de trabajo.
              </p>
            </m.div>
          )}

          <div className="w-full">
            <form className="relative w-full" onSubmit={handleSubmit}>
              <div className="absolute inset-y-0 left-4 flex items-center pointer-events-none">
                <ScanSearch className="h-5 w-5 text-outline" />
              </div>
              <input
                type="text"
                inputMode="url"
                value={inputURL}
                onChange={(event) => setInputURL(event.target.value)}
                placeholder="https://example.com"
                autoCapitalize="none"
                autoCorrect="off"
                spellCheck={false}
                className={`w-full bg-surface-container-highest border-0 border-b-2 border-outline-variant focus:border-primary focus:ring-0 rounded-t-xl pl-12 pr-32 font-body transition-all outline-none text-on-surface placeholder-on-surface-variant ${
                  report ? "py-4 text-base" : "py-5 text-lg"
                }`}
              />
              <button
                type="submit"
                disabled={isScanning}
                className="absolute right-2 top-2 bottom-2 bg-primary text-on-primary px-5 rounded-lg font-bold hover:bg-primary-dim transition-colors disabled:opacity-60 disabled:cursor-not-allowed flex items-center gap-2 text-sm"
              >
                {isScanning ? (
                  <>
                    <LoaderCircle className="h-4 w-4 animate-spin" />
                    Analizando
                  </>
                ) : (
                  "Analyze"
                )}
              </button>
            </form>
            
            {!report && (
               <div className="mt-6 flex flex-wrap items-center justify-center gap-3 text-xs uppercase tracking-widest font-label">
                 <span className="bg-surface-container-highest text-on-surface px-3 py-1.5 rounded-full border border-outline-variant/20">
                   {isScanning
                     ? scanProgressLabels[scanProgressIndex]
                     : "Listo para analizar"}
                 </span>
                 <button
                   type="button"
                   onClick={() => setInputURL(sampleURL)}
                   className="bg-surface-container-highest text-on-surface px-3 py-1.5 rounded-full border border-outline-variant/20 hover:bg-surface-container-high transition-colors"
                 >
                   Usar example.com
                 </button>
               </div>
            )}
            
            {scanError && (
               <div className="mx-auto mt-6 max-w-2xl rounded-xl bg-error-container/20 px-4 py-3 text-sm leading-6 text-error border border-error/20">
                 {scanError}
               </div>
            )}
          </div>
        </m.section>

        {/* Audit Results / Funnel Layout */}
        <AnimatePresence mode="wait">
          {report ? (
            <m.section
              key={report.url + report.total_bytes_transferred}
              initial={{ opacity: 0, y: 16 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.3, ease: "easeOut" }}
              className="flex flex-col gap-12"
            >
              {previousReport ? (
                <CompareBanner current={report} previous={previousReport} />
              ) : null}

              <div className="space-y-8">
                <section className="grid grid-cols-1 md:grid-cols-3 gap-4 lg:gap-6">
                  <ScoreRing
                    score={report.score}
                    grams={formatGrams(report.co2_grams_per_visit)}
                  />
                  <MetricCard
                    label="Payload size"
                    value={formatBytes(report.total_bytes_transferred)}
                    caption={`${report.summary.total_requests.toLocaleString("es-CO")} requests · ${formatHostingLabel(report)}`}
                    hint="Transferencia observada durante la visita sintética."
                    progress={Math.min(
                      100,
                      (report.total_bytes_transferred /
                        Math.max(
                          report.total_bytes_transferred +
                            report.summary.potential_savings_bytes,
                          1,
                        )) *
                        100,
                    )}
                    icon={Leaf}
                  />
                  <MetricCard
                    label="Performance"
                    value={formatMilliseconds(report.performance.lcp_ms)}
                    caption={`FCP ${formatMilliseconds(report.performance.fcp_ms)} · Long Tasks ${formatMilliseconds(report.performance.long_tasks_total_ms)}`}
                    hint="Render crítico y presión real de CPU."
                    progress={Math.max(
                      10,
                      100 - Math.min(report.performance.lcp_ms / 40, 90),
                    )}
                    icon={Gauge}
                  />
                </section>

                <AuditEvidenceStrip report={report} />

                {report.analysis.findings.length > 0 ? (
                  <FindingsPanel findings={report.analysis.findings} />
                ) : null}

                <section
                  id="diagnostic"
                  className="grid grid-cols-1 lg:grid-cols-[1.45fr_1fr] gap-8 xl:gap-12 pt-2"
                >
                  <div className="relative">
                    <div className="sticky top-8">
                      <ScreenshotInspector
                        screenshot={report.screenshot}
                        elements={report.vampire_elements}
                        selectedElement={selectedElement}
                        onSelect={setSelectedElementID}
                      />
                    </div>
                  </div>

                  <div className="flex flex-col gap-8">
                    <VampireList
                      elements={report.vampire_elements}
                      selectedElementID={selectedElementID}
                      capturedHeight={report.screenshot.captured_height}
                      onSelect={setSelectedElementID}
                    />
                    <InsightsPanel
                      report={report}
                      selectedElementID={selectedElementID}
                      onSelectElement={setSelectedElementID}
                    />
                  </div>
                </section>
              </div>

              {/* SECTION 4: Technical Evidence (Accordion to reduce cognitive load) */}
              <section className="bg-surface-container-low rounded-[2rem] border border-outline-variant/10 overflow-hidden mt-8">
                <button
                  type="button"
                  onClick={() => setIsTechnicalDetailsOpen(!isTechnicalDetailsOpen)}
                  className="w-full px-8 py-6 flex items-center justify-between text-left hover:bg-surface-container transition-colors group"
                >
                  <div className="flex items-center gap-4">
                     <div className={`p-2 rounded-full bg-surface-container-highest transition-colors group-hover:bg-primary/20 ${isTechnicalDetailsOpen ? "bg-primary/20" : ""}`}>
                        <ChevronDown className={`w-5 h-5 text-on-surface-variant transition-transform duration-300 ${isTechnicalDetailsOpen ? "rotate-180 text-primary" : ""}`} />
                     </div>
                     <div>
                       <h3 className="text-xl font-bold font-headline text-on-surface">Apéndice Técnico y Evidencia</h3>
                       <p className="text-sm text-on-surface-variant mt-1">
                         Breakdowns de métricas crudas, pesos en disco y supuestos metodológicos de medición.
                       </p>
                     </div>
                  </div>
                </button>
                <AnimatePresence>
                  {isTechnicalDetailsOpen && (
                    <m.div
                      initial={{ height: 0, opacity: 0 }}
                      animate={{ height: "auto", opacity: 1 }}
                      exit={{ height: 0, opacity: 0 }}
                      className="overflow-hidden"
                    >
                      <div className="p-8 pt-4 border-t border-outline-variant/10">
                        <div className="flex flex-col gap-12">
                          <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-8">
                            <div className="max-w-xl">
                              <p className="text-on-surface-variant text-xs uppercase tracking-widest font-label mb-2">
                                Métricas crudas
                              </p>
                              <h4 className="text-2xl font-bold font-headline text-on-surface">
                                Tiempos base y Anclajes
                              </h4>
                            </div>

                            <div className="grid gap-4 sm:grid-cols-2 lg:w-[32rem]">
                              <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                  Transferencia
                                </div>
                                <div className="mt-1 text-lg font-headline font-bold text-on-surface">
                                  {formatBytes(report.total_bytes_transferred)}
                                </div>
                                <div className="text-xs opacity-70">
                                  {report.summary.total_requests.toLocaleString("es-CO")} peticiones
                                </div>
                              </div>
                              <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                  Tiempos
                                </div>
                                <div className="mt-1 text-lg font-headline font-bold text-on-surface">
                                  {formatMilliseconds(report.performance.dom_content_loaded_ms)} DCL
                                </div>
                                <div className="text-xs opacity-70">
                                  Load {formatMilliseconds(report.performance.load_ms)}
                                </div>
                              </div>
                              <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                  Anclajes Visibles
                                </div>
                                <div className="mt-1 text-lg font-headline font-bold text-on-surface">
                                  {report.summary.visual_mapped_vampires} / {report.vampire_elements.length}
                                </div>
                              </div>
                              <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                  Hosting
                                </div>
                                <div className="mt-1 text-sm font-headline font-bold text-on-surface truncate">
                                  {formatHostingLabel(report)}
                                </div>
                                <div className="text-xs opacity-70 truncate px-1">
                                  {report.hosted_by || "No documentado"}
                                </div>
                              </div>
                            </div>
                          </div>

                          {report.warnings.length > 0 && (
                            <div className="rounded-2xl bg-error-container/10 p-5 border border-error-container/20">
                              <div className="text-error font-bold text-sm uppercase tracking-wider font-label">
                                Advertencias Críticas
                              </div>
                              <ul className="mt-3 space-y-2 text-sm leading-6 text-on-surface-variant">
                                {report.warnings.map((warning) => (
                                  <li key={warning}>- {warning}</li>
                                ))}
                              </ul>
                            </div>
                          )}

                          <div className="bento-grid xl:grid-cols-[minmax(0,0.88fr)_minmax(0,1.12fr)]">
                            <MethodologyCard report={report} />
                            <div className="bento-grid lg:grid-cols-2">
                              <BreakdownBars
                                title="Breakdown by type"
                                subtitle="Peso por recurso"
                                items={report.breakdown_by_type}
                              />
                              <BreakdownBars
                                title="Breakdown by party"
                                subtitle="1ra vs 3ra parte"
                                items={report.breakdown_by_party}
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    </m.div>
                  )}
                </AnimatePresence>
              </section>

              {/* SECTION 5: Export (End of Funnel) */}
              <MarkdownReportCard report={report} />
            </m.section>
          ) : (
            <m.section
              key={isScanning ? "loading" : "empty"}
              initial={{ opacity: 0, scale: 0.98 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0 }}
              className="bg-surface-container-low rounded-[2rem] p-10 border border-outline-variant/5 mt-0 w-full"
            >
              <p className="text-primary text-xs uppercase tracking-widest font-label font-bold flex items-center justify-center sm:justify-start gap-2">
                <span className="flex h-6 w-6 items-center justify-center rounded-full bg-primary/10">
                   {isScanning ? <LoaderCircle className="w-3 h-3 animate-spin"/> : <Leaf className="w-3 h-3 animate-none"/>}
                </span>
                {isScanning ? "Análisis en progreso" : "Auditoría en reposo"}
              </p>
              <h2 className="mt-5 text-2xl sm:text-3xl font-headline font-bold text-center sm:text-left text-on-surface">
                {isScanning
                  ? scanProgressLabels[scanProgressIndex]
                  : "Ejecuta un análisis y el espacio de trabajo ensamblará el reporte."}
              </h2>
              <p className="mt-4 max-w-3xl text-sm leading-relaxed text-on-surface-variant text-center sm:text-left mx-auto sm:mx-0">
                {isScanning
                  ? "Wattless está emulando un perfil moderno para recolectar métricas de transferencia de red, CPU throttling e hitos de render visual para producir un dictamen preciso."
                  : "El reporte completo fluye de manera progresiva: empieza por el score global, separa evidencia above/below the fold y termina priorizando hallazgos accionables."}
              </p>

              <div className="mt-10 grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
                {emptyStateHighlights.map((highlight) => (
                  <div
                    key={highlight.id}
                    id={highlight.id}
                    className="bg-surface-container px-6 py-5 rounded-2xl border border-outline-variant/10 text-center sm:text-left"
                  >
                    <p className="font-bold text-on-surface text-sm uppercase tracking-wider font-label">
                      {highlight.title}
                    </p>
                    <p className="mt-3 text-sm leading-relaxed text-on-surface-variant">
                      {highlight.description}
                    </p>
                  </div>
                ))}
              </div>
            </m.section>
          )}
        </AnimatePresence>
      </section>
    </LazyMotion>
  );
}

function formatHostingLabel(report: ScanReport): string {
  if (report.hosting_verdict === "unknown") return "Hosting desconocido";
  return report.hosting_is_green ? "Hosting verde" : "Hosting no verde";
}
