import { afterEach, describe, expect, it, vi } from "vitest";

import {
  APIError,
  buildAPIURL,
  buildScreenshotTileURL,
  fetchInsights,
  formatResourceLabel,
  pollScanJob,
  submitScan,
} from "./api";

describe("fetchInsights", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    window.localStorage.clear();
  });

  it("returns null when the insights endpoint responds 404", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "disabled", code: "insights_unavailable" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_missing")).resolves.toBeNull();
  });

  it("throws APIError when the insights endpoint responds 404 for a missing job", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "missing", code: "job_not_found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_missing")).rejects.toBeInstanceOf(APIError);
  });

  it("returns the parsed ready payload", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "wl_ready",
          status: "ready",
          insights: {
            provider: "gemini",
            executive_summary: "Resumen Gemini",
            pitch_line: "Pitch line",
            top_actions: [],
          },
          vampire_elements: [],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await expect(fetchInsights("wl_ready")).resolves.toMatchObject({
      job_id: "wl_ready",
      status: "ready",
    });
  });

  it("normalizes null nested arrays in the ready payload", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "wl_ready",
          status: "ready",
          insights: {
            provider: "gemini",
            executive_summary: "Resumen Gemini",
            pitch_line: "Pitch line",
            top_actions: [
              {
                id: "act-1",
                related_finding_id: "finding-1",
                title: "Acción",
                reason: "Razón",
                confidence: "high",
                evidence: null,
                estimated_savings_bytes: 1234,
                likely_lcp_impact: "medium",
                related_resource_ids: null,
                visible_related_resource_ids: null,
                recommended_fix: {
                  summary: "Fix",
                  optimized_code: "<Image />",
                  changes: null,
                  expected_impact: "medium",
                },
              },
            ],
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
              bytes: 100,
              failed: false,
              failure_reason: "",
              transfer_share: 1,
              estimated_savings_bytes: 50,
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
                title: "Hero",
                short_problem: "Peso",
                why_it_matters: "LCP",
                recommended_action: "Optimiza",
                confidence: "medium",
                likely_lcp_impact: "medium",
                evidence: null,
                recommended_fix: {
                  summary: "Fix",
                  optimized_code: "<Image />",
                  changes: null,
                  expected_impact: "medium",
                },
              },
              bounding_box: null,
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const result = await fetchInsights("wl_ready");

    expect(result?.insights?.top_actions[0]?.evidence).toEqual([]);
    expect(result?.insights?.top_actions[0]?.related_resource_ids).toEqual([]);
    expect(result?.insights?.top_actions[0]?.visible_related_resource_ids).toEqual([]);
    expect(result?.insights?.top_actions[0]?.recommended_fix?.changes).toEqual([]);
    expect(result?.vampire_elements?.[0]?.asset_insight.evidence).toEqual([]);
    expect(result?.vampire_elements?.[0]?.asset_insight.recommended_fix?.changes).toEqual([]);
  });

  it("throws APIError for expired jobs", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(JSON.stringify({ error: "expired" }), {
        status: 410,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(fetchInsights("wl_expired")).rejects.toBeInstanceOf(APIError);
  });
});

