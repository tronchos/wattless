export interface BoundingBox {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface SiteProfile {
  framework_hint: "astro" | "nextjs" | "generic" | "unknown";
  evidence: string[];
}

export interface FixSuggestion {
  summary: string;
  optimized_code: string;
  changes: string[];
  expected_impact: string;
}

export interface TopAction {
  id: string;
  related_finding_id: string;
  title: string;
  reason: string;
  confidence: "high" | "medium" | "low";
  evidence: string[];
  estimated_savings_bytes: number;
  likely_lcp_impact: "high" | "medium" | "low";
  related_resource_ids: string[];
  visible_related_resource_ids: string[];
  recommended_fix?: FixSuggestion;
}

export interface AssetInsight {
  source: "gemini" | "rule_based" | "hybrid";
  scope: "asset" | "group" | "global";
  title: string;
  short_problem: string;
  why_it_matters: string;
  recommended_action: string;
  confidence: "high" | "medium" | "low";
  likely_lcp_impact: "high" | "medium" | "low";
  related_finding_id?: string;
  related_action_id?: string;
  evidence: string[];
  recommended_fix?: FixSuggestion;
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
  position_band: "above_fold" | "near_fold" | "below_fold" | "unknown" | "mixed";
  visual_role:
    | "lcp_candidate"
    | "hero_media"
    | "repeated_card_media"
    | "above_fold_media"
    | "below_fold_media"
    | "decorative"
    | "unknown";
  dom_tag: string;
  loading_attr: string;
  fetch_priority: string;
  responsive_image: boolean;
  natural_width?: number;
  natural_height?: number;
  visible_ratio?: number;
  is_third_party_tool: boolean;
  third_party_kind:
    | "analytics"
    | "ads"
    | "support"
    | "social"
    | "video_embed"
    | "payment"
    | "unknown";
  asset_insight: AssetInsight;
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
  script_resource_duration_ms: number;
  lcp_ms: number;
  fcp_ms: number;
  render_metrics_complete: boolean;
  long_tasks_total_ms: number;
  long_tasks_count: number;
  lcp_resource_url?: string;
  lcp_resource_tag?: string;
  lcp_selector_hint?: string;
  lcp_size?: number;
}

export interface AnalysisSummary {
  above_fold_visual_bytes: number;
  below_fold_bytes: number;
  lcp_resource_id?: string;
  lcp_resource_url?: string;
  lcp_resource_bytes?: number;
  analytics_bytes: number;
  analytics_requests: number;
  font_bytes: number;
  font_requests: number;
  repeated_gallery_bytes: number;
  repeated_gallery_count: number;
  render_critical_bytes: number;
}

export interface AnalysisFinding {
  id: string;
  category: "render" | "media" | "third_party" | "fonts" | "cpu";
  severity: "high" | "medium" | "low";
  confidence: "high" | "medium" | "low";
  title: string;
  summary: string;
  evidence: string[];
  estimated_savings_bytes: number;
  related_resource_ids: string[];
}

export interface ResourceGroup {
  id: string;
  kind: "repeated_gallery" | "third_party_cluster" | "font_cluster";
  label: string;
  total_bytes: number;
  resource_count: number;
  position_band: "above_fold" | "near_fold" | "below_fold" | "unknown" | "mixed";
  related_resource_ids: string[];
}

export interface Analysis {
  summary: AnalysisSummary;
  findings: AnalysisFinding[];
  resource_groups: ResourceGroup[];
}

export interface ScreenshotPayload {
  mime_type: string;
  strategy: "single" | "tiled";
  viewport_width: number;
  viewport_height: number;
  document_width: number;
  document_height: number;
  captured_height: number;
  tiles: ScreenshotTile[];
}

export interface ScreenshotTile {
  id: string;
  y: number;
  width: number;
  height: number;
  data_base64: string;
}

export interface ScanMeta {
  generated_at: string;
  scan_duration_ms: number;
  scanner_version: string;
}

export interface ScanMethodology {
  model: string;
  formula: string;
  source: string;
  assumptions: string[];
}

export interface ScanReport {
  url: string;
  score: string;
  total_bytes_transferred: number;
  co2_grams_per_visit: number;
  hosting_is_green: boolean;
  hosting_verdict: "green" | "not_green" | "unknown";
  hosted_by: string;
  site_profile: SiteProfile;
  summary: Summary;
  breakdown_by_type: ResourceBreakdown[];
  breakdown_by_party: ResourceBreakdown[];
  insights: ScanInsights;
  vampire_elements: VampireElement[];
  performance: PerformanceMetrics;
  analysis: Analysis;
  screenshot: ScreenshotPayload;
  meta: ScanMeta;
  methodology: ScanMethodology;
  warnings: string[];
}

export type ScanJobStatus =
  | "queued"
  | "scanning"
  | "completed"
  | "failed"
  | "expired";

export interface ScanJobResponse {
  job_id: string;
  url: string;
  status: ScanJobStatus;
  position: number;
  estimated_wait_seconds?: number;
  deduplicated?: boolean;
  report?: ScanReport;
  error?: string;
}

export interface APIErrorPayload {
  error: string;
}
