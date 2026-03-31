import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { APIError, fetchInsights, pollScanJob, submitScan } from "@/lib/api";
import type {
  InsightsStatus,
  ScanInsights,
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
const lastCompletedJobStorageKey = "wattless.last_completed_scan_job";
const pollIntervalMs = 1500;
const insightsPollIntervalMs = 2000;

function getVisibleRelatedResourceIDs(
  action: ScanReport["insights"]["top_actions"][number],
): string[] {
  return Array.isArray(action.visible_related_resource_ids)
    ? action.visible_related_resource_ids
    : [];
}

function isAnchoredAction(action: ScanReport["insights"]["top_actions"][number]): boolean {
  return getVisibleRelatedResourceIDs(action).length > 0;
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
        getVisibleRelatedResourceIDs(action).includes(element.id),
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

function loadLastCompletedJobID(): string | null {
  if (typeof window === "undefined") {
    return null;
  }

  const jobID = window.sessionStorage.getItem(lastCompletedJobStorageKey);
  return jobID?.trim() || null;
}

function clearStoredCompletedJob(): void {
  if (typeof window === "undefined") {
    return;
  }

  window.sessionStorage.removeItem(lastCompletedJobStorageKey);
}

function isAIEnrichedReport(report: ScanReport): boolean {
  if (report.insights.provider !== "rule_based") {
    return true;
  }

  return report.vampire_elements.some(
    (element) => element.asset_insight.source !== "rule_based",
  );
}

function mergeAIReport(
  baseReport: ScanReport | null,
  aiInsights: ScanInsights | null,
  aiVampires: VampireElement[] | null,
): ScanReport | null {
  if (!baseReport) {
    return null;
  }

  if (!aiInsights && !aiVampires) {
    return baseReport;
  }

  return {
    ...baseReport,
    insights: aiInsights ?? baseReport.insights,
    vampire_elements: aiVampires ?? baseReport.vampire_elements,
  };
}

export function useAudit() {
  const [restoredJob] = useState<ScanJobResponse | null>(() => loadStoredJob());
  const [inputURL, setInputURL] = useState("");
  const [baseReport, setBaseReport] = useState<ScanReport | null>(null);
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
  const [reportJobId, setReportJobId] = useState<string | null>(null);
  const [conflictingJob, setConflictingJob] = useState<ScanJobResponse | null>(null);
  const [insightsStatus, setInsightsStatus] = useState<InsightsStatus>("none");
  const [aiInsights, setAIInsights] = useState<ScanInsights | null>(null);
  const [aiVampires, setAIVampires] = useState<VampireElement[] | null>(null);
  const [lastCompletedJobId, setLastCompletedJobId] = useState<string | null>(
    () => loadLastCompletedJobID(),
  );
  const pendingPreviousReportRef = useRef<ScanReport | null>(null);

  const report = useMemo(
    () => mergeAIReport(baseReport, aiInsights, aiVampires),
    [aiInsights, aiVampires, baseReport],
  );

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
    if (typeof window === "undefined") {
      return;
    }

    if (!lastCompletedJobId) {
      window.sessionStorage.removeItem(lastCompletedJobStorageKey);
      return;
    }

    window.sessionStorage.setItem(lastCompletedJobStorageKey, lastCompletedJobId);
  }, [lastCompletedJobId]);

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

  const resetAIState = useCallback((status: InsightsStatus = "none") => {
    setAIInsights(null);
    setAIVampires(null);
    setInsightsStatus(status);
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
    (nextReport: ScanReport, completedJobId: string) => {
      const previous = pendingPreviousReportRef.current;
      setPreviousReport(previous?.url === nextReport.url ? previous : null);
      setBaseReport(nextReport);
      setReportJobId(completedJobId);
      setLastCompletedJobId(completedJobId);
      resetAIState(isAIEnrichedReport(nextReport) ? "ready" : "none");
      setSelectionSignal((current) => current + 1);
      setSelectedElementID(resolvePreferredElement(nextReport)?.id ?? null);
      setScanError(null);
      setConflictingJob(null);
      setIsScanning(false);
      clearActiveJob();
      pendingPreviousReportRef.current = null;
    },
    [clearActiveJob, resetAIState],
  );

  const resumeConflictingJob = useCallback(() => {
    if (!conflictingJob || !isLiveJobStatus(conflictingJob.status)) {
      return;
    }

    pendingPreviousReportRef.current = report;
    setPreviousReport(null);
    setBaseReport(null);
    setReportJobId(null);
    resetAIState();
    setConflictingJob(null);
    setScanError(null);
    setScanProgressIndex(0);
    adoptJob(conflictingJob);
  }, [adoptJob, conflictingJob, report, resetAIState]);

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
        setBaseReport(null);
        setReportJobId(null);
        resetAIState();

        if (job.status === "completed" && job.report) {
          applyCompletedReport(job.report, job.job_id);
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
    [adoptJob, applyCompletedReport, clearActiveJob, inputURL, report, resetAIState],
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
          applyCompletedReport(result.report, jobId);
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

  useEffect(() => {
    if (jobId || baseReport || !lastCompletedJobId) {
      return;
    }

    let isCancelled = false;

    const rehydrateCompletedReport = async () => {
      try {
        const result = await pollScanJob(lastCompletedJobId);
        if (isCancelled) {
          return;
        }

        if (result.status === "completed" && result.report) {
          setBaseReport(result.report);
          setReportJobId(result.job_id);
          setInsightsStatus(isAIEnrichedReport(result.report) ? "ready" : "none");
          setSelectionSignal((current) => current + 1);
          setSelectedElementID(resolvePreferredElement(result.report)?.id ?? null);
          setScanError(null);
          return;
        }

        if (result.status === "expired") {
          setLastCompletedJobId(null);
          clearStoredCompletedJob();
        }
      } catch (error) {
        if (isCancelled) {
          return;
        }

        if (
          error instanceof APIError &&
          (error.status === 404 || error.status === 410)
        ) {
          setLastCompletedJobId(null);
          clearStoredCompletedJob();
        }
      }
    };

    void rehydrateCompletedReport();

    return () => {
      isCancelled = true;
    };
  }, [baseReport, jobId, lastCompletedJobId]);

  useEffect(() => {
    if (!reportJobId || !baseReport) {
      return;
    }

    if (isAIEnrichedReport(baseReport)) {
      setInsightsStatus("ready");
      return;
    }

    let isCancelled = false;
    let timeoutID: number | undefined;

    const scheduleNextPoll = () => {
      if (isCancelled) {
        return;
      }

      timeoutID = window.setTimeout(runPoll, insightsPollIntervalMs);
    };

    const runPoll = async () => {
      try {
        const result = await fetchInsights(reportJobId);
        if (isCancelled) {
          return;
        }

        if (result === null) {
          resetAIState("none");
          return;
        }

        if (result.status === "processing") {
          setInsightsStatus("processing");
          scheduleNextPoll();
          return;
        }

        if (result.status === "failed") {
          resetAIState("failed");
          return;
        }

        if (result.status === "ready" && result.insights && result.vampire_elements) {
          setAIInsights(result.insights);
          setAIVampires(result.vampire_elements);
          setInsightsStatus("ready");
          return;
        }

        setInsightsStatus("processing");
        scheduleNextPoll();
      } catch (error) {
        if (isCancelled) {
          return;
        }

        if (error instanceof APIError && error.status === 410) {
          setLastCompletedJobId(null);
          clearStoredCompletedJob();
          resetAIState();
          return;
        }

        if (error instanceof APIError && error.status === 404 && error.code === "job_not_found") {
          setLastCompletedJobId(null);
          clearStoredCompletedJob();
          resetAIState();
          return;
        }

        scheduleNextPoll();
      }
    };

    void runPoll();

    return () => {
      isCancelled = true;
      if (timeoutID !== undefined) {
        window.clearTimeout(timeoutID);
      }
    };
  }, [baseReport, reportJobId, resetAIState, setLastCompletedJobId]);

  return {
    inputURL,
    setInputURL,
    report,
    reportJobId,
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
    insightsStatus,
    resumeConflictingJob,
  };
}
