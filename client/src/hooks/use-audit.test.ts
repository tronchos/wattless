import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

import { useAudit } from "./use-audit";
import type { ScanJobResponse, ScanReport } from "@/lib/types";

vi.mock("@/lib/api", () => ({
  submitScan: vi.fn(),
  pollScanJob: vi.fn(),
  APIError: class APIError extends Error {
    status: number;
    retryAfterSeconds: number | null;
    job?: ScanJobResponse;

    constructor(
      message: string,
      options: { status: number; retryAfterSeconds?: number | null; job?: ScanJobResponse },
    ) {
      super(message);
      this.name = "APIError";
      this.status = options.status;
      this.retryAfterSeconds = options.retryAfterSeconds ?? null;
      this.job = options.job;
    }
  },
}));

import { APIError, pollScanJob, submitScan } from "@/lib/api";

const mockSubmitScan = vi.mocked(submitScan);
const mockPollScanJob = vi.mocked(pollScanJob);

const fakeReport = {
  url: "https://example.com",
  score: "A",
  total_bytes_transferred: 1234,
  co2_grams_per_visit: 0.12,
  hosting_is_green: true,
  hosting_verdict: "green",
  hosted_by: "Example Host",
  site_profile: {
    framework_hint: "generic",
    evidence: ["No se detectaron marcadores claros de framework; se usa perfil genérico."],
  },
  summary: {
    total_requests: 1,
    successful_requests: 1,
    failed_requests: 0,
    first_party_bytes: 1234,
    third_party_bytes: 0,
    potential_savings_bytes: 0,
    visual_mapped_vampires: 0,
  },
  breakdown_by_type: [],
  breakdown_by_party: [],
  insights: {
    provider: "rule_based",
    executive_summary: "ok",
    pitch_line: "ok",
    top_actions: [],
  },
  vampire_elements: [],
  performance: {
    load_ms: 1000,
    dom_content_loaded_ms: 500,
    script_resource_duration_ms: 10,
    lcp_ms: 1200,
    fcp_ms: 400,
    long_tasks_total_ms: 0,
    long_tasks_count: 0,
  },
  analysis: {
    summary: {
      above_fold_bytes: 1234,
      below_fold_bytes: 0,
      analytics_bytes: 0,
      analytics_requests: 0,
      font_bytes: 0,
      font_requests: 0,
      repeated_gallery_bytes: 0,
      repeated_gallery_count: 0,
      render_critical_bytes: 0,
    },
    findings: [],
    resource_groups: [],
  },
  screenshot: {
    mime_type: "image/jpeg",
    strategy: "single",
    viewport_width: 1200,
    viewport_height: 900,
    document_width: 1200,
    document_height: 900,
    captured_height: 900,
    tiles: [],
  },
  meta: {
    generated_at: "2026-03-28T00:00:00Z",
    scan_duration_ms: 1000,
    scanner_version: "2026.03",
  },
  methodology: {
    model: "test",
    formula: "test",
    source: "test",
    assumptions: [],
  },
  warnings: [],
} as ScanReport;

beforeEach(() => {
  vi.clearAllMocks();
  vi.useRealTimers();
  window.sessionStorage.clear();
});

describe("useAudit", () => {
  it("shows error when URL is empty", async () => {
    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(result.current.scanError).toBe(
      "Escribe una URL para empezar el análisis.",
    );
    expect(mockSubmitScan).not.toHaveBeenCalled();
  });

  it("shows error when URL is invalid", async () => {
    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("not a valid url :::");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(result.current.scanError).toBe(
      "La URL no es válida. Verifica el formato e intenta de nuevo.",
    );
    expect(mockSubmitScan).not.toHaveBeenCalled();
  });

  it("auto-prepends https:// when missing scheme", async () => {
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 1,
    });

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(mockSubmitScan).toHaveBeenCalledWith("https://example.com");
    expect(result.current.jobStatus).toBe("queued");
  });

  it("polls queued jobs until the report is completed", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 2,
      estimated_wait_seconds: 30,
    });
    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: fakeReport,
    });

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(result.current.report).toBeNull();
    expect(result.current.jobStatus).toBe("queued");
    expect(result.current.queuePosition).toBe(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(mockPollScanJob).toHaveBeenCalledWith("wl_job");
    expect(result.current.report?.url).toBe("https://example.com");
    expect(result.current.isScanning).toBe(false);
    expect(window.sessionStorage.getItem("wattless.active_scan_job")).toBeNull();
  });

  it("rehydrates an active job from sessionStorage", () => {
    window.sessionStorage.setItem(
      "wattless.active_scan_job",
      JSON.stringify({
        job_id: "wl_job",
        url: "https://example.com",
        status: "queued",
        position: 3,
        estimated_wait_seconds: 45,
      } satisfies ScanJobResponse),
    );

    const { result } = renderHook(() => useAudit());

    expect(result.current.jobStatus).toBe("queued");
    expect(result.current.queuePosition).toBe(3);
    expect(result.current.submittedURL).toBe("https://example.com");
    expect(result.current.isScanning).toBe(true);
  });

  it("exposes the conflicting job and lets the user resume it", async () => {
    const conflictJob: ScanJobResponse = {
      job_id: "wl_conflict",
      url: "https://example.com",
      status: "scanning",
      position: 0,
    };
    mockSubmitScan.mockRejectedValueOnce(
      new APIError("Ya tienes un análisis en curso", {
        status: 409,
        job: conflictJob,
      }),
    );

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(result.current.conflictingJob?.job_id).toBe("wl_conflict");
    expect(result.current.isScanning).toBe(false);

    act(() => {
      result.current.resumeConflictingJob();
    });

    expect(result.current.conflictingJob).toBeNull();
    expect(result.current.jobStatus).toBe("scanning");
    expect(result.current.isScanning).toBe(true);
  });

  it("surfaces expired jobs returned by polling", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 1,
    });
    mockPollScanJob.mockRejectedValueOnce(
      new APIError("Tu turno expiró. Envía un nuevo análisis.", {
        status: 410,
      }),
    );

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await result.current.handleSubmit();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.isScanning).toBe(false);
    expect(result.current.jobStatus).toBeNull();
    expect(result.current.scanError).toBe("Tu turno expiró. Envía un nuevo análisis.");
  });

  it("keeps polling after transient status errors", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 2,
      estimated_wait_seconds: 30,
    });
    mockPollScanJob
      .mockRejectedValueOnce(
        new APIError("Respuesta inesperada del escáner", {
          status: 502,
        }),
      )
      .mockResolvedValueOnce({
        job_id: "wl_job",
        url: "https://example.com",
        status: "completed",
        position: 0,
        report: fakeReport,
      });

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await result.current.handleSubmit();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.scanError).toBe("Respuesta inesperada del escáner");
    expect(result.current.isScanning).toBe(true);
    expect(result.current.jobStatus).toBe("queued");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(mockPollScanJob).toHaveBeenCalledTimes(2);
    expect(result.current.report?.url).toBe("https://example.com");
    expect(result.current.scanError).toBeNull();
    expect(result.current.isScanning).toBe(false);
  });
});
