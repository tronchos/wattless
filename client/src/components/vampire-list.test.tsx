import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";

import { VampireList } from "./vampire-list";
import type { VampireElement } from "@/lib/types";

vi.mock("@/components/dominant-asset-detail", () => ({
  DominantAssetDetail: () => <div>DominantAssetDetail</div>,
}));

const elements: VampireElement[] = [
  {
    id: "font-1",
    url: "https://example.com/font.woff2",
    type: "font",
    mime_type: "font/woff2",
    hostname: "example.com",
    party: "first_party",
    status_code: 200,
    bytes: 45_000,
    failed: false,
    failure_reason: "",
    transfer_share: 3,
    estimated_savings_bytes: 15_000,
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
      scope: "asset",
      title: "Fuente pesada",
      short_problem: "Fuente visible.",
      why_it_matters: "Impacta el render.",
      recommended_action: "Recortar tipografía.",
      confidence: "medium",
      likely_lcp_impact: "low",
      evidence: [],
    },
    bounding_box: null,
  },
];

describe("VampireList", () => {
  it("renders Spanish labels and translated selection controls", () => {
    const onSelect = vi.fn();

    render(
      <VampireList
        capturedHeight={900}
        elements={elements}
        selectedElementID={null}
        onSelect={onSelect}
      />,
    );

    expect(screen.getByText("Activos Dominantes")).toBeDefined();
    expect(screen.getByText("Sin anclaje visual")).toBeDefined();
    expect(screen.getByText("Costo tipográfico")).toBeDefined();

    fireEvent.click(screen.getByRole("button", { name: /seleccionar activo font\.woff2/i }));

    expect(onSelect).toHaveBeenCalledWith("font-1");
  });
});