describe("pollScanJob", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    window.localStorage.clear();
  });

  it("normalizes null nested arrays in completed scan reports", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "wl_done",
          url: "https://example.com",
          status: "completed",
          position: 0,
          report: {
            url: "https://example.com",
            score: "A",
            total_bytes_transferred: 1234,
            co2_grams_per_visit: 0.12,
            hosting_is_green: true,
            hosting_verdict: "green",
            hosted_by: "Example Host",
            site_profile: {
              framework_hint: "generic",
              evidence: null,
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
            breakdown_by_type: null,
            breakdown_by_party: null,
            insights: {
              provider: "rule_based",
              executive_summary: "Resumen",
              pitch_line: "Pitch",
              top_actions: [
                {
                  id: "act-1",
                  related_finding_id: "finding-1",
                  title: "Acción",
                  reason: "Razón",
                  confidence: "high",
                  evidence: null,
                  estimated_savings_bytes: 10,
                  likely_lcp_impact: "low",
                  related_resource_ids: null,
                  visible_related_resource_ids: null,
                  recommended_fix: {
                    summary: "Fix",
                    optimized_code: "<Image />",
                    changes: null,
                    expected_impact: "low",
                  },
                },
              ],
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
                bytes: 100,
                failed: false,
                failure_reason: "",
                transfer_share: 1,
                estimated_savings_bytes: 50,
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
                  title: "Hero",
                  short_problem: "Peso",
                  why_it_matters: "LCP",
                  recommended_action: "Optimiza",
                  confidence: "medium",
                  likely_lcp_impact: "medium",
                  evidence: null,
                  recommended_fix: {
                    summary: "Fix",
                    optimized_code: "<Image />",
                    changes: null,
                    expected_impact: "medium",
                  },
                },
                bounding_box: null,
              },
            ],
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
              findings: [
                {
                  id: "finding-1",
                  category: "media",
                  severity: "high",
                  confidence: "high",
                  title: "Finding",
                  summary: "Summary",
                  evidence: null,
                  estimated_savings_bytes: 10,
                  related_resource_ids: null,
                },
              ],
              resource_groups: [
                {
                  id: "group-1",
                  kind: "repeated_gallery",
                  label: "Gallery",
                  total_bytes: 100,
                  resource_count: 1,
                  position_band: "mixed",
                  related_resource_ids: null,
                },
              ],
            },
            screenshot: {
              mime_type: "image/png",
              strategy: "single",
              viewport_width: 1200,
              viewport_height: 900,
              document_width: 1200,
              document_height: 900,
              captured_height: 900,
              tiles: null,
            },
            meta: {
              generated_at: "2026-03-31T00:00:00Z",
              scan_duration_ms: 1000,
              scanner_version: "2026.03",
            },
            methodology: {
              model: "test",
              formula: "test",
              source: "test",
              assumptions: null,
            },
            warnings: null,
          },
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    const result = await pollScanJob("wl_done");

    expect(result.report?.site_profile.evidence).toEqual([]);
    expect(result.report?.breakdown_by_type).toEqual([]);
    expect(result.report?.breakdown_by_party).toEqual([]);
    expect(result.report?.insights.top_actions[0]?.evidence).toEqual([]);
    expect(result.report?.insights.top_actions[0]?.visible_related_resource_ids).toEqual([]);
    expect(result.report?.analysis.findings[0]?.evidence).toEqual([]);
    expect(result.report?.analysis.findings[0]?.related_resource_ids).toEqual([]);
    expect(result.report?.analysis.resource_groups[0]?.related_resource_ids).toEqual([]);
    expect(result.report?.screenshot.tiles).toEqual([]);
    expect(result.report?.methodology.assumptions).toEqual([]);
    expect(result.report?.warnings).toEqual([]);
  });
});

describe("API base URL", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllEnvs();
    window.localStorage.clear();
  });

  it("keeps relative URLs when no production API base is configured", () => {
    expect(buildAPIURL("/api/v1/scans")).toBe("/api/v1/scans");
    expect(buildScreenshotTileURL("wl_job", 2)).toBe("/api/v1/scans/wl_job/screenshot?tile=2");
  });

  it("uses the configured production API origin for scan and screenshot URLs", async () => {
    vi.stubEnv("VITE_API_BASE_URL", "https://api.wattless.example");

    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          job_id: "wl_123",
          url: "https://example.com",
          status: "queued",
          position: 1,
        }),
        {
          status: 202,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await submitScan("https://example.com");

    expect(fetchSpy).toHaveBeenCalledWith(
      "https://api.wattless.example/api/v1/scans",
      expect.objectContaining({ method: "POST" }),
    );
    expect(buildScreenshotTileURL("wl_job", 1)).toBe(
      "https://api.wattless.example/api/v1/scans/wl_job/screenshot?tile=1",
    );
  });

  it("tolerates an API base configured with /api suffix", () => {
    vi.stubEnv("VITE_API_BASE_URL", "https://api.wattless.example/api");

    expect(buildAPIURL("/api/v1/scans/wl_job")).toBe(
      "https://api.wattless.example/api/v1/scans/wl_job",
    );
  });
});

describe("formatResourceLabel", () => {
  it("keeps xhr and fetch distinct in technical breakdowns", () => {
    expect(formatResourceLabel("xhr")).toBe("XHR");
    expect(formatResourceLabel("fetch")).toBe("Fetch");
  });
});
