import { describe, expect, it } from "vitest";
import { createMarkdownReport } from "./report-markdown";
import type { ScanReport } from "./types";

function makeReport(): ScanReport {
  return {
    url: "https://example.com",
    score: "C",
    total_bytes_transferred: 1_000_000,
    co2_grams_per_visit: 0.42,
    hosting_is_green: false,
    hosting_verdict: "not_green",
    hosted_by: "",
    site_profile: {
      framework_hint: "astro",
      evidence: [],
    },
    summary: {
      total_requests: 10,
      successful_requests: 10,
      failed_requests: 0,
      first_party_bytes: 700_000,
      third_party_bytes: 300_000,
      potential_savings_bytes: 200_000,
      visual_mapped_vampires: 1,
    },
    breakdown_by_type: [],
    breakdown_by_party: [],
    insights: {
      provider: "rule_based",
      executive_summary: "Resumen base.",
      pitch_line: "Pitch base.",
      top_actions: [],
    },
    vampire_elements: [],
    performance: {
      load_ms: 1500,
      dom_content_loaded_ms: 800,
      script_resource_duration_ms: 200,
      lcp_ms: 900,
      fcp_ms: 700,
      long_tasks_total_ms: 100,
      long_tasks_count: 1,
      lcp_resource_tag: "h1",
    },
    analysis: {
      summary: {
        above_fold_bytes: 0,
        below_fold_bytes: 400_000,
        analytics_bytes: 0,
        analytics_requests: 0,
        font_bytes: 180_000,
        font_requests: 2,
        repeated_gallery_bytes: 300_000,
        repeated_gallery_count: 5,
        render_critical_bytes: 220_000,
      },
      findings: [
        {
          id: "render_lcp_dom_node",
          category: "render",
          severity: "medium",
          confidence: "medium",
          title: "Revisa el nodo que domina el LCP",
          summary: "El LCP es textual.",
          evidence: [],
          estimated_savings_bytes: 40_000,
          related_resource_ids: [],
        },
      ],
      resource_groups: [],
    },
    screenshot: {
      mime_type: "image/webp",
      strategy: "single",
      viewport_width: 1440,
      viewport_height: 900,
      document_width: 1440,
      document_height: 900,
      captured_height: 900,
      tiles: [],
    },
    meta: {
      generated_at: "2026-03-30T00:00:00Z",
      scan_duration_ms: 1200,
      scanner_version: "2026.03",
    },
    methodology: {
      model: "test",
      formula: "bytes * factor",
      source: "test",
      assumptions: [],
    },
    warnings: [],
  };
}

describe("createMarkdownReport", () => {
  it("adds a textual-first-render note when above-fold visuals are zero but render-critical bytes exist", () => {
    const markdown = createMarkdownReport(makeReport());

    expect(markdown).toContain("El primer render depende sobre todo de texto, fuentes y CSS.");
  });

  it("omits the textual-first-render note when above-fold visual bytes are present", () => {
    const report = makeReport();
    report.analysis.summary.above_fold_bytes = 32_000;

    const markdown = createMarkdownReport(report);

    expect(markdown).not.toContain("El primer render depende sobre todo de texto, fuentes y CSS.");
  });
});
