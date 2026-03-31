import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { InsightsPanel } from "./insights-panel";
import type { ScanReport } from "@/lib/types";

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
      evidence: ["No se detectaron marcadores claros de framework; se usa perfil genérico."],
    },
    summary: {
      total_requests: 2,
      successful_requests: 2,
      failed_requests: 0,
      first_party_bytes: 1234,
      third_party_bytes: 0,
      potential_savings_bytes: 100,
      visual_mapped_vampires: 1,
    },
    breakdown_by_type: [],
    breakdown_by_party: [],
    insights: {
      provider: "rule_based",
      executive_summary: "Resumen editorial",
      pitch_line: "Pitch line",
      top_actions: [
        {
          id: "act-unanchored",
          related_finding_id: "repeated_gallery_overdelivery",
          title: "Acción sin anchor",
          reason: "No hay recurso visible veraz para enlazar.",
          confidence: "high",
          evidence: ["Sin anchor visible."],
          estimated_savings_bytes: 250000,
          likely_lcp_impact: "low",
          related_resource_ids: [],
          visible_related_resource_ids: [],
        },
        {
          id: "act-anchored",
          related_finding_id: "font_stack_overweight",
          title: "Acción anclada",
          reason: "Esta sí apunta a un recurso visible.",
          confidence: "medium",
          evidence: ["Fuente visible."],
          estimated_savings_bytes: 50000,
          likely_lcp_impact: "low",
          related_resource_ids: ["font-1"],
          visible_related_resource_ids: ["font-1"],
        },
      ],
    },
    vampire_elements: [
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
        transfer_share: 3,
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
          short_problem: "Fuente anclada.",
          why_it_matters: "Impacta el render.",
          recommended_action: "Recortar tipografía.",
          confidence: "medium",
          likely_lcp_impact: "low",
          related_action_id: "act-anchored",
          evidence: [],
        },
        bounding_box: null,
      },
    ],
    performance: {
      load_ms: 1000,
      dom_content_loaded_ms: 500,
      script_resource_duration_ms: 10,
      lcp_ms: 900,
      fcp_ms: 400,
      render_metrics_complete: true,
      long_tasks_total_ms: 0,
      long_tasks_count: 0,
    },
    analysis: {
      summary: {
        above_fold_visual_bytes: 0,
        below_fold_bytes: 0,
        analytics_bytes: 0,
        analytics_requests: 0,
        font_bytes: 45000,
        font_requests: 1,
        repeated_gallery_bytes: 0,
        repeated_gallery_count: 0,
        render_critical_bytes: 45000,
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

describe("InsightsPanel", () => {
  it("does not select anything when clicking an unanchored action", () => {
    const onSelectElement = vi.fn();

    render(
      <InsightsPanel
        report={makeReport()}
        insightsStatus="none"
        selectedElementID={null}
        onSelectElement={onSelectElement}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Acción sin anchor/i }));

    expect(onSelectElement).not.toHaveBeenCalled();
  });

  it("marks the first anchored action as active when there is no selection", () => {
    render(
      <InsightsPanel
        report={makeReport()}
        insightsStatus="none"
        selectedElementID={null}
        onSelectElement={() => {}}
      />,
    );

    const unanchored = screen.getByRole("button", { name: /Acción sin anchor/i });
    const anchored = screen.getByRole("button", { name: /Acción anclada/i });

    expect(anchored.getAttribute("class")).toContain("bg-primary/10");
    expect(anchored.getAttribute("class")).toContain("border-primary/20");
    expect(unanchored.getAttribute("class")).not.toContain("bg-primary/10");
  });

  it("shows AI progress badge while async insights are processing", () => {
    render(
      <InsightsPanel
        report={makeReport()}
        insightsStatus="processing"
        selectedElementID={null}
        onSelectElement={() => {}}
      />,
    );

    expect(screen.getByText(/Generando insights con IA/i)).toBeTruthy();
  });
});
