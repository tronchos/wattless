import { describe, expect, it, vi } from "vitest";
import { render } from "@testing-library/react";

import { FindingsPanel } from "./findings-panel";
import type { AnalysisFinding } from "@/lib/types";

describe("FindingsPanel", () => {
  it("does not emit duplicate key warnings when evidence repeats", () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    const findings: AnalysisFinding[] = [
      {
        id: "finding-1",
        category: "media",
        severity: "high",
        confidence: "high",
        title: "Hero media demasiado pesada",
        summary: "La evidencia repetida no debe romper el render.",
        evidence: ["Misma evidencia", "Misma evidencia"],
        estimated_savings_bytes: 120000,
        related_resource_ids: ["hero"],
      },
    ];

    render(<FindingsPanel findings={findings} />);

    expect(hasDuplicateKeyWarning(consoleErrorSpy)).toBe(false);

    consoleErrorSpy.mockRestore();
  });
});

function hasDuplicateKeyWarning(spy: ReturnType<typeof vi.spyOn>): boolean {
  return spy.mock.calls.some((args) =>
    args.some(
      (arg) =>
        typeof arg === "string" &&
        arg.includes("Each child in a list should have a unique"),
    ),
  );
}
