import { useCallback, useEffect, useRef, useState } from "react";

import { APIError, pollScanJob, submitScan } from "@/lib/api";
import type {
  ScanJobResponse,
  ScanJobStatus,
  ScanReport,
  VampireElement,
} from "@/lib/types";

export const sampleURL = "https://example.com";
export const scanProgressLabels = [
  "Midiendo transferencia de red",
  "Estimando coste energético",
  "Identificando carga crítica del LCP",
];

const activeJobStorageKey = "wattless.active_scan_job";
const pollIntervalMs = 1500;

function isAnchoredAction(action: ScanReport["insights"]["top_actions"][number]): boolean {
  return action.related_resource_ids.length > 0;
}

function resolvePreferredElement(report: ScanReport): VampireElement | null {
  const anchoredActionIDs = new Set(
    report.insights.top_actions.filter(isAnchoredAction).map((action) => action.id),
  );
  const withFix = report.vampire_elements.find(
    (element) =>
      element.asset_insight.recommended_fix &&
      (!element.asset_insight.related_action_id ||
        anchoredActionIDs.has(element.asset_insight.related_action_id)),
  );
  if (withFix) {
    return withFix;
  }

  for (const action of report.insights.top_actions) {
    if (!isAnchoredAction(action)) {
      continue;
    }
    const matching = report.vampire_elements.find(
      (element) =>
        element.asset_insight.related_action_id === action.id ||
        action.related_resource_ids.includes(element.id),
    );
    if (matching) {
      return matching;
    }
  }
  return report.vampire_elements[0] ?? null;
}

function isLiveJobStatus(status: ScanJobStatus | null): status is "queued" | "scanning" {
  return status === "queued" || status === "scanning";
}

function isStoredJobPayload(value: unknown): value is ScanJobResponse {
  if (typeof value !== "object" || value === null) {
    return false;
  }

  const payload = value as Partial<ScanJobResponse>;
  return (
    typeof payload.job_id === "string" &&
    typeof payload.url === "string" &&
    typeof payload.status === "string" &&
    typeof payload.position === "number"
  );
}

function loadStoredJob(): ScanJobResponse | null {
  if (typeof window === "undefined") {
    return null;
  }

  const rawValue = window.sessionStorage.getItem(activeJobStorageKey);
  if (!rawValue) {
    return null;
  }

  try {
    const storedJob = JSON.parse(rawValue);
    if (!isStoredJobPayload(storedJob) || !isLiveJobStatus(storedJob.status)) {
      window.sessionStorage.removeItem(activeJobStorageKey);
      return null;
    }

    return storedJob;
  } catch {
    window.sessionStorage.removeItem(activeJobStorageKey);
    return null;
  }
}

