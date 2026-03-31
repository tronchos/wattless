import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook } from "@testing-library/react";

import { useAudit } from "./use-audit";
import type { ScanJobResponse, ScanReport } from "@/lib/types";

vi.mock("@/lib/api", () => ({
  submitScan: vi.fn(),
  pollScanJob: vi.fn(),
  fetchInsights: vi.fn(),
  APIError: class APIError extends Error {
    status: number;
    retryAfterSeconds: number | null;
    code: string | null;
    job?: ScanJobResponse;

    constructor(
      message: string,
      options: {
        status: number;
        retryAfterSeconds?: number | null;
        code?: string | null;
        job?: ScanJobResponse;
      },
    ) {
      super(message);
      this.name = "APIError";
      this.status = options.status;
      this.retryAfterSeconds = options.retryAfterSeconds ?? null;
      this.code = options.code ?? null;
      this.job = options.job;
    }
  },
}));

import { APIError, fetchInsights, pollScanJob, submitScan } from "@/lib/api";

const mockSubmitScan = vi.mocked(submitScan);
const mockPollScanJob = vi.mocked(pollScanJob);
const mockFetchInsights = vi.mocked(fetchInsights);

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
    render_metrics_complete: true,
    long_tasks_total_ms: 0,
    long_tasks_count: 0,
  },
  analysis: {
    summary: {
      above_fold_visual_bytes: 1234,
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
  mockFetchInsights.mockResolvedValue(null);
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

    act(() => {
      result.current.setInputURL("https://example.com");
    });

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
    expect(result.current.reportJobId).toBe("wl_job");
    expect(result.current.isScanning).toBe(false);
    expect(window.sessionStorage.getItem("wattless.active_scan_job")).toBeNull();
  });

  it("does not auto-select a vampire from an unanchored top action", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 1,
    });

    const unanchoredReport: ScanReport = {
      ...fakeReport,
      insights: {
        ...fakeReport.insights,
        top_actions: [
          {
            id: "act-1",
            related_finding_id: "repeated_gallery_overdelivery",
            title: "Comprime la galería",
            reason: "No hay anchor visible veraz.",
            confidence: "high",
            evidence: ["Grupo repetido fuera de los vampiros visibles."],
            estimated_savings_bytes: 250000,
            likely_lcp_impact: "low",
            related_resource_ids: [],
            visible_related_resource_ids: [],
            recommended_fix: {
              summary: "Optimiza el grid.",
              optimized_code: "<Image />",
              changes: ["Usa variantes responsivas"],
              expected_impact: "Menos bytes.",
            },
          },
        ],
      },
      vampire_elements: [
        {
          id: "avatar",
          url: "https://example.com/avatar.webp",
          type: "image",
          mime_type: "image/webp",
          hostname: "example.com",
          party: "first_party",
          status_code: 200,
          bytes: 18000,
          failed: false,
          failure_reason: "",
          transfer_share: 1,
          estimated_savings_bytes: 12000,
          position_band: "above_fold",
          visual_role: "above_fold_media",
          dom_tag: "img",
          loading_attr: "",
          fetch_priority: "",
          responsive_image: false,
          is_third_party_tool: false,
          third_party_kind: "unknown",
          asset_insight: {
            source: "rule_based",
            scope: "asset",
            title: "Avatar",
            short_problem: "Activo visible.",
            why_it_matters: "Referencia base.",
            recommended_action: "Revisar este recurso.",
            confidence: "low",
            likely_lcp_impact: "low",
            evidence: [],
          },
          bounding_box: null,
        },
        {
          id: "course-card",
          url: "https://example.com/course.webp",
          type: "image",
          mime_type: "image/webp",
          hostname: "example.com",
          party: "first_party",
          status_code: 200,
          bytes: 220000,
          failed: false,
          failure_reason: "",
          transfer_share: 10,
          estimated_savings_bytes: 50000,
          position_band: "below_fold",
          visual_role: "repeated_card_media",
          dom_tag: "img",
          loading_attr: "",
          fetch_priority: "",
          responsive_image: false,
          is_third_party_tool: false,
          third_party_kind: "unknown",
          asset_insight: {
            source: "rule_based",
            scope: "group",
            title: "Tarjeta repetida",
            short_problem: "Pertenece al grid.",
            why_it_matters: "Se repite en catálogo.",
            recommended_action: "Optimiza la galería.",
            confidence: "high",
            likely_lcp_impact: "low",
            related_action_id: "act-1",
            evidence: [],
            recommended_fix: {
              summary: "Optimiza el grid.",
              optimized_code: "<Image />",
              changes: ["Usa variantes responsivas"],
              expected_impact: "Menos bytes.",
            },
          },
          bounding_box: null,
        },
      ],
    };

    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: unanchoredReport,
    });

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("https://example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.report?.url).toBe("https://example.com");
    expect(result.current.selectedElementID).toBe("avatar");
  });

  it("prefers the first anchored top action when earlier actions are informational", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 1,
    });

    const anchoredReport: ScanReport = {
      ...fakeReport,
      insights: {
        ...fakeReport.insights,
        top_actions: [
          {
            id: "act-1",
            related_finding_id: "repeated_gallery_overdelivery",
            title: "Acción editorial",
            reason: "No hay anchor visible veraz.",
            confidence: "high",
            evidence: ["Sin anchor visible."],
            estimated_savings_bytes: 250000,
            likely_lcp_impact: "low",
            related_resource_ids: [],
            visible_related_resource_ids: [],
          },
          {
            id: "act-2",
            related_finding_id: "font_stack_overweight",
            title: "Acción anclada",
            reason: "Esta acción sí tiene un recurso visible asociado.",
            confidence: "medium",
            evidence: ["Fuente visible en vampires."],
            estimated_savings_bytes: 50000,
            likely_lcp_impact: "low",
            related_resource_ids: ["font-1"],
            visible_related_resource_ids: ["font-1"],
          },
        ],
      },
      vampire_elements: [
        {
          id: "avatar",
          url: "https://example.com/avatar.webp",
          type: "image",
          mime_type: "image/webp",
          hostname: "example.com",
          party: "first_party",
          status_code: 200,
          bytes: 18000,
          failed: false,
          failure_reason: "",
          transfer_share: 1,
          estimated_savings_bytes: 12000,
          position_band: "above_fold",
          visual_role: "above_fold_media",
          dom_tag: "img",
          loading_attr: "",
          fetch_priority: "",
          responsive_image: false,
          is_third_party_tool: false,
          third_party_kind: "unknown",
          asset_insight: {
            source: "rule_based",
            scope: "asset",
            title: "Avatar",
            short_problem: "Activo visible.",
            why_it_matters: "Referencia base.",
            recommended_action: "Revisar este recurso.",
            confidence: "low",
            likely_lcp_impact: "low",
            evidence: [],
          },
          bounding_box: null,
        },
        {
          id: "font-1",
          url: "https://example.com/font.woff2",
          type: "font",
          mime_type: "font/woff2",
          hostname: "example.com",
          party: "first_party",
          status_code: 200,
          bytes: 45000,
          failed: false,
          failure_reason: "",
          transfer_share: 4,
          estimated_savings_bytes: 15000,
          position_band: "unknown",
          visual_role: "unknown",
          dom_tag: "",
          loading_attr: "",
          fetch_priority: "",
          responsive_image: false,
          is_third_party_tool: false,
          third_party_kind: "unknown",
          asset_insight: {
            source: "rule_based",
            scope: "group",
            title: "Fuente pesada",
            short_problem: "Fuente visible.",
            why_it_matters: "Impacta el render.",
            recommended_action: "Recortar tipografía.",
            confidence: "medium",
            likely_lcp_impact: "low",
            related_action_id: "act-2",
            evidence: [],
          },
          bounding_box: null,
        },
      ],
    };

    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: anchoredReport,
    });

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("https://example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.report?.url).toBe("https://example.com");
    expect(result.current.selectedElementID).toBe("font-1");
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

    act(() => {
      result.current.setInputURL("https://example.com");
    });

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

    act(() => {
      result.current.setInputURL("https://example.com");
    });

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

    act(() => {
      result.current.setInputURL("https://example.com");
    });

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

  it("polls async insights and overlays the enriched report", async () => {
    vi.useFakeTimers();
    mockSubmitScan.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "queued",
      position: 1,
    });
    mockFetchInsights
      .mockResolvedValueOnce({
        job_id: "wl_job",
        status: "processing",
      })
      .mockResolvedValueOnce({
        job_id: "wl_job",
        status: "ready",
        insights: {
          ...fakeReport.insights,
          provider: "gemini",
          executive_summary: "Resumen Gemini",
        },
        vampire_elements: [
          {
            id: "hero",
            url: "https://example.com/hero.webp",
            type: "image",
            mime_type: "image/webp",
            hostname: "example.com",
            party: "first_party",
            status_code: 200,
            bytes: 200000,
            failed: false,
            failure_reason: "",
            transfer_share: 10,
            estimated_savings_bytes: 50000,
            position_band: "above_fold",
            visual_role: "hero_media",
            dom_tag: "img",
            loading_attr: "",
            fetch_priority: "",
            responsive_image: true,
            is_third_party_tool: false,
            third_party_kind: "unknown",
            asset_insight: {
              source: "gemini",
              scope: "asset",
              title: "Hero optimizable",
              short_problem: "Resumen Gemini",
              why_it_matters: "Empuja el arranque.",
              recommended_action: "Comprime la hero.",
              confidence: "medium",
              likely_lcp_impact: "medium",
              evidence: [],
            },
            bounding_box: null,
          },
        ],
      });

    const baseReportWithVampire: ScanReport = {
      ...fakeReport,
      vampire_elements: [
        {
          id: "hero",
          url: "https://example.com/hero.webp",
          type: "image",
          mime_type: "image/webp",
          hostname: "example.com",
          party: "first_party",
          status_code: 200,
          bytes: 200000,
          failed: false,
          failure_reason: "",
          transfer_share: 10,
          estimated_savings_bytes: 50000,
          position_band: "above_fold",
          visual_role: "hero_media",
          dom_tag: "img",
          loading_attr: "",
          fetch_priority: "",
          responsive_image: true,
          is_third_party_tool: false,
          third_party_kind: "unknown",
          asset_insight: {
            source: "rule_based",
            scope: "asset",
            title: "Hero pesada",
            short_problem: "Base rule-based",
            why_it_matters: "Empuja el arranque.",
            recommended_action: "Comprime la hero.",
            confidence: "medium",
            likely_lcp_impact: "medium",
            evidence: [],
          },
          bounding_box: null,
        },
      ],
    };
    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_job",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: baseReportWithVampire,
    });

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("https://example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.insightsStatus).toBe("processing");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });

    expect(mockFetchInsights).toHaveBeenCalledWith("wl_job");
    expect(result.current.insightsStatus).toBe("ready");
    expect(result.current.report?.insights.provider).toBe("gemini");
    expect(result.current.report?.insights.executive_summary).toBe("Resumen Gemini");
  });

  it("rehydrates the last completed job from sessionStorage and resumes insights polling", async () => {
    vi.useFakeTimers();
    window.sessionStorage.setItem("wattless.last_completed_scan_job", "wl_completed");
    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_completed",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: fakeReport,
    });
    mockFetchInsights.mockResolvedValueOnce({
      job_id: "wl_completed",
      status: "processing",
    });

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await vi.runAllTicks();
    });

    expect(mockPollScanJob).toHaveBeenCalledWith("wl_completed");
    expect(result.current.reportJobId).toBe("wl_completed");
    expect(result.current.report?.url).toBe("https://example.com");
    expect(result.current.insightsStatus).toBe("processing");
  });

  it("stops polling insights and clears persisted recovery state when the completed job no longer exists", async () => {
    vi.useFakeTimers();
    window.sessionStorage.setItem("wattless.last_completed_scan_job", "wl_gone");
    mockPollScanJob.mockResolvedValueOnce({
      job_id: "wl_gone",
      url: "https://example.com",
      status: "completed",
      position: 0,
      report: fakeReport,
    });
    mockFetchInsights.mockRejectedValueOnce(
      new APIError("No encontramos ese turno.", {
        status: 404,
        code: "job_not_found",
      }),
    );

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await vi.runAllTicks();
    });

    expect(result.current.report?.url).toBe("https://example.com");
    expect(window.sessionStorage.getItem("wattless.last_completed_scan_job")).toBeNull();
    expect(result.current.insightsStatus).toBe("none");
  });
});
