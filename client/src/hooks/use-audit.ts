import { useState, useTransition, useCallback, useEffect } from "react";
import { scanURL } from "@/lib/api";
import type { ScanReport, VampireElement } from "@/lib/types";

export const sampleURL = "https://example.com";
export const scanProgressLabels = [
  "Midiendo transferencia de red",
  "Estimando coste energético",
  "Identificando carga crítica del LCP",
];

function resolvePreferredElement(report: ScanReport): VampireElement | null {
  const action = report.insights.top_actions[0];
  if (action) {
    const matching = report.vampire_elements.find(
      (element) => action.related_resource_ids.includes(element.id)
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
  const [isScanning, setIsScanning] = useState(false);
  const [scanProgressIndex, setScanProgressIndex] = useState(0);
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


  return {
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
  };
}