export function useAudit() {
  const [restoredJob] = useState<ScanJobResponse | null>(() => loadStoredJob());
  const [inputURL, setInputURL] = useState(sampleURL);
  const [report, setReport] = useState<ScanReport | null>(null);
  const [previousReport, setPreviousReport] = useState<ScanReport | null>(null);
  const [selectedElementID, setSelectedElementID] = useState<string | null>(null);
  const [selectionSignal, setSelectionSignal] = useState(0);
  const [scanError, setScanError] = useState<string | null>(null);
  const [isScanning, setIsScanning] = useState(
    () => restoredJob !== null && isLiveJobStatus(restoredJob.status),
  );
  const [scanProgressIndex, setScanProgressIndex] = useState(0);
  const [jobId, setJobId] = useState<string | null>(() => restoredJob?.job_id ?? null);
  const [jobStatus, setJobStatus] = useState<ScanJobStatus | null>(
    () => restoredJob?.status ?? null,
  );
  const [queuePosition, setQueuePosition] = useState<number | null>(
    () => restoredJob?.position ?? null,
  );
  const [estimatedWaitSeconds, setEstimatedWaitSeconds] = useState<number | null>(
    () => restoredJob?.estimated_wait_seconds ?? null,
  );
  const [submittedURL, setSubmittedURL] = useState<string | null>(
    () => restoredJob?.url ?? null,
  );
  const [conflictingJob, setConflictingJob] = useState<ScanJobResponse | null>(null);
  const pendingPreviousReportRef = useRef<ScanReport | null>(null);

  const selectedElement =
    report?.vampire_elements.find((element) => element.id === selectedElementID) ??
    null;

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    if (!jobId || !jobStatus || !isLiveJobStatus(jobStatus) || !submittedURL) {
      window.sessionStorage.removeItem(activeJobStorageKey);
      return;
    }

    const payload: ScanJobResponse = {
      job_id: jobId,
      url: submittedURL,
      status: jobStatus,
      position: queuePosition ?? 0,
      estimated_wait_seconds: estimatedWaitSeconds ?? undefined,
    };
    window.sessionStorage.setItem(activeJobStorageKey, JSON.stringify(payload));
  }, [estimatedWaitSeconds, jobId, jobStatus, queuePosition, submittedURL]);

  useEffect(() => {
    if (jobStatus !== "scanning") {
      return;
    }

    const intervalID = window.setInterval(() => {
      setScanProgressIndex((current) => (current + 1) % scanProgressLabels.length);
    }, 1200);

    return () => window.clearInterval(intervalID);
  }, [jobStatus]);

  const selectElement = useCallback((id: string | null) => {
    setSelectionSignal((current) => current + 1);
    setSelectedElementID(id);
  }, []);

  const clearActiveJob = useCallback(() => {
    setJobId(null);
    setJobStatus(null);
    setQueuePosition(null);
    setEstimatedWaitSeconds(null);
    setSubmittedURL(null);
    setScanProgressIndex(0);

    if (typeof window !== "undefined") {
      window.sessionStorage.removeItem(activeJobStorageKey);
    }
  }, []);

  const adoptJob = useCallback((job: ScanJobResponse) => {
    setJobId(job.job_id);
    setJobStatus(job.status);
    setQueuePosition(job.position);
    setEstimatedWaitSeconds(job.estimated_wait_seconds ?? null);
    setSubmittedURL(job.url);
    setIsScanning(isLiveJobStatus(job.status));
    setScanError(null);
  }, []);

  const applyCompletedReport = useCallback(
    (nextReport: ScanReport) => {
      const previous = pendingPreviousReportRef.current;
      setPreviousReport(previous?.url === nextReport.url ? previous : null);
      setReport(nextReport);
      setSelectionSignal((current) => current + 1);
      setSelectedElementID(resolvePreferredElement(nextReport)?.id ?? null);
      setScanError(null);
      setConflictingJob(null);
      setIsScanning(false);
      clearActiveJob();
      pendingPreviousReportRef.current = null;
    },
    [clearActiveJob],
  );

  const resumeConflictingJob = useCallback(() => {
    if (!conflictingJob || !isLiveJobStatus(conflictingJob.status)) {
      return;
    }

    pendingPreviousReportRef.current = report;
    setPreviousReport(null);
    setReport(null);
    setConflictingJob(null);
    setScanError(null);
    setScanProgressIndex(0);
    adoptJob(conflictingJob);
  }, [adoptJob, conflictingJob, report]);

  const handleSubmit = useCallback(
    async (event?: React.FormEvent<HTMLFormElement>) => {
      event?.preventDefault();

      let nextURL = inputURL.trim();
      if (!nextURL) {
        setScanError("Escribe una URL para empezar el análisis.");
        return;
      }

      if (!/^https?:\/\//i.test(nextURL)) {
        nextURL = `https://${nextURL}`;
      }

      try {
        new URL(nextURL);
      } catch {
        setScanError("La URL no es válida. Verifica el formato e intenta de nuevo.");
        return;
      }

      const currentReport = report;
      setIsScanning(true);
      setScanError(null);
      setConflictingJob(null);
      setScanProgressIndex(0);

      try {
        const job = await submitScan(nextURL);
        pendingPreviousReportRef.current = currentReport;
        setPreviousReport(null);
        setReport(null);

        if (job.status === "completed" && job.report) {
          applyCompletedReport(job.report);
          return;
        }

        if (job.status === "failed") {
          clearActiveJob();
          setIsScanning(false);
          setScanError(job.error ?? "El escaneo falló");
          return;
        }

        adoptJob(job);
      } catch (submitError) {
        if (submitError instanceof APIError && submitError.status === 409 && submitError.job) {
          setConflictingJob(submitError.job);
        }

        setScanError(
          submitError instanceof Error ? submitError.message : "El escaneo falló",
        );
        setIsScanning(false);
      }
    },
    [adoptJob, applyCompletedReport, clearActiveJob, inputURL, report],
  );

  useEffect(() => {
    if (!jobId || !jobStatus || !isLiveJobStatus(jobStatus)) {
      return;
    }

    let isCancelled = false;
    let timeoutID: number | undefined;

    const scheduleNextPoll = () => {
      if (isCancelled) {
        return;
      }

      timeoutID = window.setTimeout(runPoll, pollIntervalMs);
    };

    const runPoll = async () => {
      try {
        const result = await pollScanJob(jobId);
        if (isCancelled) {
          return;
        }

        if (result.status === "completed" && result.report) {
          applyCompletedReport(result.report);
          return;
        }

        if (result.status === "failed") {
          clearActiveJob();
          setIsScanning(false);
          setScanError(result.error ?? "El escaneo falló");
          return;
        }

        if (result.status === "expired") {
          clearActiveJob();
          setIsScanning(false);
          setScanError(result.error ?? "Tu turno expiró. Envía un nuevo análisis.");
          return;
        }

        adoptJob(result);
      } catch (pollError) {
        if (isCancelled) {
          return;
        }

        if (pollError instanceof APIError && pollError.status === 410) {
          clearActiveJob();
          setIsScanning(false);
          setScanError(pollError.message);
          return;
        }

        if (pollError instanceof APIError && pollError.status === 404) {
          clearActiveJob();
          setIsScanning(false);
          setScanError("No encontramos ese turno. Envía un nuevo análisis.");
          return;
        }

        setScanError(
          pollError instanceof Error
            ? pollError.message
            : "No se pudo consultar el estado del turno.",
        );
      }
      scheduleNextPoll();
    };

    scheduleNextPoll();

    return () => {
      isCancelled = true;
      if (timeoutID !== undefined) {
        window.clearTimeout(timeoutID);
      }
    };
  }, [adoptJob, applyCompletedReport, clearActiveJob, jobId, jobStatus]);

  return {
    inputURL,
    setInputURL,
    report,
    previousReport,
    selectedElementID,
    setSelectedElementID: selectElement,
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
    conflictingJob,
    resumeConflictingJob,
  };
}
