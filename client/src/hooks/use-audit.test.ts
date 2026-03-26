import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, act } from "@testing-library/react";
import { useAudit } from "./use-audit";
import type { ScanReport } from "@/lib/types";

vi.mock("@/lib/api", () => ({
  scanURL: vi.fn(),
}));

import { scanURL } from "@/lib/api";
const mockScanURL = vi.mocked(scanURL);

beforeEach(() => {
  vi.clearAllMocks();
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
      "Escribe una URL para empezar el análisis."
    );
    expect(mockScanURL).not.toHaveBeenCalled();
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
      "La URL no es válida. Verifica el formato e intenta de nuevo."
    );
    expect(mockScanURL).not.toHaveBeenCalled();
  });

  it("auto-prepends https:// when missing scheme", async () => {
    const fakeReport = {
      url: "https://example.com",
      vampire_elements: [],
      insights: { top_actions: [] },
    };
    mockScanURL.mockResolvedValueOnce(fakeReport as never);

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(mockScanURL).toHaveBeenCalledWith("https://example.com");
    expect(result.current.scanError).toBeNull();
  });

  it("increments selectionSignal when selecting the same asset again", async () => {
    const fakeReport = {
      url: "https://example.com",
      vampire_elements: [
        {
          id: "asset-1",
          asset_insight: {
            source: "rule_based",
            scope: "asset",
            title: "Asset",
            short_problem: "Problem",
            why_it_matters: "Why",
            recommended_action: "Do this",
            confidence: "medium",
            likely_lcp_impact: "low",
            evidence: [],
          },
        },
      ],
      insights: { top_actions: [] },
    } as unknown as ScanReport;
    mockScanURL.mockResolvedValueOnce(fakeReport as never);

    const { result } = renderHook(() => useAudit());

    await act(async () => {
      await result.current.handleSubmit();
    });

    const initialSignal = result.current.selectionSignal;
    act(() => {
      result.current.setSelectedElementID("asset-1");
    });

    expect(result.current.selectionSignal).toBe(initialSignal + 1);
  });

  it("displays network error message on scan failure", async () => {
    mockScanURL.mockRejectedValueOnce(new Error("Network error"));

    const { result } = renderHook(() => useAudit());

    act(() => {
      result.current.setInputURL("https://example.com");
    });

    await act(async () => {
      await result.current.handleSubmit();
    });

    expect(result.current.scanError).toBe("Network error");
    expect(result.current.isScanning).toBe(false);
  });
});
