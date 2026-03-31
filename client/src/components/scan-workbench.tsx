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
import type { ScanJobStatus, ScanReport } from "@/lib/types";
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

function formatRenderMetric(value: number, complete: boolean): string {
  return complete ? formatMilliseconds(value) : "N/D";
}

function renderMetricProgress(value: number, complete: boolean): number {
  if (!complete) {
    return 30;
  }
  return Math.max(10, 100 - Math.min(value / 40, 90));
}

export function ScanWorkbench() {
  const {
    inputURL,
    setInputURL,
    report,
    previousReport,
    selectedElementID,
    setSelectedElementID,
    selectionSignal,
    selectedElement,
    scanError,
    isScanning,
    scanProgressIndex,
    handleSubmit,
    jobStatus,
    queuePosition,
    estimatedWaitSeconds,
    submittedURL,
    reportJobId,
    conflictingJob,
    resumeConflictingJob,
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
              <label htmlFor="scan-url-input" className="sr-only">URL a analizar</label>
              <input
                id="scan-url-input"
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
            
            {!report && isScanning && (
               <div className="mt-6 flex flex-wrap items-center justify-center gap-3 text-xs uppercase tracking-widest font-label">
                 <span className="bg-surface-container-highest text-on-surface px-3 py-1.5 rounded-full border border-outline-variant/20">
                   {formatProgressBadgeLabel(
                     isScanning,
                     jobStatus,
                     queuePosition,
                     scanProgressIndex,
                   )}
                 </span>
               </div>
            )}
            
            {scanError && (
               <div className="mx-auto mt-6 max-w-2xl rounded-xl bg-error-container/20 px-4 py-3 text-sm leading-6 text-error border border-error/20">
                 <p>{scanError}</p>
                 {conflictingJob ? (
                   <button
                     type="button"
                     onClick={resumeConflictingJob}
                     className="mt-3 inline-flex items-center rounded-lg bg-error text-on-error px-3 py-2 text-xs font-bold uppercase tracking-wide transition-colors hover:opacity-90"
                   >
                     Reanudar turno actual
                   </button>
                 ) : null}
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
                <section className="grid grid-cols-1 xl:grid-cols-[1.2fr_1fr_1fr] gap-4 lg:gap-6">
                  <div className="xl:col-span-1 flex flex-col h-full w-full">
                    <ScoreRing
                      score={report.score}
                      grams={formatGrams(report.co2_grams_per_visit)}
                    />
                  </div>
                  
                  <div className="xl:col-span-2 grid grid-cols-1 sm:grid-cols-2 gap-4 lg:gap-6 h-full w-full">
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
                      value={formatRenderMetric(report.performance.lcp_ms, report.performance.render_metrics_complete)}
                      caption={`FCP ${formatRenderMetric(report.performance.fcp_ms, report.performance.render_metrics_complete)} · Long Tasks ${formatMilliseconds(report.performance.long_tasks_total_ms)}`}
                      hint="Render crítico y presión real de CPU."
                      progress={renderMetricProgress(report.performance.lcp_ms, report.performance.render_metrics_complete)}
                      icon={Gauge}
                    />
                  </div>

                  {/* Segunda fila del Bento: Evidence Strip y Summary */}
                  <div className="xl:col-span-3 w-full">
                    <AuditEvidenceStrip report={report} />
                  </div>
                </section>

                {report.analysis.findings.length > 0 ? (
                  <FindingsPanel findings={report.analysis.findings} />
                ) : null}

                <section
                  id="diagnostic"
                  className="grid grid-cols-1 lg:grid-cols-[1.45fr_1fr] gap-8 xl:gap-12 pt-2"
                >
                  <div className="relative">
                    <div className="sticky top-8">
                      {reportJobId ? (
                        <ScreenshotInspector
                          jobId={reportJobId}
                          screenshot={report.screenshot}
                          elements={report.vampire_elements}
                          selectedElement={selectedElement}
                          selectionSignal={selectionSignal}
                          onSelect={setSelectedElementID}
                        />
                      ) : null}
                    </div>
                  </div>

                  <div className="flex flex-col gap-6 w-full lg:pt-8">
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
                        <div className="flex flex-col gap-8 lg:gap-10">
                          {/* ROW 1: Network & Payload vs CPU Lab */}
                          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4 lg:gap-6">
                            
                            {/* COL 1: Network & Payload */}
                            <div className="bg-surface-container-high rounded-3xl p-6 border border-outline-variant/10 flex flex-col h-full">
                              <div className="text-primary text-[10px] uppercase tracking-widest font-label font-bold mb-4">
                                Network & Payload
                              </div>
                              <div className="grid grid-cols-2 gap-3 h-full">
                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Transferencia Observada
                                  </div>
                                  <div className="mt-1 text-2xl font-headline font-bold text-on-surface">
                                    {formatBytes(report.total_bytes_transferred)}
                                  </div>
                                  <div className="text-xs opacity-70 mt-1.5 flex gap-2 font-medium">
                                     <span className="text-success">{report.summary.successful_requests} OK</span>
                                     {report.summary.failed_requests > 0 && <span className="text-error">{report.summary.failed_requests} FO</span>}
                                  </div>
                                </div>

                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Ahorro Potencial
                                  </div>
                                  <div className="mt-1 text-2xl font-headline font-bold text-on-surface text-primary">
                                    {formatBytes(report.summary.potential_savings_bytes)}
                                  </div>
                                  <div className="text-xs opacity-70 mt-1.5">
                                     {(report.summary.potential_savings_bytes / Math.max(report.total_bytes_transferred, 1) * 100).toFixed(1)}% del total
                                  </div>
                                </div>

                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Tiempos Base
                                  </div>
                                  <div className="mt-1 text-xl font-headline font-bold text-on-surface flex items-baseline gap-1">
                                    {formatMilliseconds(report.performance.dom_content_loaded_ms)} <span className="text-[10px] font-normal text-on-surface-variant uppercase">DCL</span>
                                  </div>
                                  <div className="text-xs opacity-70 mt-1.5">
                                    Load: {formatMilliseconds(report.performance.load_ms)}
                                  </div>
                                </div>

                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5 relative overflow-hidden">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Infraestructura
                                  </div>
                                  <div className="mt-1.5 text-sm font-headline font-bold text-on-surface truncate">
                                    {formatHostingLabel(report)}
                                  </div>
                                  <div className="text-[11px] opacity-70 truncate mt-1">
                                    {report.hosted_by || "Proveedor no detectado"}
                                  </div>
                                  {report.hosting_is_green && (
                                    <Leaf className="w-12 h-12 text-success/[0.08] absolute -bottom-3 -right-3 rotate-12" />
                                  )}
                                </div>
                              </div>
                            </div>

                            {/* COL 2: Performance Lab & CPU */}
                            <div className="bg-surface-container-high rounded-3xl p-6 border border-outline-variant/10 flex flex-col h-full">
                              <div className="text-primary text-[10px] uppercase tracking-widest font-label font-bold mb-4">
                                Performance Lab & CPU
                              </div>
                              <div className="grid grid-cols-2 gap-3 h-full">
                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Métricas Críticas
                                  </div>
                                  <div className="mt-1 flex items-baseline gap-2">
                                    <span className="text-2xl font-headline font-bold text-on-surface">
                                      {formatRenderMetric(report.performance.lcp_ms, report.performance.render_metrics_complete)}
                                    </span>
                                    <span className="text-[10px] text-on-surface-variant font-bold uppercase tracking-wide">LCP</span>
                                  </div>
                                  <div className="text-xs opacity-70 mt-1.5">
                                    FCP: {formatRenderMetric(report.performance.fcp_ms, report.performance.render_metrics_complete)}
                                  </div>
                                </div>

                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                                    Bloqueo de CPU
                                  </div>
                                  <div className="mt-1 flex items-baseline gap-2">
                                    <span className="text-2xl font-headline font-bold text-on-surface text-error">
                                      {formatMilliseconds(report.performance.long_tasks_total_ms)}
                                    </span>
                                    <span className="text-[10px] text-on-surface-variant font-bold uppercase tracking-wide">Long</span>
                                  </div>
                                  <div className="text-xs opacity-70 mt-1.5 truncate" title={`JS: ${formatMilliseconds(report.performance.script_resource_duration_ms)}`}>
                                    {report.performance.long_tasks_count} tareas · JS: {formatMilliseconds(report.performance.script_resource_duration_ms)}
                                  </div>
                                </div>

                                <div className="col-span-2 bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                  <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label mb-2 flex justify-between">
                                    <span>Identidad del LCP (Largest Contentful Paint)</span>
                                    <span>{report.performance.lcp_size ? formatBytes(report.performance.lcp_size) : ""}</span>
                                  </div>
                                  <div className="text-sm font-medium text-on-surface break-words font-mono bg-surface-container-highest p-2.5 rounded-xl text-xs border border-outline-variant/10">
                                    {report.performance.lcp_selector_hint || (report.performance.lcp_resource_tag ? `<${report.performance.lcp_resource_tag}> (Sin selector estricto)` : "Selector no identificado por agente")}
                                  </div>
                                  {report.performance.lcp_resource_url && (
                                    <div className="text-[11px] opacity-70 mt-2 truncate w-full">
                                      URL: {report.performance.lcp_resource_url}
                                    </div>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>

                          {/* ROW 2: Warnings (if any) */}
                          {report.warnings.length > 0 && (
                            <div className="rounded-3xl bg-error-container/10 p-6 border border-error-container/20">
                              <div className="text-error font-bold text-[10px] uppercase tracking-widest font-label">
                                Advertencias Críticas
                              </div>
                              <ul className="mt-3 space-y-2 text-sm leading-6 text-on-surface-variant">
                                {report.warnings.map((warning, index) => (
                                  <li key={`warning-${index}`}>- {warning}</li>
                                ))}
                              </ul>
                            </div>
                          )}

                          {/* ROW 3: Breakdowns and Dimensions */}
                          <div className="grid grid-cols-1 xl:grid-cols-[1.4fr_1fr] gap-4 lg:gap-6">
                            <div className="bg-surface-container-high rounded-3xl p-6 border border-outline-variant/10 flex flex-col h-full">
                               <div className="text-primary text-[10px] uppercase tracking-widest font-label font-bold mb-4">
                                Distribución Dimensional
                              </div>
                              
                              <div className="grid grid-cols-2 gap-3 mb-6">
                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5">
                                   <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Viewport Fold</div>
                                   <div className="mt-3 flex items-center gap-4">
                                     <div className="h-16 w-3 bg-surface-container-highest rounded-full relative overflow-hidden flex flex-col justify-end border border-outline-variant/10">
                                        <div 
                                          className="w-full bg-primary" 
                                          style={{height: `${(report.analysis.summary.below_fold_bytes / Math.max(report.analysis.summary.above_fold_visual_bytes + report.analysis.summary.below_fold_bytes, 1)) * 100}%`}}
                                        ></div>
                                     </div>
                                     <div className="flex flex-col gap-2">
                                       <div className="leading-none">
                                         <div className="text-[15px] font-bold font-headline text-on-surface">{formatBytes(report.analysis.summary.above_fold_visual_bytes)}</div>
                                         <div className="text-[10px] text-on-surface-variant uppercase mt-0.5">Above Fold</div>
                                       </div>
                                       <div className="leading-none">
                                         <div className="text-[15px] font-bold font-headline text-primary">{formatBytes(report.analysis.summary.below_fold_bytes)}</div>
                                         <div className="text-[10px] text-on-surface-variant uppercase mt-0.5">Below Fold (Oculto)</div>
                                       </div>
                                     </div>
                                   </div>
                                </div>

                                <div className="bg-surface-container rounded-2xl p-4 border border-outline-variant/5 flex flex-col justify-center">
                                   <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">Render Critical vs Total</div>
                                   <div className="mt-2 text-2xl font-headline font-bold text-on-surface">
                                      {formatBytes(report.analysis.summary.render_critical_bytes)}
                                   </div>
                                   <div className="text-[10px] text-on-surface-variant uppercase mt-1">Crítico Bloqueante</div>
                                   <div className="w-full bg-surface-container-highest rounded-full h-1.5 mt-4 overflow-hidden border border-outline-variant/10">
                                     <div className="bg-error h-1.5 rounded-full" style={{width: `${Math.min(100, (report.analysis.summary.render_critical_bytes / Math.max(report.total_bytes_transferred, 1)) * 100)}%`}}></div>
                                   </div>
                                </div>
                              </div>

                              <div className="bento-grid lg:grid-cols-2 mt-auto">
                                <BreakdownBars
                                  title="Breakdown by type"
                                  subtitle="Peso por recurso"
                                  items={report.breakdown_by_type}
                                />
                                <BreakdownBars
                                  title="Breakdown by party"
                                  subtitle="Propiedad"
                                  items={report.breakdown_by_party}
                                />
                              </div>
                            </div>
                            
                            <MethodologyCard report={report} />
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
                {getLoadingHeading(isScanning, jobStatus, queuePosition, scanProgressIndex)}
              </h2>
              <p className="mt-4 max-w-3xl text-sm leading-relaxed text-on-surface-variant text-center sm:text-left mx-auto sm:mx-0">
                {getLoadingDescription(
                  isScanning,
                  jobStatus,
                  queuePosition,
                  estimatedWaitSeconds,
                  submittedURL,
                )}
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

function formatProgressBadgeLabel(
  isScanning: boolean,
  jobStatus: ScanJobStatus | null,
  queuePosition: number | null,
  scanProgressIndex: number,
): string {
  if (!isScanning) {
    return "Listo para analizar";
  }

  if (jobStatus === "queued") {
    return queuePosition && queuePosition > 0 ? `Turno #${queuePosition}` : "Turno en cola";
  }

  if (jobStatus === "scanning") {
    return scanProgressLabels[scanProgressIndex];
  }

  return "Preparando turno";
}

function getLoadingHeading(
  isScanning: boolean,
  jobStatus: ScanJobStatus | null,
  queuePosition: number | null,
  scanProgressIndex: number,
): string {
  if (!isScanning) {
    return "Ejecuta un análisis y el espacio de trabajo ensamblará el reporte.";
  }

  if (jobStatus === "queued") {
    if (queuePosition === 1) {
      return "Turno #1";
    }

    if (queuePosition && queuePosition > 1) {
      return `Turno #${queuePosition}`;
    }

    return "Tu análisis está en cola";
  }

  if (jobStatus === "scanning") {
    return scanProgressLabels[scanProgressIndex];
  }

  return "Preparando tu turno";
}

function getLoadingDescription(
  isScanning: boolean,
  jobStatus: ScanJobStatus | null,
  queuePosition: number | null,
  estimatedWaitSeconds: number | null,
  submittedURL: string | null,
): string {
  if (!isScanning) {
    return "El reporte completo fluye de manera progresiva: empieza por el score global, separa evidencia above/below the fold y termina priorizando hallazgos accionables.";
  }

  if (jobStatus === "queued") {
    const waitLabel =
      estimatedWaitSeconds && estimatedWaitSeconds > 0
        ? ` Espera estimada: ${formatWaitTime(estimatedWaitSeconds)}.`
        : "";
    const subject = submittedURL ? `${submittedURL} quedó en cola.` : "Tu análisis quedó en cola.";

    if (queuePosition === 1) {
      return `${subject} Ya eres el siguiente en la fila.${waitLabel}`;
    }

    if (queuePosition && queuePosition > 1) {
      return `${subject} Posición actual: ${queuePosition}.${waitLabel}`;
    }

    return `${subject}${waitLabel}`;
  }

  return "Wattless está emulando un perfil moderno para recolectar métricas de transferencia de red, CPU throttling e hitos de render visual para producir un dictamen preciso.";
}

function formatWaitTime(seconds: number): string {
  if (seconds < 60) {
    return `${seconds}s`;
  }

  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = seconds % 60;
  if (remainingSeconds === 0) {
    return `${minutes} min`;
  }

  return `${minutes} min ${remainingSeconds}s`;
}
