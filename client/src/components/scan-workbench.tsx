"use client";

import {
  AnimatePresence,
  LazyMotion,
  domAnimation,
  m,
} from "framer-motion";
import {
  Gauge,
  Leaf,
  LoaderCircle,
  ScanSearch,
} from "lucide-react";

import { BreakdownBars } from "@/components/breakdown-bars";
import { CompareBanner } from "@/components/compare-banner";
import { GreenFixStudio } from "@/components/green-fix-studio";
import { InsightsPanel } from "@/components/insights-panel";
import { MarkdownReportCard } from "@/components/markdown-report-card";
import { MethodologyCard } from "@/components/methodology-card";
// We will also need to update MetricCard and others subsequently
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
    description: "Inspector visual, activos dominantes y prioridades de mejora.",
  },
  {
    id: "methodology",
    title: "Método",
    description: "Trazabilidad, fórmula y supuestos visibles después del escaneo.",
  },
  {
    id: "green-fix",
    title: "Green Fix",
    description: "Refactor guiado para un snippet real cuando el informe esté listo.",
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
    greenFixError,
    isScanning,
    scanProgressIndex,
    greenFixCode,
    isGeneratingFix,
    greenFixResult,
    handleSubmit,
    handleGenerateGreenFix,
    handleGreenFixCodeChange,
  } = useAudit();

  return (
    <LazyMotion features={domAnimation}>
      <section className="flex flex-col gap-16 w-full">
        {/* Scanner Input Section */}
        <section
          id="scanner"
          className="flex flex-col items-center text-center space-y-8 mt-8"
        >
          <div className="max-w-2xl w-full">
            <h1 className="text-4xl md:text-6xl font-bold tracking-tight mb-4 text-on-surface">
              Wattless Audit
            </h1>
            <p className="text-on-surface-variant max-w-lg mx-auto mb-10">
              Measure your digital carbon footprint with precision. Enter a URL
              to start the biome analysis.
            </p>

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
                className="w-full bg-surface-container-highest border-0 border-b-2 border-outline-variant focus:border-primary focus:ring-0 rounded-t-xl py-5 pl-12 pr-32 text-lg font-body transition-all outline-none text-on-surface placeholder-on-surface-variant"
              />
              <button
                type="submit"
                disabled={isScanning}
                className="absolute right-2 top-2 bottom-2 bg-primary text-on-primary px-6 rounded-lg font-bold hover:bg-primary-dim transition-colors disabled:opacity-60 disabled:cursor-not-allowed flex items-center gap-2"
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

            {scanError ? (
              <div className="mx-auto mt-6 max-w-2xl rounded-xl bg-error-container/20 px-4 py-3 text-sm leading-6 text-error border border-error/20">
                {scanError}
              </div>
            ) : null}
          </div>
        </section>

        <AnimatePresence mode="wait">
          {report ? (
            <m.section
              key={report.url + report.total_bytes_transferred}
              initial={{ opacity: 0, y: 12 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -8 }}
              transition={{ duration: 0.28, ease: "easeOut" }}
              className="flex flex-col gap-16"
            >
              {/* KPI Cards Grid */}
              <section className="grid grid-cols-1 md:grid-cols-3 gap-6">
                <ScoreRing
                  score={report.score}
                  grams={formatGrams(report.co2_grams_per_visit)}
                />
                
                {/* Temporary replacement until metric-card is refactored, or leaving as is for now depending on internal metric-card implementation */}
                <MetricCard
                  label="Payload size"
                  value={formatBytes(report.total_bytes_transferred)}
                  caption={`${report.summary.total_requests.toLocaleString(
                    "es-CO"
                  )} requests · ${formatHostingLabel(report)}`}
                  hint="Perfil de transferencia observado durante el runtime."
                  progress={Math.min(
                    100,
                    (report.total_bytes_transferred /
                      Math.max(
                        report.total_bytes_transferred +
                          report.summary.potential_savings_bytes,
                        1
                      )) *
                      100
                  )}
                  icon={Leaf}
                />
                <MetricCard
                  label="Performance"
                  value={formatMilliseconds(report.performance.lcp_ms)}
                  caption={`FCP ${formatMilliseconds(report.performance.fcp_ms)}`}
                  hint="Lectura rápida de carga crítica."
                  progress={Math.max(
                    10,
                    100 - Math.min(report.performance.lcp_ms / 40, 90)
                  )}
                  icon={Gauge}
                />
              </section>

              {previousReport ? (
                <CompareBanner current={report} previous={previousReport} />
              ) : null}

              {/* Visual Inspector & Assets Bento */}
              <section
                id="diagnostic"
                className="grid grid-cols-1 lg:grid-cols-5 gap-8"
              >
                <div className="lg:col-span-3">
                  <ScreenshotInspector
                    screenshot={report.screenshot}
                    elements={report.vampire_elements}
                    selectedElement={selectedElement}
                    onSelect={setSelectedElementID}
                  />
                </div>

                <div className="lg:col-span-2">
                  <VampireList
                    elements={report.vampire_elements}
                    selectedElementID={selectedElementID}
                    onSelect={setSelectedElementID}
                  />
                </div>
              </section>

              <InsightsPanel
                report={report}
                selectedElementID={selectedElementID}
                onSelectElement={setSelectedElementID}
              />

              {/* Green Fix Studio */}
              <GreenFixStudio
                report={report}
                code={greenFixCode}
                onCodeChange={handleGreenFixCodeChange}
                onGenerate={handleGenerateGreenFix}
                isGenerating={isGeneratingFix}
                result={greenFixResult}
                error={greenFixError}
              />

              {/* Technical Context & Breakdowns */}
              <section className="flex flex-col gap-8">
                <section className="bg-surface-container-low rounded-3xl p-8 border border-outline-variant/10">
                  <div className="flex flex-col gap-8 lg:flex-row lg:items-start lg:justify-between">
                    <div className="max-w-xl">
                      <p className="text-on-surface-variant text-xs uppercase tracking-widest font-label mb-2">
                        Contexto técnico
                      </p>
                      <h2 className="text-2xl font-bold font-headline text-on-surface">
                        Audit evidence
                      </h2>
                      <p className="mt-3 text-sm leading-relaxed text-on-surface-variant">
                        La capa de evidencia mantiene visibles los tiempos base, el
                        hosting y el mix de transferencia sin competir con el núcleo
                        diagnóstico.
                      </p>
                    </div>

                    <div className="grid gap-4 sm:grid-cols-2 lg:w-[32rem]">
                      <div className="bg-surface-container rounded-2xl p-5 border border-outline-variant/5">
                        <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                          Transferencia
                        </div>
                        <div className="mt-2 text-xl font-headline font-bold text-on-surface">
                          {formatBytes(report.total_bytes_transferred)}
                        </div>
                        <div className="mt-1 text-xs opacity-70">
                          {report.summary.total_requests.toLocaleString("es-CO")}{" "}
                          peticiones
                        </div>
                      </div>

                      <div className="bg-surface-container rounded-2xl p-5 border border-outline-variant/5">
                        <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                          Tiempos base
                        </div>
                        <div className="mt-2 text-xl font-headline font-bold text-on-surface">
                          {formatMilliseconds(
                            report.performance.dom_content_loaded_ms
                          )}
                        </div>
                        <div className="mt-1 text-xs opacity-70">
                          Load {formatMilliseconds(report.performance.load_ms)}
                        </div>
                      </div>

                      <div className="bg-surface-container rounded-2xl p-5 border border-outline-variant/5">
                        <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                          Anclajes
                        </div>
                        <div className="mt-2 text-xl font-headline font-bold text-on-surface">
                          {report.summary.visual_mapped_vampires} /{" "}
                          {report.vampire_elements.length}
                        </div>
                        <div className="mt-1 text-xs opacity-70">
                          Recursos visibles
                        </div>
                      </div>

                      <div className="bg-surface-container rounded-2xl p-5 border border-outline-variant/5">
                        <div className="text-on-surface-variant text-[10px] uppercase tracking-widest font-label">
                          Hosting
                        </div>
                        <div className="mt-2 text-lg font-headline font-bold text-on-surface truncate">
                          {formatHostingLabel(report)}
                        </div>
                        <div className="mt-1 text-xs opacity-70 truncate px-1">
                          {report.hosted_by || "No se pudo resolver el proveedor"}
                        </div>
                      </div>
                    </div>
                  </div>

                  {report.warnings.length > 0 ? (
                    <div className="mt-8 rounded-2xl bg-error-container/10 p-5 border border-error-container/20">
                      <div className="text-error font-bold text-sm uppercase tracking-wider font-label">
                        Advertencias
                      </div>
                      <ul className="mt-3 space-y-2 text-sm leading-6 text-on-surface-variant">
                        {report.warnings.map((warning) => (
                          <li key={warning}>- {warning}</li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                </section>

                <section className="bento-grid xl:grid-cols-[minmax(0,0.88fr)_minmax(0,1.12fr)]">
                  <MethodologyCard report={report} />
                  <div className="bento-grid lg:grid-cols-2">
                    <BreakdownBars
                      title="Breakdown by type"
                      subtitle="Peso por tipo de recurso"
                      items={report.breakdown_by_type}
                    />
                    <BreakdownBars
                      title="Breakdown by party"
                      subtitle="Primera parte vs terceros"
                      items={report.breakdown_by_party}
                    />
                  </div>
                </section>
              </section>

              <MarkdownReportCard report={report} greenFix={greenFixResult} />
            </m.section>
          ) : (
            <m.section
              key={isScanning ? "loading" : "empty"}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              id="diagnostic"
              className="bg-surface-container-low rounded-[2rem] p-10 mt-8 border border-outline-variant/5"
            >
              <p className="text-primary text-xs uppercase tracking-widest font-label font-bold flex items-center gap-2">
                <span className="material-symbols-outlined text-sm">info</span>
                {isScanning ? "Escaneo en curso" : "Estado del informe"}
              </p>
              <h2 className="mt-4 text-3xl font-headline font-bold text-on-surface">
                {isScanning
                  ? scanProgressLabels[scanProgressIndex]
                  : "Ejecuta el primer análisis para poblar el dashboard."}
              </h2>
              <p className="mt-4 max-w-3xl text-sm leading-relaxed text-on-surface-variant">
                {isScanning
                  ? "Wattless está capturando transferencia, Web Vitals, recursos dominantes, hosting y señales de ahorro para construir un informe legible en una sola pasada."
                  : "El flujo sigue la lógica de un informe técnico: primero score y métricas clave, luego diagnóstico visual, después acción con Green Fix y finalmente exportación."}
              </p>

              <div className="mt-10 grid gap-4 md:grid-cols-3">
                {emptyStateHighlights.map((highlight) => (
                  <div
                    key={highlight.id}
                    id={highlight.id}
                    className="bg-surface-container px-6 py-5 rounded-2xl border border-outline-variant/10"
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
  if (report.hosting_verdict === "unknown") {
    return "Hosting desconocido";
  }
  return report.hosting_is_green ? "Hosting verde" : "Hosting no verde";
}
