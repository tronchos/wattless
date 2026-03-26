import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MarkdownReportCard } from "./markdown-report-card";
import type { ScanReport } from "@/lib/types";

vi.mock("@/lib/report-markdown", () => ({
  createMarkdownReport: vi.fn(() => "# Mock Report\nTest content"),
}));

const fakeReport = {
  url: "https://example.com",
  score: "B",
  vampire_elements: [],
  insights: { top_actions: [] },
} as unknown as ScanReport;

beforeEach(() => {
  vi.clearAllMocks();
  Object.assign(navigator, {
    clipboard: {
      writeText: vi.fn().mockResolvedValue(undefined),
    },
  });
});

describe("MarkdownReportCard", () => {
  it("renders the report content", () => {
    render(<MarkdownReportCard report={fakeReport} />);
    expect(screen.getByText(/Mock Report/)).toBeDefined();
    expect(screen.getByText("Audit Summary")).toBeDefined();
  });

  it("shows 'Copied!' feedback after clicking copy", async () => {
    render(<MarkdownReportCard report={fakeReport} />);

    const copyButton = screen.getByRole("button", { name: /copy report/i });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText("Copied!")).toBeDefined();
    });

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      "# Mock Report\nTest content"
    );
  });
});
