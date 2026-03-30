import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { ScanReport } from "@/lib/types";

import { AuditEvidenceStrip } from "./audit-evidence-strip";

function makeReport(): ScanReport {
  return {
    url: "https://example.com",
    score: "B",
    total_bytes_transferred: 1234,
    co2_grams_per_visit: 0.12,
    hosting_is_green: true,
    hosting_verdict: "green",
    hosted_by: "Example Host",
    site_profile: {
      framework_hint: "generic",
      evidence: [],
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
      script_resource_duration_ms: 20,
      lcp_ms: 1680,
      fcp_ms: 1100,
      render_metrics_complete: true,
      long_tasks_total_ms: 0,
      long_tasks_count: 0,
      lcp_resource_url: "https://cdn.example.com/hero.webp",
      lcp_resource_tag: "img",
    },
    analysis: {
      summary: {
        above_fold_visual_bytes: 0,
        below_fold_bytes: 0,
        lcp_resource_id: "lcp-1",
        lcp_resource_url: "https://cdn.example.com/hero.webp",
        lcp_resource_bytes: 16_817,
        analytics_bytes: 0,
        analytics_requests: 0,
        font_bytes: 0,
        font_requests: 0,
        repeated_gallery_bytes: 0,
        repeated_gallery_count: 0,
        render_critical_bytes: 16_817,
      },
      findings: [],
      resource_groups: [],
    },
    screenshot: {
      mime_type: "image/png",
      strategy: "single",
      viewport_width: 1200,
      viewport_height: 900,
      document_width: 1200,
      document_height: 900,
      captured_height: 900,
      tiles: [],
    },
    meta: {
      generated_at: "2026-03-30T00:00:00Z",
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
  };
}

describe("AuditEvidenceStrip", () => {
  it("shows mapped lcp bytes even when the resource is outside vampire elements", () => {
    render(<AuditEvidenceStrip report={makeReport()} />);

    expect(screen.getByText("16.4 KB")).toBeTruthy();
    expect(screen.getByText("Recurso LCP mapeado fuera del overlay")).toBeTruthy();
  });
});
