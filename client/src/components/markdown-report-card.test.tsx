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
  Object.assign(document, {
    execCommand: vi.fn(),
  });
});

describe("MarkdownReportCard", () => {
  it("renders the report content", () => {
    render(<MarkdownReportCard report={fakeReport} />);
    expect(screen.getByText(/Mock Report/)).toBeDefined();
    expect(screen.getByText("Resumen del reporte")).toBeDefined();
  });

  it("shows copy feedback after clicking copy", async () => {
    render(<MarkdownReportCard report={fakeReport} />);

    const copyButton = screen.getByRole("button", { name: /copiar reporte/i });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText("¡Copiado!")).toBeDefined();
    });

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      "# Mock Report\nTest content"
    );
  });

  it("falls back to execCommand when the Clipboard API fails", async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error("denied"));
    const execCommandSpy = vi
      .spyOn(document, "execCommand")
      .mockReturnValue(true);

    render(<MarkdownReportCard report={fakeReport} />);

    const copyButton = screen.getByRole("button", { name: /copiar reporte/i });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText("¡Copiado!")).toBeDefined();
    });

    expect(execCommandSpy).toHaveBeenCalledWith("copy");
  });

  it("shows a visible error when copy fails completely", async () => {
    vi.mocked(navigator.clipboard.writeText).mockRejectedValueOnce(new Error("denied"));
    vi.spyOn(document, "execCommand").mockReturnValue(false);

    render(<MarkdownReportCard report={fakeReport} />);

    const copyButton = screen.getByRole("button", { name: /copiar reporte/i });
    fireEvent.click(copyButton);

    await waitFor(() => {
      expect(screen.getByText("Falló la copia")).toBeDefined();
    });
  });
});
