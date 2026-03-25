export interface BoundingBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface TopAction {
  id: string;
  title: string;
  reason: string;
  estimated_savings_bytes: number;
  likely_lcp_impact: "high" | "medium" | "low";
  related_resource_id: string;
}

export interface ScanInsights {
  provider: string;
  executive_summary: string;
  pitch_line: string;
  top_actions: TopAction[];
}

export interface VampireElement {
  id: string;
  url: string;
  type: string;
  mime_type: string;
  hostname: string;
  party: "first_party" | "third_party";
  status_code: number;
  bytes: number;
  failed: boolean;
  failure_reason: string;
  transfer_share: number;
  estimated_savings_bytes: number;
  recommendation: string;
  bounding_box: BoundingBox | null;
}

export interface ResourceBreakdown {
  label: string;
  bytes: number;
  requests: number;
  percentage: number;
}

export interface Summary {
  total_requests: number;
  successful_requests: number;
  failed_requests: number;
  first_party_bytes: number;
  third_party_bytes: number;
  potential_savings_bytes: number;
  visual_mapped_vampires: number;
}

export interface PerformanceMetrics {
  load_ms: number;
  dom_content_loaded_ms: number;
  script_duration_ms: number;
  lcp_ms: number;
  fcp_ms: number;
}

export interface ScreenshotPayload {
  mime_type: string;
  width: number;
  height: number;
  data_base64: string;
}

export interface ScanReport {
  url: string;
  score: string;
  total_bytes_transferred: number;
  co2_grams_per_visit: number;
  hosting_is_green: boolean;
  hosting_verdict: "green" | "not_green" | "unknown";
  hosted_by: string;
  summary: Summary;
  breakdown_by_type: ResourceBreakdown[];
  breakdown_by_party: ResourceBreakdown[];
  insights: ScanInsights;
  vampire_elements: VampireElement[];
  performance: PerformanceMetrics;
  screenshot: ScreenshotPayload;
  warnings: string[];
}

export interface RefactorReportContext {
  url: string;
  score?: string;
  co2_grams_per_visit?: number;
  total_bytes_transferred?: number;
  lcp_ms?: number;
  fcp_ms?: number;
}

export interface GreenFixRequest {
  framework: string;
  language: string;
  code: string;
  related_resource_id?: string;
  report_context: RefactorReportContext;
}

export interface GreenFixResponse {
  provider: string;
  summary: string;
  optimized_code: string;
  changes: string[];
  expected_impact: string;
}

export interface APIErrorPayload {
  error: string;
}
