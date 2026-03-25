import { useState, useTransition, useCallback, useEffect } from "react";
import { scanURL, generateGreenFix } from "@/lib/api";
import type { ScanReport, GreenFixResponse, VampireElement } from "@/lib/types";

export const sampleURL = "https://example.com";
export const scanProgressLabels = [
  "Midiendo transferencia de red",
  "Estimando coste energético",
  "Identificando carga crítica del LCP",
];
const minimumGreenFixCodeLength = 20;
const maximumGreenFixCodeLength = 20_000;

function resolvePreferredElement(report: ScanReport): VampireElement | null {
  const action = report.insights.top_actions[0];
  if (action) {
    const matching = report.vampire_elements.find(
      (element) => element.id === action.related_resource_id
    );
    if (matching) return matching;
  }
  return report.vampire_elements[0] ?? null;
}

export function useAudit() {
  const [inputURL, setInputURL] = useState(sampleURL);
  const [report, setReport] = useState<ScanReport | null>(null);
  const [previousReport, setPreviousReport] = useState<ScanReport | null>(null);
  const [selectedElementID, setSelectedElementID] = useState<string | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);
  const [greenFixError, setGreenFixError] = useState<string | null>(null);
  const [isScanning, setIsScanning] = useState(false);
  const [scanProgressIndex, setScanProgressIndex] = useState(0);
  const [greenFixCode, setGreenFixCode] = useState("");
  const [isGeneratingFix, setIsGeneratingFix] = useState(false);
  const [greenFixResult, setGreenFixResult] = useState<GreenFixResponse | null>(null);
  const [, startTransition] = useTransition();

  const selectedElement =
    report?.vampire_elements.find((element) => element.id === selectedElementID) ??
    null;

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

  const handleSubmit = useCallback(
    async (event?: React.FormEvent<HTMLFormElement>) => {
      event?.preventDefault();

      const nextURL = inputURL.trim();
      if (!nextURL) {
        setScanError("Escribe una URL para empezar el análisis.");
        return;
      }

      const currentReport = report;
      setIsScanning(true);
      setScanError(null);
      setGreenFixError(null);
      setGreenFixResult(null);

      try {
        const nextReport = await scanURL(nextURL);
        startTransition(() => {
          setPreviousReport(
            currentReport?.url === nextReport.url ? currentReport : null
          );
          setReport(nextReport);
          setSelectedElementID(resolvePreferredElement(nextReport)?.id ?? null);
        });
      } catch (submitError) {
        setScanError(
          submitError instanceof Error ? submitError.message : "El escaneo falló"
        );
      } finally {
        setIsScanning(false);
      }
    },
    [inputURL, report]
  );

  const handleGenerateGreenFix = useCallback(async () => {
    if (!report) return;

    const trimmedCode = greenFixCode.trim();
    if (trimmedCode.length < minimumGreenFixCodeLength) {
      setGreenFixResult(null);
      setGreenFixError(
        "Pega al menos 20 caracteres de código para generar un Green Fix útil."
      );
      return;
    }
    if (trimmedCode.length > maximumGreenFixCodeLength) {
      setGreenFixResult(null);
      setGreenFixError(
        "Reduce el snippet a 20.000 caracteres o menos para generar el Green Fix."
      );
      return;
    }

    setIsGeneratingFix(true);
    setGreenFixError(null);

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
      setGreenFixError(
        generationError instanceof Error
          ? generationError.message
          : "No se pudo generar el Green Fix"
      );
    } finally {
      setIsGeneratingFix(false);
    }
  }, [report, greenFixCode, selectedElement]);

  const handleGreenFixCodeChange = useCallback((value: string) => {
    setGreenFixCode(value);
    setGreenFixResult(null);
    setGreenFixError(null);
  }, []);

  return {
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
  };
}
