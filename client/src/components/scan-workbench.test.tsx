import type { ComponentPropsWithoutRef, ReactNode } from "react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";

const mockUseAudit = vi.fn();

function passthrough(tag: "div" | "section") {
  return function PassthroughComponent({
    children,
    ...props
  }: ComponentPropsWithoutRef<typeof tag> & {
    children?: ReactNode;
    layout?: boolean;
    layoutId?: string;
    initial?: unknown;
    animate?: unknown;
    exit?: unknown;
    transition?: unknown;
  }) {
    const {
      layout: _layout,
      layoutId: _layoutID,
      initial: _initial,
      animate: _animate,
      exit: _exit,
      transition: _transition,
      ...rest
    } = props;
    void _layout;
    void _layoutID;
    void _initial;
    void _animate;
    void _exit;
    void _transition;
    const Tag = tag;
    return <Tag {...rest}>{children}</Tag>;
  };
}

vi.mock("framer-motion", () => ({
  AnimatePresence: ({ children }: { children?: ReactNode }) => <>{children}</>,
  LazyMotion: ({ children }: { children?: ReactNode }) => <>{children}</>,
  domAnimation: {},
  m: {
    section: passthrough("section"),
    div: passthrough("div"),
  },
}));

vi.mock("@/components/breakdown-bars", () => ({
  BreakdownBars: () => <div>BreakdownBars</div>,
}));
vi.mock("@/components/compare-banner", () => ({
  CompareBanner: () => <div>CompareBanner</div>,
}));
vi.mock("@/components/audit-evidence-strip", () => ({
  AuditEvidenceStrip: () => <div>AuditEvidenceStrip</div>,
}));
vi.mock("@/components/findings-panel", () => ({
  FindingsPanel: () => <div>FindingsPanel</div>,
}));
vi.mock("@/components/insights-panel", () => ({
  InsightsPanel: () => <div>InsightsPanel</div>,
}));
vi.mock("@/components/markdown-report-card", () => ({
  MarkdownReportCard: () => <div>MarkdownReportCard</div>,
}));
vi.mock("@/components/methodology-card", () => ({
  MethodologyCard: () => <div>MethodologyCard</div>,
}));
vi.mock("@/components/metric-card", () => ({
  MetricCard: () => <div>MetricCard</div>,
}));
vi.mock("@/components/screenshot-inspector", () => ({
  ScreenshotInspector: () => <div>ScreenshotInspector</div>,
}));
vi.mock("@/components/score-ring", () => ({
  ScoreRing: () => <div>ScoreRing</div>,
}));
vi.mock("@/components/vampire-list", () => ({
  VampireList: () => <div>VampireList</div>,
}));
vi.mock("@/hooks/use-audit", async () => {
  const actual = await vi.importActual<typeof import("@/hooks/use-audit")>(
    "@/hooks/use-audit",
  );
  return {
    ...actual,
    useAudit: () => mockUseAudit(),
  };
});

import { ScanWorkbench } from "./scan-workbench";

const fakeReport = {
  url: "https://example.com",
  score: "A",
  total_bytes_transferred: 1234,
  co2_grams_per_visit: 0.12,
  hosting_is_green: true,
  hosting_verdict: "green",
  hosted_by: "Example Host",
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
  warnings: ["Advertencia repetida", "Advertencia repetida"],
} as const;

describe("ScanWorkbench", () => {
  beforeEach(() => {
    mockUseAudit.mockReturnValue({
      inputURL: "https://example.com",
      setInputURL: vi.fn(),
      report: fakeReport,
      previousReport: null,
      selectedElementID: null,
      setSelectedElementID: vi.fn(),
      selectionSignal: 0,
      selectedElement: null,
      scanError: null,
      isScanning: false,
      scanProgressIndex: 0,
      handleSubmit: vi.fn(),
      jobStatus: null,
      queuePosition: null,
      estimatedWaitSeconds: null,
      submittedURL: "https://example.com",
      reportJobId: "wl_report",
      conflictingJob: null,
      insightsStatus: "none",
      resumeConflictingJob: vi.fn(),
    });
  });

  it("does not emit duplicate key warnings when report warnings repeat", () => {
    const consoleErrorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    render(<ScanWorkbench />);

    expect(hasDuplicateKeyWarning(consoleErrorSpy)).toBe(false);

    consoleErrorSpy.mockRestore();
  });
});

function hasDuplicateKeyWarning(spy: ReturnType<typeof vi.spyOn>): boolean {
  return spy.mock.calls.some((args: unknown[]) =>
    args.some(
      (arg: unknown) =>
        typeof arg === "string" &&
        arg.includes("Each child in a list should have a unique"),
    ),
  );
}
